// Copyright 2015 Apcera Inc. All rights reserved.

package graphstorage

// StorageProvisioner is a generic interface for the configuration and
// management of pod filesystems.
type StorageProvisioner interface {
	// Create is used to generate a unioned filesystem based on the specified set
	// of images. The imagedefintion contains a list of the directory of the
	// extracted image's rootfs in order from the top most image to the bottom. It
	// should return a PodStorage object on success, or an error on any failure.
	Create(podDirectory string, imagedefinition []string) error
}
