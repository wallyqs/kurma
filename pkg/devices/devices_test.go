// Copyright 2015 Apcera Inc. All rights reserved.

package devices

import (
	"testing"

	tt "github.com/apcera/util/testtool"
)

func TestGetRawDevice(t *testing.T) {
	tt.TestEqual(t, GetRawDevice("/dev/sda1"), "/dev/sda")
	tt.TestEqual(t, GetRawDevice("/dev/nvme0n1p3"), "/dev/nvme0n1")
}

func TestGetPartitionNumber(t *testing.T) {
	tt.TestEqual(t, GetPartitionNumber("/dev/sda1"), "1")
	tt.TestEqual(t, GetPartitionNumber("/dev/sda3"), "3")
	tt.TestEqual(t, GetPartitionNumber("/dev/nvme0n1p2"), "2")
	tt.TestEqual(t, GetPartitionNumber("/dev/sda"), "")
}
