// Copyright 2016 Apcera Inc. All rights reserved.
// getKernelVersion Copyright 2014 Google Inc. All Rights Reserved.

package server

import (
	"bytes"
	"syscall"
)

// getKernelVersion parses the result from uname() into a string representation
// of the kernel version.
func getKernelVersion() string {
	uname := &syscall.Utsname{}
	if err := syscall.Uname(uname); err != nil {
		return "Unknown"
	}

	release := make([]byte, len(uname.Release))
	i := 0
	for _, c := range uname.Release {
		release[i] = byte(c)
		i++
	}
	release = release[:bytes.IndexByte(release, 0)]

	return string(release)
}
