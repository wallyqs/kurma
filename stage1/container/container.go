// Copyright 2015 Apcera Inc. All rights reserved.

package container

import (
	"fmt"
	"os"
	"sync"

	kschema "github.com/apcera/kurma/schema"
	"github.com/apcera/kurma/stage1"
	"github.com/apcera/kurma/stage1/graphstorage"
	client2 "github.com/apcera/kurma/stage2/client"
	client3 "github.com/apcera/kurma/stage3/client"
	"github.com/apcera/kurma/util/cgroups"
	"github.com/apcera/logray"
	"github.com/apcera/util/envmap"
	"github.com/appc/spec/schema"

	_ "github.com/apcera/kurma/schema"
)

// Container represents the operation and management of an individual container
// on the current system.
type Container struct {
	manager *Manager
	log     *logray.Logger

	image     *schema.ImageManifest
	imageHash string
	pod       *kschema.PodManifest
	uuid      string

	// Linux capabilities which will be applied to any process started on the
	// container. The capabiltiies are not applied to the initd, since it could
	// hinder its operation. Instead, it is passed in with any start call.
	capabilities string

	// initdClient is the client object to talk to the initd process running
	// within the container.
	initdClient client3.Client

	// initdPid is the PID of the initd process within the container
	initdPid int

	// skipNetworking is used when a container is not creating its own network
	// namespace. This happens when it is sharing the host's namespace or the
	// namespace of another container.
	skipNetworking bool

	storage     graphstorage.PodStorage
	cgroup      *cgroups.Cgroup
	directory   string
	environment *envmap.EnvMap

	shuttingDown bool
	state        stage1.ContainerState
	mutex        sync.Mutex
	waitch       chan bool
}

// PodManifest returns the current pod manifest for the App Container
// Specification.
func (container *Container) PodManifest() *kschema.PodManifest {
	return container.pod
}

// ImageManifest returns the current image manifest for the App Container
// Specification.
func (container *Container) ImageManifest() *schema.ImageManifest {
	return container.image
}

// Pid returns the pid of the top level process withint he container.
func (container *Container) Pid() (int, error) {
	// Get a process from the container and copy its namespaces
	tasks, err := container.cgroup.Tasks()
	if err != nil {
		return 0, err
	}
	if len(tasks) == 0 {
		return 0, fmt.Errorf("no processes are running inside the container")
	}
	return tasks[0], nil
}

// State returns the current operating state of the container.
func (container *Container) State() stage1.ContainerState {
	container.mutex.Lock()
	defer container.mutex.Unlock()
	return container.state
}

// isShuttingDown returns whether the container is currently in the state of
// being shut down. This is an internal flag, separate from the State.
func (container *Container) isShuttingDown() bool {
	container.mutex.Lock()
	defer container.mutex.Unlock()
	return container.shuttingDown
}

// start is an internal function which launches and starts the processes within
// the container.
func (container *Container) start() {
	container.mutex.Lock()
	container.state = stage1.STARTING
	container.mutex.Unlock()

	// loop over the container startup functions
	for _, f := range containerStartup {
		if err := f(container); err != nil {
			// FIXME more error handling
			container.log.Errorf("startup error: %v", err)
			return
		}
	}

	container.mutex.Lock()
	container.state = stage1.RUNNING
	container.mutex.Unlock()
}

// Stop triggers the shutdown of the Container.
func (container *Container) Stop() error {
	container.mutex.Lock()
	container.shuttingDown = true
	container.state = stage1.STOPPING
	container.mutex.Unlock()

	// loop over the container stopping functions
	for _, f := range containerStopping {
		if err := f(container); err != nil {
			// FIXME more error handling
			container.log.Errorf("stopping error: %v", err)
			return err
		}
	}

	container.mutex.Lock()
	container.state = stage1.STOPPED
	container.mutex.Unlock()
	return nil
}

// UUID returns the UUID associated with the current Container.
func (container *Container) UUID() string {
	if container == nil {
		return ""
	}
	return container.uuid
}

// ShortName returns a shortened name that can be used to reference the
// Container. It is made of up of the first 8 digits of the container's UUID.
func (container *Container) ShortName() string {
	if container == nil {
		return ""
	} else if len(container.uuid) >= 8 {
		return container.uuid[0:8]
	}
	return container.uuid
}

// Enter is used to load a console session within the container. It re-enters
// the container through the stage2 rather than through the initd so that it can
// easily stream in and out.
func (c *Container) Enter(cmdline []string, stdin, stdout, stderr *os.File, postStart func()) (*os.Process, error) {
	launcher := &client2.Launcher{
		Environment:   c.environment.Strings(),
		Taskfiles:     c.cgroup.TasksFiles(),
		Stdin:         stdin,
		Stdout:        stdout,
		Stderr:        stderr,
		User:          c.image.App.User,
		Group:         c.image.App.Group,
		Capabilities:  c.capabilities,
		PostStartFunc: postStart,
	}

	// If the command was blank, then use /bin/sh
	if len(cmdline) == 0 {
		cmdline = []string{"/bin/sh"}
	}

	// Check for a privileged isolator
	if iso := c.image.App.Isolators.GetByName(kschema.HostPrivilegedName); iso != nil {
		if piso, ok := iso.Value().(*kschema.HostPrivileged); ok {
			if *piso {
				launcher.HostPrivileged = true
			}
		}
	}

	// Get a process from the container and copy its namespaces
	tasks, err := c.cgroup.Tasks()
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no processes are running inside the container")
	}
	launcher.SetNS(tasks[0])

	// launch!
	c.log.Debugf("Executing command: %v", cmdline)
	p, err := launcher.Run(cmdline...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// getInitdClient is an accessor to get current initd client object. This should
// be used instead of accessing it directly because it retrives it within a
// mutex, and should then be set to a local variable. This is safest because on
// teardown, the initdClient is nil'd out and may cause a panic if another
// goroutine is still running and tries to use it.
func (c *Container) getInitdClient() client3.Client {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.initdClient
}

// markFailed is used to transition the container to the exited state.
func (c *Container) markExited() {
	c.mutex.Lock()
	if c.state != stage1.EXITED {
		close(c.waitch)
	}
	c.state = stage1.EXITED
	c.mutex.Unlock()
}

// Wait can be used to block until the processes within a container are finished
// executed. It is primarily intended for an internal API to code against system
// services.
func (c *Container) Wait() {
	<-c.waitch
}
