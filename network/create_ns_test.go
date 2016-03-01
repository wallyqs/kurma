// Copyright 2016 Apcera Inc. All rights reserved.

package network

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/appc/cni/pkg/ns"

	tt "github.com/apcera/util/testtool"
)

func TestCreateNetworkNamespace(t *testing.T) {
	tt.TestRequiresRoot(t)

	tt.StartTest(t)
	defer tt.FinishTest(t)

	tmpdir := tt.TempDir(t)
	dest := filepath.Join(tmpdir, "netns")

	tt.TestExpectSuccess(t, CreateNetworkNamespace(dest))
	tt.AddTestFinalizer(func() {
		syscall.Unmount(dest, 0)
	})

	hostnics, err := net.Interfaces()
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, len(hostnics) > 1, true, "expect more than 1 host NIC")

	var nsnics []net.Interface

	err = ns.WithNetNSPath(dest, true, func(*os.File) error {
		var err error
		nsnics, err = net.Interfaces()
		return err
	})
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, len(nsnics), 1)
	tt.TestEqual(t, nsnics[0].Name, "lo")
}
