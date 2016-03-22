// Copyright 2016 Apcera Inc. All rights reserved.

package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/graphstorage"
	"github.com/apcera/kurma/pkg/graphstorage/overlay"
	"github.com/apcera/logray"
	"github.com/opencontainers/runc/libcontainer"
)

type containerSetup struct {
	log *logray.Logger

	// Passed in configuration related fields.
	manifest     backend.StagerManifest
	stagerConfig *stagerConfig

	// state objects
	state      *stagerState
	stateMutex sync.Mutex
	isStopping bool

	// libcontainer related objects
	factory       libcontainer.Factory
	initContainer libcontainer.Container
	initProcess   *libcontainer.Process
	initWaitch    chan struct{}

	// app specific state. Be sure to use the appMutex when accessing the app maps
	appMutex      sync.RWMutex
	appContainers map[string]libcontainer.Container
	appProcesses  map[string]*libcontainer.Process
	appWaitch     map[string]chan struct{}
}

var (
	defaultStagerConfig = &stagerConfig{
		RequiredNamespaces: []string{"ipc", "pid", "uts"},
		DefaultNamespaces:  []string{"ipc", "net", "pid", "uts"},
		GraphStorage:       "overlay",
	}

	// These are the functions that will be called in order to handle stager startup.
	stagerStartup = []func(*containerSetup) error{
		(*containerSetup).setupSignalHandling,
		(*containerSetup).readManifest,
		(*containerSetup).populateState,
		(*containerSetup).writeState,
		(*containerSetup).createFactory,
		(*containerSetup).containerFilesystem,
		(*containerSetup).launchInit,
		(*containerSetup).createContainers,
		(*containerSetup).markRunning,
		(*containerSetup).writeState,
		(*containerSetup).signalReadyPipe,
	}

	stagerTeardown = []func(*containerSetup) error{
		(*containerSetup).markShuttingDown,
		(*containerSetup).writeState,
		(*containerSetup).stopProcesses,
		(*containerSetup).stopContainers,
	}
)

func Run() error {
	cs := &containerSetup{
		log:           logray.New(),
		stagerConfig:  defaultStagerConfig,
		appContainers: make(map[string]libcontainer.Container),
		appProcesses:  make(map[string]*libcontainer.Process),
		appWaitch:     make(map[string]chan struct{}),
	}
	return cs.run()
}

// run executes the setup functions to start up the stager and the applications
// in its pod.
func (cs *containerSetup) run() error {
	for i, f := range stagerStartup {
		if err := f(cs); err != nil {
			cs.log.Errorf("Startup function %d errored: %v", i, err)
			cs.isStopping = true
			cs.stop()
			return err
		}

		if cs.isStopping == true {
			break
		}
	}
	return nil
}

// stop is used to teardown the stager and its applications. Note that the
// functions do not return after errors and they need to be cognizant of any
// state they're accessing, as the stager may be tearing down after only
// partially setting up.
func (cs *containerSetup) stop() {
	if cs.isStopping {
		return
	}

	for i, f := range stagerTeardown {
		if err := f(cs); err != nil {
			cs.log.Errorf("Teardown function %d errored: %v", i, err)
		}
	}

	cs.log.Flush()
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

// setupSignalHandling registers the necessary signal handlers to perform
// shutdown.
func (cs *containerSetup) setupSignalHandling() error {
	cs.log.Debug("Setting up signal handlers.")

	signalc := make(chan os.Signal, 1)
	signal.Notify(signalc, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Watch the channel and handle any signals that come in.
	go func() {
		for sig := range signalc {
			cs.log.Infof("Received %s. Shutting down.", sig.String())
			cs.stop()
			fmt.Fprintln(os.Stderr, "Stager teardown complete, exiting")
			os.Exit(0)
		}
	}()
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
	cs.stateMutex.Lock()
	defer cs.stateMutex.Unlock()

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
			if err := os.MkdirAll(appVolume, os.FileMode(0755)); err != nil {
				return fmt.Errorf("failed to create mount path for app %q volume %q path %q: %v", app.Name, mount.Volume, appVolume, err)
			}
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

	config, err := cs.getInitContainerConfig()
	if err != nil {
		return fmt.Errorf("failed to generate init container config: %v", err)
	}

	container, err := cs.factory.Create("init", config)
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
		cs.appMutex.Lock()
		cs.appContainers[name] = container
		cs.appMutex.Unlock()

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
			Cwd:  workingDirectory,
			User: app.User,
			Args: app.Exec,
		}
		for _, env := range app.Environment {
			process.Env = append(process.Env, fmt.Sprintf("%s=%s", env.Name, env.Value))
		}

		// apply inputs/outputs passed in, then apply defaults
		cs.applyIO(name, process)
		if process.Stdout == nil {
			process.Stdout = applog
		}
		if process.Stderr == nil {
			process.Stderr = applog
		}

		if err := container.Start(process); err != nil {
			return fmt.Errorf("failed to launch stager process: %v", err)
		}
		cs.appMutex.Lock()
		cs.appProcesses[name] = process
		cs.appMutex.Unlock()

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
	cs.log.Info("Marking stager as running.")
	cs.stateMutex.Lock()
	cs.state.State = stagerStateRunning
	cs.stateMutex.Unlock()
	return nil
}

// signalReadyPipe is used to close the ready pipe from the kurma daemon to let
// it know that the pod is up.
func (cs *containerSetup) signalReadyPipe() error {
	f := os.NewFile(3, "ready")
	f.Close()
	cs.log.Info("Signaled kurma the pod is running")
	return nil
}

func (cs *containerSetup) markShuttingDown() error {
	cs.isStopping = true
	cs.log.Info("Marking stager as shutting down.")
	cs.stateMutex.Lock()
	cs.state.State = stagerStateTeardown
	cs.stateMutex.Unlock()
	return nil
}

// stopProcesses signals the processes within the containers to stop.
func (cs *containerSetup) stopProcesses() error {
	cs.log.Debug("Stopping application processes")
	wg := sync.WaitGroup{}

	cs.appMutex.RLock()
	defer cs.appMutex.RUnlock()

	// Stop the user applications first. For them, send a TERM signal, allow 30
	// seconds for them to stop, then send a kill.
	for app, process := range cs.appProcesses {
		cs.log.Tracef("Sending app %q TERM signal", app)
		if err := process.Signal(os.Signal(syscall.SIGTERM)); err != nil {
			cs.log.Errorf("failed to TERM process %q: %v", app, err)
			continue
		}

		wg.Add(1)
		go func(app string, ch chan struct{}, process *libcontainer.Process) {
			defer wg.Done()

			// Wait up to 30 seconds for the process to exit gracefully
			if ch != nil {
				select {
				case <-ch:
					cs.log.Tracef("App %q has exited", app)
					return
				case <-time.After(time.Second * 30):
				}
			}

			// Send it a kill signal and move on.
			cs.log.Tracef("Sending app %q KILL signal", app)
			if err := process.Signal(os.Signal(syscall.SIGKILL)); err != nil {
				cs.log.Errorf("failed to SIGKILL process %q: %v", app, err)
			}
		}(app, cs.appWaitch[app], process)
	}

	// Wait for all the apps to finish.
	wg.Wait()

	// Stop the init container. It listens for TERM and just exits.
	if cs.initProcess != nil {
		cs.log.Trace("Sending init TERM signal")
		if err := cs.initProcess.Signal(os.Signal(syscall.SIGTERM)); err != nil {
			cs.log.Errorf("failed to TERM the init process: %v", err)
		}

		if cs.initWaitch != nil {
			select {
			case <-cs.initWaitch:
				cs.log.Trace("Init has exited")
			case <-time.After(time.Second * 30):
				cs.log.Trace("Sending init KILL signal")
				if err := cs.initProcess.Signal(os.Signal(syscall.SIGKILL)); err != nil {
					cs.log.Errorf("failed to KILL the init process: %v", err)
				}
			}
		}
	}

	cs.log.Debug("Done stopping application processes")
	return nil
}

// stopContainers tears down the containers the applications were using.
func (cs *containerSetup) stopContainers() error {
	cs.log.Debug("Stopping application containers")

	cs.appMutex.RLock()
	defer cs.appMutex.RUnlock()

	for app, container := range cs.appContainers {
		if err := container.Destroy(); err != nil {
			cs.log.Errorf("error destroying app %q container: %v", app, err)
		}
	}

	if cs.initContainer != nil {
		if err := cs.initContainer.Destroy(); err != nil {
			cs.log.Errorf("error destroying init container: %v", err)
		}
	}

	cs.log.Debug("Done stopping application containers")
	return nil
}

// initWait is used to call Wait on the init process. If the init process exits,
// this will trigger all of the applications to be killed. When this happens,
// the stager will teardown and exit.
func (cs *containerSetup) initWait() {
	cs.initWaitch = make(chan struct{})
	cs.initProcess.Wait()
	close(cs.initWaitch)

	if cs.isShuttingDown() {
		return
	}

	cs.log.Error("The init process has exited, but the pod is not tearing down. Stager exiting.")
	fmt.Fprintln(os.Stderr, "The stager init process has exited. Stager exiting.")
	cs.stop()
	os.Exit(1)
}

// appWait is used to call Wait on an app's process and update the container
// state if the processes exits.
func (cs *containerSetup) appWait(name string, process *libcontainer.Process) {
	ch := make(chan struct{})
	cs.appMutex.Lock()
	cs.appWaitch[name] = ch
	cs.appMutex.Unlock()

	ps, err := process.Wait()
	close(ch)

	cs.stateMutex.Lock()
	cs.state.Apps[name].Pid = 0
	cs.state.Apps[name].Exited = true
	if ps != nil {
		cs.state.Apps[name].ExitCode = ps.Sys().(syscall.WaitStatus).ExitStatus()
	}
	if err != nil {
		cs.state.Apps[name].ExitReason = err.Error()
	}
	cs.stateMutex.Unlock()

	cs.log.Warnf("Application %q has exited %d: %s", name, cs.state.Apps[name].ExitCode, cs.state.Apps[name].ExitReason)

	if cs.isShuttingDown() {
		return
	}

	if err := cs.writeState(); err != nil {
		cs.log.Errorf("Failed to write state file: %v", err)
	}
}
