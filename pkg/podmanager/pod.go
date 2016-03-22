// Copyright 2015-2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer"
)

// Pod represents the operation and management of an individual container
// on the current system.
type Pod struct {
	manager *Manager
	log     *logray.Logger

	netNsPath string

	stagerPath      string
	stagerImage     *schema.ImageManifest
	stagerContainer libcontainer.Container
	stagerProcess   *libcontainer.Process
	stagerReady     *os.File
	stagerWaitCh    <-chan struct{}

	manifest   *backend.StagerManifest
	layerPaths map[string]string
	uuid       string
	name       string

	options *backend.PodOptions

	// skipNetworking is used when a container is not creating its own network
	// namespace. This happens when it is sharing the host's namespace or the
	// namespace of another container.
	skipNetworking bool

	directory string

	shuttingDown   bool
	shuttingDownCh chan struct{}
	state          backend.PodState
	mutex          sync.Mutex
	waitch         chan bool
}

// PodManifest returns the current pod manifest for the App Pod
// Specification.
func (pod *Pod) PodManifest() *schema.PodManifest {
	return pod.manifest.Pod
}

// State returns the current operating state of the pod.
func (pod *Pod) State() backend.PodState {
	pod.mutex.Lock()
	defer pod.mutex.Unlock()
	return pod.state
}

// isShuttingDown returns whether the pod is currently in the state of
// being shut down. This is an internal flag, separate from the State.
func (pod *Pod) isShuttingDown() bool {
	pod.mutex.Lock()
	defer pod.mutex.Unlock()
	return pod.shuttingDown
}

// start is an internal function which launches and starts the processes within
// the pod.
func (pod *Pod) start() {
	pod.mutex.Lock()
	pod.state = backend.STARTING
	pod.mutex.Unlock()

	// loop over the pod startup functions
	for _, f := range podStartup {
		if err := f(pod); err != nil {
			// FIXME more error handling
			pod.log.Errorf("startup error: %v", err)

			pod.mutex.Lock()
			pod.state = backend.ERRORED
			pod.mutex.Unlock()
			return
		}
	}

	pod.mutex.Lock()
	pod.state = backend.RUNNING
	pod.mutex.Unlock()
}

// Stop triggers the shutdown of the Pod.
func (pod *Pod) Stop() error {
	pod.mutex.Lock()
	if pod.shuttingDown {
		return nil
	}
	pod.shuttingDown = true
	pod.state = backend.STOPPING
	pod.mutex.Unlock()
	close(pod.shuttingDownCh)

	// loop over the pod stopping functions
	for _, f := range podStopping {
		if err := f(pod); err != nil {
			// FIXME more error handling
			pod.log.Errorf("Pod stopping error: %v", err)
		}
	}

	pod.mutex.Lock()
	pod.state = backend.STOPPED
	pod.mutex.Unlock()
	return nil
}

// UUID returns the UUID associated with the current Pod.
func (pod *Pod) UUID() string {
	if pod == nil {
		return ""
	}
	return pod.uuid
}

// Name returns the Name given to the current Pod.
func (pod *Pod) Name() string {
	if pod == nil {
		return ""
	}
	return pod.name
}

// ShortName returns a shortened name that can be used to reference the
// Pod. It is made of up of the first 8 digits of the pod's UUID.
func (pod *Pod) ShortName() string {
	if pod == nil {
		return ""
	} else if len(pod.uuid) >= 8 {
		return pod.uuid[0:8]
	}
	return pod.uuid
}

// Enter is used to load a console session within the container. It re-enters
// the container through the stage2 rather than through the initd so that it can
// easily stream in and out.
func (pod *Pod) Enter(appName string, app *types.App, stdin, stdout, stderr *os.File, postStart func()) (*os.Process, error) {
	if pod.State() != backend.RUNNING {
		return nil, fmt.Errorf("pod must be in the running state to enter it")
	}

	// get the working directory
	if app.WorkingDirectory == "" {
		app.WorkingDirectory = "/"
	}

	// set command to /bin/sh if none was est
	if len(app.Exec) == 0 {
		app.Exec = []string{"/bin/sh"}
	}

	// Encode the app to bytes so we can get any errors before starting.
	appBytes, err := json.Marshal(app)
	if err != nil {
		return nil, fmt.Errorf("failed to encode App: %v", err)
	}

	// allocate a pipe to send the app JSON over
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate pipe for sending configuration: %v", err)
	}
	ch := make(chan struct{})
	go func() {
		w.Write(appBytes)
		w.Close()
		close(ch)
	}()

	// Construct and launch the stager run process
	process := &libcontainer.Process{
		Cwd:        "/",
		User:       "0",
		Args:       []string{"/opt/stager/run", appName},
		Stdin:      stdin,
		Stdout:     stdout,
		Stderr:     stderr,
		ExtraFiles: []*os.File{r},
	}
	if err := pod.stagerContainer.Start(process); err != nil {
		return nil, fmt.Errorf("failed to start process in stager: %v", err)
	}
	r.Close()
	if postStart != nil {
		postStart()
	}

	// Allow 10 seconds for the JSON configuration to be read in.
	select {
	case <-ch:
	case <-time.After(time.Second * 10):
		pod.log.Error("Stager run failed to read in configuration within 10 seconds, stopping run process")
		process.Signal(syscall.SIGKILL)
		return nil, fmt.Errorf("stager run process timed out reading configuration")
	}

	pid, err := process.Pid()
	if err != nil {
		return nil, fmt.Errorf("failed to get process pid")
	}
	return os.FindProcess(pid)
}

// WaitForState is used to poll until the state of the pod reaches a desired
// state.
func (pod *Pod) WaitForState(timeout time.Duration, states ...backend.PodState) error {
	for end := time.Now().Add(timeout); time.Now().Before(end); time.Sleep(time.Millisecond * 100) {
		pod.mutex.Lock()
		state := pod.state
		pod.mutex.Unlock()

		for _, s := range states {
			if s == state {
				return nil
			}
		}
	}
	return fmt.Errorf("timeout exceeded waiting for state change")
}

// Wait can be used to block until the processes within a container are finished
// executed. It is primarily intended for an internal API to code against system
// services.
func (pod *Pod) Wait() {
	<-pod.waitch
}
