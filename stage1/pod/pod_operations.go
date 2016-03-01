// Copyright 2015-2016 Apcera Inc. All rights reserved.

package pod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/apcera/kurma/network"
	"github.com/opencontainers/runc/libcontainer"
)

var (
	// These are the functions that will be called in order to handle pod spin up.
	podStartup = []func(*Pod) error{
		(*Pod).startingGetStager,
		(*Pod).startingDependencySet,
		(*Pod).startingBaseDirectories,
		(*Pod).startingWriteManifest,
		(*Pod).setupLinuxNamespaceIsolator,
		(*Pod).startingNetwork,
		(*Pod).startingInitializeContainer,
		(*Pod).launchStager,
	}

	// These are the functions that will be called in order to handle pod
	// teardown.
	podStopping = []func(*Pod) error{
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
	hash, image := pod.manager.imageManager.FindImage("stager", "1.0.0")
	if image == nil {
		return fmt.Errorf("failed to locate specified stager image")
	}
	pod.stagerImage = image

	resolution, err := pod.manager.imageManager.ResolveTree(hash)
	if err != nil {
		return fmt.Errorf("failed to resolve stager tree: %v", err)
	}

	if len(resolution.Paths) != 1 {
		return fmt.Errorf("stager image must have no dependencies")
	}
	pod.stagerPath = resolution.Paths[hash]
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
	pod.log.Debug("Creating networking for stager")

	// Create a new network namespace
	if err := network.CreateNetworkNamespace(pod.stagerNetPath()); err != nil {
		return fmt.Errorf("failed to create network namespace: %v", err)
	}

	if pod.manager.networkManager == nil {
		pod.log.Debug("Skipping network provisioning, network manager is not set")
		return nil
	}
	_, err := pod.manager.networkManager.Provision(pod)
	if err != nil {
		return fmt.Errorf("failed to provision networking: %v", err)
	}

	// FIXME c.pod.Networks = results
	pod.log.Debug("Finshed configuring networking")
	return err
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
	stage2Stdout, err := os.OpenFile(pod.stagerLogPath(), flags, os.FileMode(0666))
	if err != nil {
		return fmt.Errorf("failed to open stager log path: %v", err)
	}
	defer stage2Stdout.Close()

	cwd := pod.stagerImage.App.WorkingDirectory
	if cwd == "" {
		cwd = "/"
	}

	pod.stagerProcess = &libcontainer.Process{
		Cwd:    cwd,
		User:   "0",
		Args:   pod.stagerImage.App.Exec,
		Stdin:  bytes.NewBuffer(nil),
		Stdout: stage2Stdout,
		Stderr: stage2Stdout,
	}
	for _, env := range pod.stagerImage.App.Environment {
		pod.stagerProcess.Env = append(pod.stagerProcess.Env, fmt.Sprintf("%s=%s", env.Name, env.Value))
	}
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

// stoppingSignal sends a SIGQUIT to the stager process to have it gracefully
// shutdown the application's in the pod.
func (pod *Pod) stoppingSignal() error {
	if pod.stagerProcess == nil {
		return nil
	}

	pod.log.Trace("Sending shutdown signal to the stager process")
	pod.stagerProcess.Signal(os.Signal(syscall.SIGQUIT))

	select {
	case <-pod.stagerWaitCh:
	case <-time.After(time.Minute):
		pod.log.Error("Stager failed to shutdown within 60 seconds...")
	}

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
	pod.log.Trace("Tearing down stager container.")

	if pod.stagerContainer == nil {
		return nil
	}

	if err := pod.stagerContainer.Destroy(); err != nil {
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
	pod.log.Trace("Removing container directories.")

	// If a directory has not been assigned then bail out early.
	if pod.directory == "" {
		return nil
	}

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
