// Copyright 2016 Apcera Inc. All rights reserved.

package aufs

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

type aufsProvisioner struct {
}

// New returns a new graph storage provisioner that uses the aufs filesystem
// module for doing a union fileysstem.
func New() (graphstorage.StorageProvisioner, error) {
	// ensure aufs filesystem is available
	if err := loadAufsSupport(); err != nil {
		return nil, err
	}

	return &aufsProvisioner{}, nil
}

// Create will trigger the creation of an aufs mount at the specified
// location and with the included base containers in a new mount namespace. It
// will return a PodStorage object on success, or an error on any failures.
func (o *aufsProvisioner) Create(target string, imagedefintion []string) error {
	scratch, err := ioutil.TempDir(os.TempDir(), "scratch")
	if err != nil {
		return fmt.Errorf("failed to create aufs write branch: %v", err)
	}

	// Mount the read/write portion of aufs first.
	err = syscall.Mount("none", target, "aufs", syscall.MS_MGC_VAL, fmt.Sprintf("br=%s=rw", scratch))
	if err != nil {
		return fmt.Errorf("failed to mount aufs write branch: %v", err)
	}

	// Mount each layer individually. Doing them as separate calls avoids issues
	// with a single mount call with a lot of layers failing.
	for _, imagePath := range imagedefintion {
		err := syscall.Mount("none", target, "aufs", syscall.MS_REMOUNT, fmt.Sprintf("append:%s=ro+wh", imagePath))
		if err != nil {
			return fmt.Errorf("failed to mount layer %q: %v", imagePath, err)
		}
	}

	return nil
}

// loadAufsSupport will ensure the aufs filesystem is available for use. It will
// return an error if it is unavailable or fails to load the associated kernel
// module.
func loadAufsSupport() error {
	// Check to see if aufs is already available. It could be compiled into the
	// kernel, or the module could be already loaded.
	avail, err := checkIfAufsIsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if aufs filesystem was supported: %v", err)
	}
	if avail {
		return nil
	}

	// If the filesystem type isn't available, then check if the module is available
	avail, err = checkIfAufsModuleAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if aufs module is available: %v", err)
	}
	if !avail {
		return fmt.Errorf("aufs module is not available")
	}

	// It is not available yet, so load the module.
	if b, err := exec.Command("modprobe", "aufs").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load the aufs module: %s - %v", string(b), err)
	}

	// recheck that it is available
	avail, err = checkIfAufsIsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if aufs filesystem was supported: %v", err)
	}
	if !avail {
		return fmt.Errorf("aufs filesystem support unavailable after loading the module")
	}
	return nil
}

// checkIfAufsIsAvailable scans the /proc/filesystems file to see if aufs is
// listed as a filesystem type that is available.
func checkIfAufsIsAvailable() (bool, error) {
	available := false
	err := proc.ParseSimpleProcFile("/proc/filesystems", nil,
		func(line, index int, elem string) error {
			if elem == "aufs" {
				available = true
			}
			return nil
		},
	)
	return available, err
}

// checkIfAufsModuleAvailable checks if the aufs module is available in
// the modules.alias file. This is more efficient than attempting to load the
// module when it doesn't exist.
func checkIfAufsModuleAvailable() (bool, error) {
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
		if parts[len(parts)-1] != "aufs" {
			continue
		}
		return true, nil
	}
	return false, nil
}
