// Copyright 2016 Apcera Inc. All rights reserved.

package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/apcera/kurma/stage1"
	"github.com/apcera/kurma/stager/graphstorage"
	"github.com/apcera/kurma/stager/graphstorage/overlay"
	"github.com/apcera/logray"
	"github.com/opencontainers/runc/libcontainer"
)

type containerSetup struct {
	log *logray.Logger

	// Passed in configuration related fields.
	manifest     stage1.StagerManifest
	stagerConfig *stagerConfig

	// state objects
	state      *stagerState
	stateMutex sync.Mutex

	// libcontainer related objects
	factory       libcontainer.Factory
	initContainer libcontainer.Container
	initProcess   *libcontainer.Process
	appContainers map[string]libcontainer.Container
	appProcesses  map[string]*libcontainer.Process
}

var (
	defaultStagerConfig = &stagerConfig{
		RequiredNamespaces: []string{"ipc", "pid", "uts"},
		DefaultNamespaces:  []string{"ipc", "net", "pid", "uts"},
		GraphStorage:       "overlay",
	}

	// These are the functions that will be called in order to handle stager startup.
	stagerStartup = []func(*containerSetup) error{
		(*containerSetup).readManifest,
		(*containerSetup).populateState,
		(*containerSetup).writeState,
		(*containerSetup).createFactory,
		(*containerSetup).containerFilesystem,
		(*containerSetup).launchInit,
		// (*containerSetup).transferNetworking,
		(*containerSetup).createContainers,
		(*containerSetup).markRunning,
		(*containerSetup).writeState,
	}
)

func Run() error {
	cs := &containerSetup{
		log:           logray.New(),
		stagerConfig:  defaultStagerConfig,
		appContainers: make(map[string]libcontainer.Container),
		appProcesses:  make(map[string]*libcontainer.Process),
	}
	return cs.run()
}

func (cs *containerSetup) run() error {
	for i, f := range stagerStartup {
		if err := f(cs); err != nil {
			cs.log.Errorf("Startup function %d errored: %v", i, err)
			return err
		}
	}

	time.Sleep(time.Hour * 24 * 30)
	return nil
}

// writeState is used to persist the current stager state to the state.json
// file. This can be read by other processes calling in to the stager's exposed
// command API to quickly access the pod state.
func (cs *containerSetup) writeState() error {
	f, err := os.OpenFile("state.json", os.O_WRONLY|os.O_CREATE, os.FileMode(0600))
	if err != nil {
		return fmt.Errorf("failed to open the state JSON file")
	}
	defer f.Close()

	cs.stateMutex.Lock()
	defer cs.stateMutex.Unlock()

	if err := json.NewEncoder(f).Encode(cs.state); err != nil {
		return fmt.Errorf("failed to write the stager state: %v", err)
	}
	return nil
}

// readManifest reads in the manifest provided by Kurma for the pod's
// definition. It will also parse out the stager configuration that was provided
// to Kurma and passed along.
func (cs *containerSetup) readManifest() error {
	cs.log.Debug("Reading stager manifest")

	f, err := os.Open("/manifest")
	if err != nil {
		return fmt.Errorf("failed to open stager manifest: %v", err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cs.manifest); err != nil {
		return fmt.Errorf("failed to parse stager manifest: %v", err)
	}

	if err := json.Unmarshal(cs.manifest.StagerConfig, &cs.stagerConfig); err != nil {
		return fmt.Errorf("failed to parse stager configuration: %v", err)
	}

	return nil
}

// populateState is used to initialize the state tracking object and ensure it
// includes all of the apps within the pod, even before it has started running
// them.
func (cs *containerSetup) populateState() error {
	cs.state = &stagerState{
		Apps:  make(map[string]*stagerAppState),
		State: stagerStateSetup,
	}
	for _, app := range cs.manifest.Pod.Apps {
		cs.state.Apps[app.Name.String()] = &stagerAppState{}
	}
	return nil
}

// createFactory initializes the libcontainer factory to be ready to create
// containers with.
func (cs *containerSetup) createFactory() error {
	factory, err := libcontainer.New("/containers")
	if err != nil {
		return fmt.Errorf("failed to create the libcontainer factory: %v", err)
	}
	cs.factory = factory
	return nil
}

// containerFilesystem configures the filesystem for the pod's applications.
func (cs *containerSetup) containerFilesystem() error {
	cs.log.Debug("Setting up container filesystem")

	// Create the top level directories for execution
	os.Mkdir("/apps", os.FileMode(0755))
	os.Mkdir("/init", os.FileMode(0755))
	os.Mkdir("/logs", os.FileMode(0755))

	// Create the configured provisioner
	var err error
	var provisioner graphstorage.StorageProvisioner
	switch cs.stagerConfig.GraphStorage {
	case "overlay":
		provisioner, err = overlay.New()
	default:
		return fmt.Errorf("unrecognized graph storage provider %q specified", cs.stagerConfig.GraphStorage)
	}
	if err != nil {
		return fmt.Errorf("failed to configure app storage: %v", err)
	}

	// Setup the applications
	for _, app := range cs.manifest.Pod.Apps {
		name := app.Name.String()
		apppath := filepath.Join("/apps", name)

		if err := os.Mkdir(apppath, os.FileMode(0755)); err != nil {
			return fmt.Errorf("failed to create app directory for %q: %v", name, err)
		}

		imagedefinition := make([]string, len(cs.manifest.AppImageOrder[name]))
		for i, hash := range cs.manifest.AppImageOrder[name] {
			imagedefinition[i] = filepath.Join("/layers", hash)
		}

		if err := provisioner.Create(apppath, imagedefinition); err != nil {
			return fmt.Errorf("failed to configure app %q filesystem: %v", name, err)
		}

		// Apply volumes to the application as well
		for _, mount := range app.Mounts {
			appVolume := filepath.Join(apppath, mount.Path)
			volPath := filepath.Join("/volumes", mount.Volume.String())
			err := syscall.Mount(volPath, appVolume, "", syscall.MS_BIND|syscall.MS_REC, "")
			if err != nil {
				return fmt.Errorf("failed to mount volume %q for app %q: %v", mount.Volume, name, err)
			}
		}
	}

	cs.log.Debug("Done setting up filesystem")
	return nil
}

// launchInit is used to launch the init process for the pod, which will be used
// to initially create the main namespaces.
func (cs *containerSetup) launchInit() error {
	cs.log.Debug("Launching the init process")

	container, err := cs.factory.Create("init", initContainerConfig)
	if err != nil {
		return fmt.Errorf("failed to create init container: %v", err)
	}
	cs.initContainer = container

	// Open the log file
	flags := os.O_WRONLY | os.O_APPEND | os.O_CREATE | os.O_EXCL | os.O_TRUNC
	initlog, err := os.OpenFile("/logs/init.log", flags, os.FileMode(0666))
	if err != nil {
		return fmt.Errorf("failed to open init process log: %v", err)
	}
	defer initlog.Close()

	cs.initProcess = &libcontainer.Process{
		Cwd:    "/",
		User:   "0",
		Args:   []string{"/init"},
		Stdin:  bytes.NewBuffer(nil),
		Stdout: initlog,
		Stderr: initlog,
	}
	if err := cs.initContainer.Start(cs.initProcess); err != nil {
		return fmt.Errorf("failed to launch init process: %v", err)
	}

	pid, err := cs.initProcess.Pid()
	if err != nil {
		return fmt.Errorf("failed to retrieve the pid of the init process: %v", err)
	}
	cs.log.Tracef("Launched init process, pid: %d", pid)

	go cs.initWait()

	return nil
}

// createContainers creates the containers for the applications in the pod
// manifest.
func (cs *containerSetup) createContainers() error {
	cs.log.Debug("Creating application containers")

	// Create the mount namespace for each app
	for _, runtimeApp := range cs.manifest.Pod.Apps {
		name := runtimeApp.Name.String()
		app := cs.getPodApp(runtimeApp)

		// create the container config
		containerConfig, err := cs.getAppContainerConfig(runtimeApp)
		if err != nil {
			return fmt.Errorf("failed to generate config for app %q: %v", name, err)
		}

		container, err := cs.factory.Create(name, containerConfig)
		if err != nil {
			return fmt.Errorf("failed to initialize the container for %q: %v", name, err)
		}
		cs.appContainers[name] = container

		// validate the working directory
		workingDirectory := app.WorkingDirectory
		if workingDirectory == "" {
			workingDirectory = "/"
		}

		cs.log.Tracef("Launching application [%q:%q]: %#v", app.User, app.Group, app.Exec)

		// Open a log file that all output from the container will be written to
		flags := os.O_WRONLY | os.O_APPEND | os.O_CREATE | os.O_EXCL | os.O_TRUNC
		applog, err := os.OpenFile(filepath.Join("/logs", name), flags, os.FileMode(0666))
		if err != nil {
			return err
		}
		defer applog.Close()

		process := &libcontainer.Process{
			Cwd:    workingDirectory,
			User:   app.User,
			Args:   app.Exec,
			Stdin:  bytes.NewBuffer(nil),
			Stdout: applog,
			Stderr: applog,
		}
		for _, env := range app.Environment {
			process.Env = append(process.Env, fmt.Sprintf("%s=%s", env.Name, env.Value))
		}
		if err := container.Start(process); err != nil {
			return fmt.Errorf("failed to launch stager process: %v", err)
		}
		cs.appProcesses[name] = process

		pid, err := process.Pid()
		if err != nil {
			return fmt.Errorf("failed to retrieve the pid of application %q: %v", name, err)
		}
		cs.log.Tracef("Launched app %q process, pid: %d", name, pid)
		cs.stateMutex.Lock()
		cs.state.Apps[name].Pid = pid
		cs.stateMutex.Unlock()

		go cs.appWait(name, process)
	}

	return nil
}

// markRunning is used to update the state flag that indicates the pod has been
// fully setup.
func (cs *containerSetup) markRunning() error {
	cs.stateMutex.Lock()
	cs.state.State = stagerStateRunning
	cs.stateMutex.Unlock()
	return nil
}

// initWait is used to call Wait on the init process. If the init process exits,
// this will trigger all of the applications to be killed. When this happens,
// the stager will teardown and exit.
func (cs *containerSetup) initWait() {
	cs.initProcess.Wait()
	if cs.isShuttingDown() {
		return
	}

	cs.log.Error("The init process has exited, but the pod is not tearing down. Stager exiting.")
	fmt.Fprintln(os.Stderr, "The stager init process has exited. Stager exiting.")
	os.Exit(1)
}

// appWait is used to call Wait on an app's process and update the container
// state if the processes exits.
func (cs *containerSetup) appWait(name string, process *libcontainer.Process) {
	ps, err := process.Wait()

	cs.stateMutex.Lock()
	cs.state.Apps[name].Exited = true
	if err != nil {
		cs.log.Errorf("Process.Wait() for %q returned error: %v", name, err)
	}
	if ps != nil {
		cs.state.Apps[name].ExitCode = ps.Sys().(syscall.WaitStatus).ExitStatus()
	}
	cs.stateMutex.Unlock()

	if err := cs.writeState(); err != nil {
		cs.log.Errorf("Failed to write state file: %v", err)
	}
}
