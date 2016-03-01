// Copyright 2015-2016 Apcera Inc. All rights reserved.

package stage1

import (
	"encoding/json"
	"io"
	"os"

	ntypes "github.com/apcera/kurma/network/types"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

// PodState is used to track the basic state that a pod is in, such
// as starting, running, stopped, or exited.
type PodState int

const (
	NEW = PodState(iota)
	STARTING
	RUNNING
	STOPPING
	STOPPED
	EXITED
)

func (c PodState) String() string {
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
// images that are available for use for pods.
type ImageManager interface {
	// Rescan will reset the list of current images and reload it from disk.
	Rescan() error

	// CreateImage will process the provided reader to extract the image and make
	// it available for pods. It will return the image hash ID, image
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

	// ResolveTree will resolve the dependency tree for the specified image. It
	// will return a []string returning the order images should be merged, the
	// []string with all the relevant image paths on disk, the map of all the
	// image manifests, or an error if there is any resolution issue.
	ResolveTree(hash string) (*ResolutionTree, error)
}

// ResolutionTree contains all of the metadata about the resolved set of
// packages for a pod.
type ResolutionTree struct {
	// Order is the ordering of the images as they should be merged, from the
	// top most layer (the one requested) to the bottom.
	Order []string

	// Paths is a may of the layer ID and the path to its filesystem on disk.
	Paths map[string]string

	// Manifests it the map of the hash ID to the image manifest for all of the
	// images involved.
	Manifests map[string]*schema.ImageManifest
}

// PodManager is responsible for the pod lifecycle management.
type PodManager interface {
	// SetHostSocketFile sets the path to the host's socket file for granting API
	// access.
	SetHostSocketFile(hostSocketFile string)

	// SetNetworkManager sets the network manager that should be used to configure
	// networking for pods.
	SetNetworkManager(NetworkManager)

	// Create begins launching a pod with the provided image manifest and
	// reader as the source of the ACI.
	Create(name string, manifest *schema.PodManifest) (Pod, error)

	// Pods returns a slice of the current pods on the host.
	Pods() []Pod

	// Pod returns a specific pod matching the provided UUID, or nil
	// if a pod with the UUID does not exist.
	Pod(uuid string) Pod

	// SwapDirectory can be used to temporarily use a different pod path for
	// an operation. This is a temporary hack util a Pod object can specify
	// its own path.
	SwapDirectory(podDirectory string, f func())
}

// StagerManifest is the information that is passed over to the pod lifecycle
// manager. This contains the pod's manifest, all of the image manifests
// involved, as well as information the order the image layers should be applied
// for each app in the pod.
type StagerManifest struct {
	// The pod manifest.
	Pod *schema.PodManifest `json:"pod"`

	// Images is the map of hash ID to ImageManifests.
	Images map[string]*schema.ImageManifest `json:"images"`

	// AppImageOrder is a map of app name to the []string order of image layers as
	// they should be applied to the filesystem, with the app's primary image
	// first and the bottom most layer last.
	AppImageOrder map[string][]string `json:"appImageOrder"`

	// StagerConfig is an arbitruary JSON configuration that will be passed along
	// for the stager.
	StagerConfig json.RawMessage `json:"stagerConfig"`
}

// Pod represents the interactions that are possible with an individual
// instance running within the PodManager.
type Pod interface {
	// UUID returns the UUID associated with the current Pod.
	UUID() string

	// Name returns the Name given to the current Pod.
	Name() string

	// PodManifest returns the current pod manifest for the App Pod
	// Specification.
	PodManifest() *schema.PodManifest

	// State returns the current operating state of the pod.
	State() PodState

	// Stop triggers the shutdown of the Pod.
	Stop() error

	// Enter is used to load a console session within the pod. It re-enters
	// the pod through the stage2 rather than through the initd so that it
	// can easily stream in and out.
	Enter(appName string, app *types.App, stdin, stdout, stderr *os.File, postStart func()) (*os.Process, error)

	// Wait can be used to block until the processes within a pod are
	// finished executed. It is primarily intended for an internal API to code
	// against system services.
	Wait()
}

// NetworkManager is responsible for managing the list of configured network
// plugins, and communicating with the plugins for provisioning networking on
// individual pods.
type NetworkManager interface {
	// SetLog sets the logger to be used by the manager.
	SetLog(log *logray.Logger)

	// CreateDriver handles the launching of a new networking plugin within the
	// system.
	CreateDriver(imageManifest *schema.ImageManifest, imageHash string, config *ntypes.NetConf) error

	// DeleteDriver handles removing a networking plugin from the system.
	DeleteDriver(name string) error

	// Provision handles setting up the networking for a new pod. It is
	// responsible for instrumenting the necessary network plugins for the
	// pod.
	Provision(pod Pod) ([]*ntypes.IPResult, error)

	// Deprovision is called when a pod is shutting down to handle any
	// deallocation or cleanup processes that are necessary.
	Deprovision(pod Pod) error
}
