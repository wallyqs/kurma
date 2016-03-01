// Copyright 2016 Apcera Inc. All rights reserved.
// +build !windows

package capabilities

import (
	"fmt"
	"strings"

	"github.com/syndtr/gocapability/capability"
)

var capabilityList []string

func init() {
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

func GetAllCapabilities() []string {
	return capabilityList
}
