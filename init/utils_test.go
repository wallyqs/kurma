// Copyright 2015 Apcera Inc. All rights reserved.

package init

import (
	"testing"

	tt "github.com/apcera/util/testtool"
)

func TestGetRawDevice(t *testing.T) {
	tt.TestEqual(t, getRawDevice("/dev/sda1"), "/dev/sda")
	tt.TestEqual(t, getRawDevice("/dev/nvme0n1p3"), "/dev/nvme0n1")
}

func TestGetPartitionNumber(t *testing.T) {
	tt.TestEqual(t, getPartitionNumber("/dev/sda1"), "1")
	tt.TestEqual(t, getPartitionNumber("/dev/sda3"), "3")
	tt.TestEqual(t, getPartitionNumber("/dev/nvme0n1p2"), "2")
	tt.TestEqual(t, getPartitionNumber("/dev/sda"), "")
}
