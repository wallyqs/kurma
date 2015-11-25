// Copyright 2015 Apcera Inc. All rights reserved.

package graphstorage

// StorageProvisioner is a generic interface for the configuration and
// management of pod filesystems.
type StorageProvisioner interface {
	// Create is used to generate a unioned filesystem based on the specified set
	// of images. The imagedefintion contains a list of the directory of the
	// extracted image's rootfs in order from the top most image to the bottom. It
	// should return a PodStorage object on success, or an error on any failure.
	Create(podDirectory string, imagedefinition []string) (PodStorage, error)
}

type PodStorage interface {
	// NS returns the pid to use for the mount namespace.
	NS() int

	// Returns the directory that the host has visibility of and can write to.
	HostRoot() string

	// Returns the directory that references the root location.
	Root() string

	// MarkRunning is used for any semi-cleanup operations needed once the pod for
	// the filesystem is running and health.
	MarkRunning()

	// Cleanup is used once a pod has been torn down and is no longer running.
	Cleanup() error
}
