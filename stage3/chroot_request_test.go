// Copyright 2013-2015 Apcera Inc. All rights reserved.

// +build linux,cgo

package stage3_test

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/apcera/util/uuid"

	. "github.com/apcera/util/testtool"
)

func TestChrootRequest(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)
	TestRequiresRoot(t)

	// Start the initd process.
	_, socket, _, _ := StartInitd(t)

	chrootDir, err := ioutil.TempDir("/var", "container"+uuid.Variant4().String())
	TestExpectSuccess(t, err)
	AddTestFinalizer(func() { os.RemoveAll(chrootDir) })
	err = os.Chmod(chrootDir, os.FileMode(0755))
	TestExpectSuccess(t, err)

	request := [][]string{[]string{"CHROOT", chrootDir}}
	reply, err := MakeRequest(socket, request, 10*time.Second)
	TestExpectSuccess(t, err)
	TestEqual(t, reply, "REQUEST OK\n")
}

func TestBadChrootcRequest(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)
	TestRequiresRoot(t)

	tests := [][][]string{
		// Test 1: Request is too long.
		[][]string{
			[]string{"CHROOT", "DIR", "FALSE"},
			[]string{"EXTRA"},
		},

		// Test 2: Request is missing a directory.
		[][]string{
			[]string{"CHROOT"},
		},

		// Test 3: Extra cruft.
		[][]string{
			[]string{"CHROOT", "DIR", "FALSE", "EXTRA"},
		},
	}
	BadResultsCheck(t, tests)
}
