// Copyright 2016 Apcera Inc. All rights reserved.
// +build !windows

package capabilities

import (
	"fmt"
	"os"
	"strings"

	"github.com/syndtr/gocapability/capability"
)

var capabilityList []string

func init() {
	RefreshCapabilities()
}

func GetAllCapabilities() []string {
	return capabilityList
}

// Copied from github.com/syndtr/gocapability, because kurmaOS is started before
// proc is mounted, this read may fail in its init() func. It is replicated here
// so we can recalculate the last cap after proc is mounted.
func initLastCap() (capability.Cap, error) {
	var capLastCap capability.Cap
	f, err := os.Open("/proc/sys/kernel/cap_last_cap")
	if err != nil {
		return capability.Cap(0), err
	}
	defer f.Close()

	var b []byte = make([]byte, 11)
	_, err = f.Read(b)
	if err != nil {
		return capability.Cap(0), err
	}

	fmt.Sscanf(string(b), "%d", &capLastCap)

	return capLastCap, nil
}

func RefreshCapabilities() {
	if lastCap, err := initLastCap(); err == nil {
		capability.CAP_LAST_CAP = lastCap
	}

	capabilityList = make([]string, 0)
	last := capability.CAP_LAST_CAP
	// hack for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}
	for _, cap := range capability.List() {
		if cap > last {
			continue
		}
		capabilityList = append(capabilityList, fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String())))
	}
}
