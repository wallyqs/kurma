// Copyright 2013-2015 Apcera Inc. All rights reserved.

// +build linux,cgo

package stage3_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/apcera/util/uuid"

	. "github.com/apcera/util/testtool"
)

func TestMountRequest(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)
	TestRequiresRoot(t)

	// Start the initd process.
	_, socket, _, pid := StartInitd(t)

	content := uuid.Variant4().String()

	tempDir, err := ioutil.TempDir("/var", "container"+uuid.Variant4().String())
	TestExpectSuccess(t, err)
	AddTestFinalizer(func() { os.RemoveAll(tempDir) })
	err = os.Chmod(tempDir, os.FileMode(0755))
	TestExpectSuccess(t, err)
	TestExpectSuccess(t, ioutil.WriteFile(filepath.Join(tempDir, "foo"), []byte(content), os.FileMode(0644)))

	otherTempDir, err := ioutil.TempDir("/var", "container"+uuid.Variant4().String())
	TestExpectSuccess(t, err)
	AddTestFinalizer(func() { os.RemoveAll(otherTempDir) })

	request := [][]string{
		[]string{"MOUNT", tempDir, otherTempDir},
		[]string{"", fmt.Sprintf("%d", syscall.MS_BIND), ""},
	}
	reply, err := MakeRequest(socket, request, 10*time.Second)
	TestExpectSuccess(t, err)
	TestEqual(t, reply, "REQUEST OK\n")

	// Next check to see that the init daemon is mounted the path correctly.
	contents, err := ioutil.ReadFile(filepath.Join(fmt.Sprintf("/proc/%d/root", pid), otherTempDir, "foo"))
	TestExpectSuccess(t, err)
	TestEqual(t, string(contents), content)
}

func TestBadMountRequest(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)
	TestRequiresRoot(t)

	tests := [][][]string{
		// Test 1: Request is too short.
		[][]string{
			[]string{"MOUNT", "SRC", "DST"},
		},

		// Test 2: Request is too long..
		[][]string{
			[]string{"MOUNT", "SRC", "DST"},
			[]string{"FS", "FLAGS", ""},
			[]string{},
		},

		// Test 3: Extra cruft after DST
		[][]string{
			[]string{"MOUNT", "SRC", "DST", "EXTRA"},
			[]string{"FS", "0", ""},
		},

		// Test 4: Extra cruft after CREATE
		[][]string{
			[]string{"MOUNT", "SRC", "DST"},
			[]string{"FS", "0", "", "EXTRA"},
		},

		// Test 5: Missing SRC
		[][]string{
			[]string{"MOUNT", "", "DST"},
			[]string{"FS", "0", ""},
		},

		// Test 6: Missing DST
		[][]string{
			[]string{"MOUNT", "SRC"},
			[]string{"FS", "0", ""},
		},

		// Test 7: Missing FLAGS
		[][]string{
			[]string{"MOUNT", "SRC", "DST"},
			[]string{"FS", "", ""},
		},
	}
	BadResultsCheck(t, tests)
}
