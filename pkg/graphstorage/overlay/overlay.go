// Copyright 2015 Apcera Inc. All rights reserved.

package overlay

import "C"

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/apcera/kurma/pkg/graphstorage"
	"github.com/apcera/kurma/pkg/misc"
	"github.com/apcera/util/proc"
)

type overlayProvisioner struct {
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
// location and with the included base image paths. It will return an error on
// any failures.
func (o *overlayProvisioner) Create(target string, imagedefinition []string) error {
	upper, err := ioutil.TempDir(os.TempDir(), "upper")
	if err != nil {
		return err
	}
	work, err := ioutil.TempDir(os.TempDir(), "work")
	if err != nil {
		return err
	}

	lower := strings.Join(imagedefinition, ":")
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

	// If the filesystem type isn't available, then check if the module is available
	avail, err = checkIfOverlayModuleAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if overlay module is available: %v", err)
	}
	if !avail {
		return fmt.Errorf("overlay module is not available")
	}

	// It is not available yet, so load the module.
	if b, err := exec.Command("modprobe", "overlay").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load the overlay module: %s - %v", string(b), err)
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

// checkIfOverlayModuleAvailable checks if the overlay module is available in
// the modules.alias file. This is more efficient than attempting to load the
// module when it doesn't exist.
func checkIfOverlayModuleAvailable() (bool, error) {
	aliasPath := filepath.Join("/lib/modules", misc.GetKernelVersion(), "modules.alias")
	f, err := os.Open(aliasPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 3 {
			continue
		}
		if parts[0] != "alias" {
			continue
		}
		if parts[len(parts)-1] != "overlay" {
			continue
		}
		return true, nil
	}
	return false, nil
}
