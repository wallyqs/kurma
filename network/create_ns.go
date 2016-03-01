// Copyright 2016 Apcera Inc. All rights reserved.

package network

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/appc/cni/pkg/ns"
)

// CreateNetworkNamespace will create a new network namespace and bind mount it
// to the specified path.
func CreateNetworkNamespace(bindpath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// first, open our current host ns
	hostns, err := os.Open("/proc/self/ns/net")
	if err != nil {
		return fmt.Errorf("failed to open the host ns: %v", err)
	}

	// Pre-create our bind mount location. Prefer to minimize the time in the
	// other namespace.
	f, err := os.OpenFile(bindpath, os.O_CREATE|os.O_EXCL, os.FileMode(0600))
	if err != nil {
		return fmt.Errorf("failed to create bind mount destination: %v", err)
	}
	f.Close()
	success := false
	defer func() {
		if !success {
			os.Remove(bindpath)
		}
	}()

	// Clone the new namespace
	syscall.Unshare(syscall.CLONE_NEWNET)
	defer func() {
		if err := ns.SetNS(hostns, syscall.CLONE_NEWNET); err != nil {
			// intentionally panic, because the thread is no longer in the same
			// namespace
			panic(err)
		}
	}()

	// Perform the mount
	if err := syscall.Mount("/proc/self/ns/net", bindpath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to bind mount namespace: %v", err)
	}
	success = true
	return nil
}
