// Copyright 2015-2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/appc/spec/schema"
	"github.com/opencontainers/runc/libcontainer"

	cnitypes "github.com/appc/cni/pkg/types"
)

var (
	// These are the functions that will be called in order to handle pod spin up.
	podStartup = []func(*Pod) error{
		(*Pod).startingGetStager,
		(*Pod).startingDependencySet,
		(*Pod).startingBaseDirectories,
		(*Pod).startingApplyIsolators,
		(*Pod).startingNetwork,
		(*Pod).startingResolvConf,
		(*Pod).startingInitializeContainer,
		(*Pod).startingWriteManifest,
		(*Pod).launchStager,
		(*Pod).waitForReady,
	}

	// These are the functions that will be called in order to handle pod
	// teardown.
	podStopping = []func(*Pod) error{
		(*Pod).stoppingReadyPipe,
		(*Pod).stoppingSignal,
		(*Pod).stoppingNetwork,
		(*Pod).stoppingStager,
		(*Pod).stoppingDirectories,
		(*Pod).stoppingrRemoveFromParent,
	}
)

// startingGetStager locates the image manifest for the stager and validates
// that it can be used.
func (pod *Pod) startingGetStager() error {
	image := pod.manager.imageManager.GetImage(pod.options.StagerHash)
	if image == nil {
		return fmt.Errorf("failed to locate specified stager image")
	}
	if image.App == nil {
		return fmt.Errorf("the specified stager does not define an \"app\"")
	}
	pod.stagerImage = image

	resolution, err := pod.manager.imageManager.ResolveTree(pod.options.StagerHash)
	if err != nil {
		return fmt.Errorf("failed to resolve stager tree: %v", err)
	}

	if len(resolution.Paths) != 1 {
		return fmt.Errorf("stager image must have no dependencies")
	}
	pod.stagerPath = resolution.Paths[pod.options.StagerHash]
	return nil
}

// startingDependencySet resolves the dependencies for the pod's applications
// and applies them to the StagerManifest.
func (pod *Pod) startingDependencySet() error {
	for _, app := range pod.manifest.Pod.Apps {
		resolution, err := pod.manager.imageManager.ResolveTree(app.Image.ID.String())
		if err != nil {
			return fmt.Errorf("failed to resolve dependencies for app %q: %v", app.Name, err)
		}

		pod.manifest.AppImageOrder[string(app.Name)] = resolution.Order
		for k, v := range resolution.Manifests {
			pod.manifest.Images[k] = v
		}
		for layer, layerPath := range resolution.Paths {
			pod.layerPaths[layer] = layerPath
		}
	}

	return nil
}

// startingBaseDirectories handles creating the directory to store the container
// filesystem and tracking files.
func (pod *Pod) startingBaseDirectories() error {
	pod.log.Debug("Setting up directories.")

	// This is the top level directory that we will create for this container.
	pod.directory = filepath.Join(pod.manager.Options.PodDirectory, pod.ShortName())

	// Make the directories.
	mode := os.FileMode(0755)
	dirs := []string{pod.directory, pod.stagerRootPath(), filepath.Join(pod.stagerRootPath(), "tmp")}
	if err := mkdirs(dirs, mode, false); err != nil {
		return fmt.Errorf("failed to create base directories: %v", err)
	}

	pod.log.Debug("Done setting up directories.")
	return nil
}

// startingApplyIsolators is used to apply isolators onto the pod and update the
// stager to have everything necessary for the isolators. These are only
// specialized isolators that require coordination ourside the stager.
func (pod *Pod) startingApplyIsolators() error {
	var appList []schema.RuntimeApp

	for _, ra := range pod.manifest.Pod.Apps {
		runtimeApp := ra
		pod.setupHostPrivilegeIsolator(&runtimeApp)

		if err := pod.setupHostApiAccessIsolator(&runtimeApp); err != nil {
			return err
		}

		appList = append(appList, runtimeApp)
	}

	// Push any modifications to the pod manifest back onto it.
	pod.manifest.Pod.Apps = appList

	if err := pod.setupLinuxNamespaceIsolator(); err != nil {
		return err
	}

	return nil
}

// startingWriteManifest writes the manifest for the stager to the filesystem
// for it to read on startup.
func (pod *Pod) startingWriteManifest() error {
	pod.log.Debug("Setting up stager manifest")
	root := pod.stagerRootPath()

	// copy in the stager's filesystem
	if err := copypath(pod.stagerPath, root); err != nil {
		return fmt.Errorf("failed to prepare stager manifest: %q %v", pod.stagerPath, err)
	}

	// write the stager manifest
	f, err := os.Create(filepath.Join(root, "manifest"))
	if err != nil {
		return fmt.Errorf("failed to create stager manifest: %v", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(pod.manifest); err != nil {
		return fmt.Errorf("failed to marshal stager manifest: %v", err)
	}

	pod.log.Debug("Done up stager manifest")
	return nil
}

// startingNetwork configures the networking for the container using the Network
// Manager.
func (pod *Pod) startingNetwork() error {
	if pod.skipNetworking {
		pod.log.Debug("Skipping stager network configuration")
		return nil
	}
	if pod.manager.networkManager == nil {
		pod.log.Debug("Skipping network provisioning, network manager is not set")
		return nil
	}

	pod.log.Debug("Creating networking for stager")

	netNsPath, networkResults, err := pod.manager.networkManager.Provision(pod, pod.options.Networks)
	if err != nil {
		return fmt.Errorf("failed to provision networking: %v", err)
	}

	pod.netNsPath = netNsPath
	pod.networkResults = networkResults

	pod.log.Debug("Finshed configuring networking")
	return nil
}

// startingResolvConf handles writing the resolv.conf in the stager's filesystem
// after networking has been set up.
func (pod *Pod) startingResolvConf() error {
	var dns *cnitypes.DNS

	// Check to see if a DNS configuration was provided
	for _, result := range pod.networkResults {
		if result.DNS != nil {
			dns = result.DNS
			break
		}
	}

	// If none was found, parse the system's resolv.conf
	if dns == nil {
		var err error
		dns, err = dnsReadConfig("/etc/resolv.conf")
		if err != nil {
			return fmt.Errorf("failed to parse system resolv.conf: %v", err)
		}
	}

	if err := mkdirs([]string{filepath.Join(pod.stagerRootPath(), "etc")}, os.FileMode(0755), true); err != nil {
		return fmt.Errorf("failed to create /etc in stager: %v", err)
	}

	resolvConf := filepath.Join(pod.stagerRootPath(), "etc", "resolv.conf")
	f, err := os.OpenFile(resolvConf, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("failed to create /etc/resolv.conf for stager: %v", err)
	}
	defer f.Close()

	if dns.Domain != "" {
		fmt.Fprintf(f, "domain %s\n", dns.Domain)
	}
	if len(dns.Search) > 0 {
		fmt.Fprintf(f, "search %s\n", strings.Join(dns.Search, " "))
	}
	for _, ns := range dns.Nameservers {
		fmt.Fprintf(f, "nameserver %s\n", ns)
	}
	if len(dns.Options) > 0 {
		fmt.Fprintf(f, "options %s\n", strings.Join(dns.Options, " "))
	}

	return nil
}

// startingInitializeContainer handles the initialization of the container the
// stager process will be launched in. This is primarily around the container's
// configuration, not actually creating the container.
func (pod *Pod) startingInitializeContainer() error {
	pod.log.Debug("Initializing the container for the stager")

	config, err := pod.generateContainerConfig()
	if err != nil {
		return fmt.Errorf("failed to generate stager configuration: %v", err)
	}

	// setup the container
	container, err := pod.manager.factory.Create(pod.ShortName(), config)
	if err != nil {
		return fmt.Errorf("failed to initialize the container: %v", err)
	}
	pod.stagerContainer = container
	return nil
}

// Start the initd. This doesn't actually configure it, just starts it so we
// have a process and namespace to work with in the networking side of the
// world.
func (pod *Pod) launchStager() error {
	pod.log.Debug("Starting pod stager.")

	// Open a log file that all output from the container will be written to
	flags := os.O_WRONLY | os.O_APPEND | os.O_CREATE | os.O_EXCL | os.O_TRUNC
	stagerlog, err := os.OpenFile(pod.stagerLogPath(), flags, os.FileMode(0666))
	if err != nil {
		return fmt.Errorf("failed to open stager log path: %v", err)
	}
	defer stagerlog.Close()

	cwd := pod.stagerImage.App.WorkingDirectory
	if cwd == "" {
		cwd = "/"
	}

	// create the read pipes to pass to the stager
	readyR, readyW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to allocate pipe: %v", err)
	}
	defer readyW.Close()
	pod.stagerReady = readyR

	pod.stagerProcess = &libcontainer.Process{
		Cwd:        cwd,
		User:       "0",
		Args:       pod.stagerImage.App.Exec,
		Stdout:     stagerlog,
		Stderr:     stagerlog,
		ExtraFiles: []*os.File{readyW},
	}
	for _, env := range pod.stagerImage.App.Environment {
		pod.stagerProcess.Env = append(pod.stagerProcess.Env, fmt.Sprintf("%s=%s", env.Name, env.Value))
	}

	// apply any provided inputs/ouputs for individual apps
	pod.applyIOs()

	if err := pod.stagerContainer.Start(pod.stagerProcess); err != nil {
		pod.stagerProcess = nil
		return fmt.Errorf("failed to launch stager process: %v", err)
	}

	pid, err := pod.stagerProcess.Pid()
	if err != nil {
		return fmt.Errorf("failed to retrieve the pid of the stager process: %v", err)
	}
	pod.log.Tracef("Launched stager process, pid: %d", pid)
	pod.waitRoutine()
	return nil
}

// waitForReady is used to wait until the stager closes its end of the pipe to
// signal the pod is ready.
func (pod *Pod) waitForReady() error {
	pod.mutex.Lock()
	f := pod.stagerReady
	pod.mutex.Unlock()
	if f == nil {
		return fmt.Errorf("no ready pipe was found")
	}

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		io.Copy(ioutil.Discard, f)
		f.Close()
	}()

	select {
	case <-ch:
		pod.log.Debug("Stager signaled pod is running.")
	case <-pod.shuttingDownCh:
		pod.log.Debug("Pod is shutting down, not ready signal")
	}
	return nil
}

// stoppingReadyPipe is used to close out the ready pipe, if it was still open.
func (pod *Pod) stoppingReadyPipe() error {
	if pod.stagerReady != nil {
		pod.stagerReady.Close()
		pod.stagerReady = nil
	}
	return nil
}

// stoppingSignal sends a SIGQUIT to the stager process to have it gracefully
// shutdown the application's in the pod.
func (pod *Pod) stoppingSignal() error {
	pod.mutex.Lock()
	process := pod.stagerProcess
	pod.mutex.Unlock()
	if process == nil {
		return nil
	}

	pod.log.Trace("Sending shutdown signal to the stager process")
	if err := process.Signal(os.Signal(syscall.SIGTERM)); err != nil {
		return fmt.Errorf("failed to send TERM signal to stager: %v", err)
	}

	select {
	case <-pod.stagerWaitCh:
		pod.log.Trace("Stager has exited")
	case <-time.After(time.Second * 60):
		pod.log.Error("Stager failed to shutdown within 60 seconds.")
		if err := process.Signal(os.Signal(syscall.SIGKILL)); err != nil {
			return fmt.Errorf("failed to send KILL signal to stager: %v", err)
		}
	}

	pod.mutex.Lock()
	pod.stagerProcess = nil
	pod.mutex.Unlock()

	return nil
}

// stoppingNetwork handles the teardown of the networks the container is
// attached to.
func (pod *Pod) stoppingNetwork() error {
	if pod.skipNetworking || pod.manager.networkManager == nil {
		return nil
	}
	return pod.manager.networkManager.Deprovision(pod)
}

// stoppingStager handles terminating all of the processes belonging to the
// current container's cgroup and then deleting the container itself.
func (pod *Pod) stoppingStager() error {
	pod.mutex.Lock()
	container := pod.stagerContainer
	pod.mutex.Unlock()
	if container == nil {
		return nil
	}

	pod.log.Trace("Tearing down stager container.")
	if err := container.Destroy(); err != nil {
		return fmt.Errorf("failed to teardown stager container: %v", err)
	}

	// Make sure future calls don't attempt destruction.
	pod.mutex.Lock()
	pod.stagerContainer = nil
	pod.mutex.Unlock()

	pod.log.Trace("Done tearing down stager container.")
	return nil
}

// stoppingDirectories removes the directories associated with this Pod.
func (pod *Pod) stoppingDirectories() error {
	// If a directory has not been assigned then bail out early.
	if pod.directory == "" {
		return nil
	}

	pod.log.Trace("Removing container directories.")
	if err := unmountDirectories(pod.directory); err != nil {
		pod.log.Warnf("failed to unmount container directories: %s", err)
		return err
	}

	// Remove the directory that was created for this container, unless it is
	// specified to keep it.
	if err := os.RemoveAll(pod.directory); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	pod.log.Trace("Done tearing down container directories.")
	return nil
}

// stoppingrRemoveFromParent removes the container object itself from the Pod
// Manager.
func (pod *Pod) stoppingrRemoveFromParent() error {
	pod.log.Trace("Removing from the Pod Manager.")
	pod.manager.remove(pod)
	return nil
}
