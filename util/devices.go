// Copyright 2015 Apcera Inc. All rights reserved.
//
// This file is based on code from:
//   https://github.com/rancherio/os
//
// Code is licensed under Apache 2.0.
// Copyright (c) 2014-2015 Rancher Labs, Inc.

package util

/*
#cgo LDFLAGS: -lmount -lblkid
#include<blkid/blkid.h>
#include<libmount/libmount.h>
#include<stdlib.h>
*/
import "C"
import "unsafe"

import (
	"errors"
	"regexp"
)

func ResolveDevice(spec string) string {
	cSpec := C.CString(spec)
	defer C.free(unsafe.Pointer(cSpec))
	cString := C.blkid_evaluate_spec(cSpec, nil)
	defer C.free(unsafe.Pointer(cString))
	return C.GoString(cString)
}

func GetFsType(device string) (string, error) {
	var ambi *C.int
	cDevice := C.CString(device)
	defer C.free(unsafe.Pointer(cDevice))
	cString := C.mnt_get_fstype(cDevice, ambi, nil)
	defer C.free(unsafe.Pointer(cString))
	if cString != nil {
		return C.GoString(cString), nil
	}
	return "", errors.New("Error while getting fstype")
}

func intToBool(value C.int) bool {
	if value == 0 {
		return false
	}
	return true
}

var rawDeviceRegexp = regexp.MustCompile(`([0-9])$`)
var rawDevicePRegexp = regexp.MustCompile(`.*[0-9](p)$`)

// GetRawDevice takes in the device path to a partition and will return the
// device path to the raw block device.
func GetRawDevice(dev string) string {
	rawdev := rawDeviceRegexp.ReplaceAllString(dev, "")

	// handle devices that end in a number, and then get a p before the partition
	// number. ie, /dev/nvme0n1p1, where the raw device is /dev/nvme0n1
	if rawDevicePRegexp.MatchString(rawdev) {
		rawdev = rawdev[0 : len(rawdev)-1]
	}

	return rawdev
}

// GetPartitionNumber parses out the numerical partition number from the device
// string.
func GetPartitionNumber(dev string) string {
	matches := rawDeviceRegexp.FindAllString(dev, 1)
	if len(matches) != 1 {
		return ""
	}
	return matches[0]
}
