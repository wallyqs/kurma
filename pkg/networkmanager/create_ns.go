// Copyright 2016 Apcera Inc. All rights reserved.

package networkmanager

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

func init() {
	if len(os.Args) == 4 && os.Args[1] == "create-ns-mount" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		createNamespaceMount()
		os.Exit(0)
	}
}

// createNetworkNamespace will create a new network namespace and bind mount it
// to the specified path.
func createNetworkNamespace(bindpath string) error {
	// We will shell out to ourself and execute with a parameter to have us create
	// the necessary bind mount for the namespace. It was found this was more
	// reliable, since locking the OS thread and creating a namespace, was still
	// getting it where ~75% of the time, reading the link for /proc/self/ns/net
	// was still returning the same namespace.

	// get the executable path to ourself
	self, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %v", err)
	}

	// Pre-create our bind mount location.
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

	cmd := exec.Command(self, "create-ns-mount", "net", bindpath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNET,
	}
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create network namespace failed: %s", string(b))
	}

	success = true
	return nil
}

// createNamespaceMount performs the actual mount on the new execution.
func createNamespaceMount() {
	// Perform the mount
	if err := syscall.Mount(fmt.Sprintf("/proc/self/ns/%s", os.Args[2]), os.Args[3], "", syscall.MS_BIND, ""); err != nil {
		panic(err)
	}
}

// deleteNetworkNamespace handles unmounting a network namespace path
func deleteNetworkNamespace(bindpath string) error {
	if _, err := os.Stat(bindpath); err != nil && os.IsNotExist(err) {
		return nil
	}

	if err := syscall.Unmount(bindpath, 0); err != nil {
		return fmt.Errorf("failed to unmount namespace path: %v", err)
	}
	if err := os.Remove(bindpath); err != nil {
		return fmt.Errorf("failed to remove namespace path: %v", err)
	}
	return nil
}
