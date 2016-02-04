// Copyright 2015-2016 Apcera Inc. All rights reserved.

package stage1

import (
	"io"
	"os"

	"github.com/apcera/kurma/networking/types"
	kschema "github.com/apcera/kurma/schema"
	"github.com/apcera/kurma/stage1/graphstorage"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"
)

// ContainerState is used to track the basic state that a container is in, such
// as starting, running, stopped, or exited.
type ContainerState int

const (
	NEW = ContainerState(iota)
	STARTING
	RUNNING
	STOPPING
	STOPPED
	EXITED
)

func (c ContainerState) String() string {
	switch c {
	case NEW:
		return "NEW"
	case STARTING:
		return "STARTING"
	case RUNNING:
		return "RUNNING"
	case STOPPING:
		return "STOPPING"
	case STOPPED:
		return "STOPPED"
	case EXITED:
		return "EXITED"
	default:
		return ""
	}
}

// ImageManager is responsible for storing the extracted representation of
// images that are available for use for containers.
type ImageManager interface {
	// SetLog sets the logger to be used by the manager.
	SetLog(log *logray.Logger)

	// Rescan will reset the list of current images and reload it from disk.
	Rescan() error

	// CreateImage will process the provided reader to extract the image and make
	// it available for containers. It will return the image hash ID, image
	// manifest from within the image, or an error on any failures.
	CreateImage(reader io.Reader) (string, *schema.ImageManifest, error)

	// ListImages returns a map of the image hash to image manifest for all images
	// that are available.
	ListImages() map[string]*schema.ImageManifest

	// GetImage will return the image manifest for the provided image hash.
	GetImage(hash string) *schema.ImageManifest

	// FindImage will find the image manifest and hash for the specified name and
	// version label.
	FindImage(name, version string) (string, *schema.ImageManifest)

	// GetImageSize will return the on disk size of the image.
	GetImageSize(hash string) (int64, error)

	// DeleteImage will remove the specified image hash from disk.
	DeleteImage(hash string) error

	// ProvisionPod will create a PodStorage handler for the specified image hash
	// and at the specified destination pod directory. This will include resolving
	// all of the dependencies and launch a unionfs mount in a new mount namespace
	// in the PodStorage.
	ProvisionPod(hash, podDirectory string) (graphstorage.PodStorage, error)
}

// ContainerManager is responsible for the container lifecycle management.
type ContainerManager interface {
	// SetLog sets the logger to be used by the manager.
	SetLog(log *logray.Logger)

	// SetHostSocketFile sets the path to the host's socket file for granting API
	// access.
	SetHostSocketFile(hostSocketFile string)

	SetNetworkManager(NetworkManager)

	// Validate will ensure that the image manifest provided is valid to be run on
	// the system. It will return nil if it is valid, or will return an error if
	// something is invalid.
	Validate(imageManifest *schema.ImageManifest) error

	// Create begins launching a container with the provided image manifest and
	// reader as the source of the ACI.
	Create(name string, imageManifest *schema.ImageManifest, imageHash string) (Container, error)

	// Containers returns a slice of the current containers on the host.
	Containers() []Container

	// Container returns a specific container matching the provided UUID, or nil
	// if a container with the UUID does not exist.
	Container(uuid string) Container

	// SwapDirectory can be used to temporarily use a different container path for
	// an operation. This is a temporary hack util a Container object can specify
	// its own path.
	SwapDirectory(containerDirectory string, f func())
}

// Container represents the interactions that are possible with an individual
// instance running within the ContainerManager.
type Container interface {
	// UUID returns the UUID associated with the current Container.
	UUID() string

	// PodManifest returns the current pod manifest for the App Container
	// Specification.
	PodManifest() *kschema.PodManifest

	// ImageManifest returns the current image manifest for the App Container
	// Specification.
	ImageManifest() *schema.ImageManifest

	// Pid returns the pid of the top level process withint he container.
	Pid() (int, error)

	// State returns the current operating state of the container.
	State() ContainerState

	// Stop triggers the shutdown of the Container.
	Stop() error

	// Enter is used to load a console session within the container. It re-enters
	// the container through the stage2 rather than through the initd so that it
	// can easily stream in and out.
	Enter(cmdline []string, stdin, stdout, stderr *os.File, postStart func()) (*os.Process, error)

	// Wait can be used to block until the processes within a container are
	// finished executed. It is primarily intended for an internal API to code
	// against system services.
	Wait()
}

// NetworkManager is responsible for managing the list of configured network
// plugins, and communicating with the plugins for provisioning networking on
// individual containers.
type NetworkManager interface {
	// SetLog sets the logger to be used by the manager.
	SetLog(log *logray.Logger)

	// CreateDriver handles the launching of a new networking plugin within the
	// system.
	CreateDriver(imageManifest *schema.ImageManifest, imageHash string, config *types.NetConf) error

	// DeleteDriver handles removing a networking plugin from the system.
	DeleteDriver(name string) error

	// Provision handles setting up the networking for a new container. It is
	// responsible for instrumenting the necessary network plugins for the
	// container.
	Provision(container Container) ([]*types.IPResult, error)

	// Deprovision is called when a container is shutting down to handle any
	// deallocation or cleanup processes that are necessary.
	Deprovision(container Container) error
}
