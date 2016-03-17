// Copyright 2015 Apcera Inc. All rights reserved.

package overlay

import "C"

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/apcera/kurma/pkg/graphstorage"
	"github.com/apcera/util/proc"
)

type overlayProvisioner struct {
}

type overlayDefinition struct {
	LowerDirectories     []string
	UpperDirectory       string
	WorkDirectory        string
	DestinationDirectory string
	finishedDirectory    string
}

// New returns a new graph storage provisioner that uses the overlay filesystem
// module for doing a union fileysstem.
func New() (graphstorage.StorageProvisioner, error) {
	// ensure overlay filesystem is available
	if err := loadOverlaySupport(); err != nil {
		return nil, err
	}

	return &overlayProvisioner{}, nil
}

// Create will trigger the creation of an overlay mount at the specified
// location and with the included base containers in a new mount namespace. It
// will return a PodStorage object on success, or an error on any failures.
func (o *overlayProvisioner) Create(target string, imagedefintion []string) error {

	upper, err := ioutil.TempDir(os.TempDir(), "upper")
	if err != nil {
		return err
	}
	work, err := ioutil.TempDir(os.TempDir(), "work")
	if err != nil {
		return err
	}

	lower := strings.Join(imagedefintion, ":")
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	if err := syscall.Mount("overlay", target, "overlay", 0, opts); err != nil {
		return fmt.Errorf("failed to mount storage: %v", err)
	}
	return nil
}

// loadOverlaySupport will ensure the overlay filesystem is available for
// use. It will return an error if it is unavailable or fails to load the
// associated kernel module.
func loadOverlaySupport() error {
	// Check to see if overlay is already available. It could be compiled into the
	// kernel, or the module could be already loaded.
	avail, err := checkIfOverlayIsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if overlay filesystem was supported: %v", err)
	}
	if avail {
		return nil
	}

	// It is not available yet, so load the module.
	if b, err := exec.Command("modprobe", "overlay").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load the overlay module: %s", string(b))
	}

	// recheck that it is available
	avail, err = checkIfOverlayIsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if overlay filesystem was supported: %v", err)
	}
	if !avail {
		return fmt.Errorf("overlay filesystem support unavailable after loading the module")
	}
	return nil
}

// checkIfOverlayIsAvailable scans the /proc/filesystems file to see if overlay
// is listed as a filesystem type that is available.
func checkIfOverlayIsAvailable() (bool, error) {
	available := false
	err := proc.ParseSimpleProcFile("/proc/filesystems", nil,
		func(line, index int, elem string) error {
			if elem == "overlay" {
				available = true
			}
			return nil
		},
	)
	return available, err
}
