// Copyright 2013-2015 Apcera Inc. All rights reserved.

// +build linux,cgo

package capability

import (
	"os"
	"testing"

	tt "github.com/apcera/util/testtool"
)

func TestCapValue(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	for i := 0; i < int(CAP_LAST); i++ {
		CapValue(i).capT()
	}
	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("Panic not called.")
		}
	}()
	CapValue(CAP_LAST).capT()
}

func TestCapValueFromName(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	capv, err := FromName("CAP_KILL")
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, capv, CAP_KILL)
}

func TestEffectiveCapability(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	cap, err := NewFromPid(os.Getpid())
	tt.TestExpectSuccess(t, err)

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("Panic not called.")
		}
	}()
	for i := 0; i <= int(CAP_LAST); i++ {
		c := CapValue(i)
		if _, err := cap.EffectiveCapability(c); err != nil {
			t.Fatalf("Error returned %d: %s", i, err)
		}
	}
}

func TestPermittedCapability(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	cap, err := NewFromPid(os.Getpid())
	tt.TestExpectSuccess(t, err)

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("Panic not called.")
		}
	}()
	for i := 0; i <= int(CAP_LAST); i++ {
		c := CapValue(i)
		if _, err := cap.PermittedCapability(c); err != nil {
			t.Fatalf("Error returned %d: %s", i, err)
		}
	}
}

func TestInheritableCapability(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	cap, err := NewFromPid(os.Getpid())
	tt.TestExpectSuccess(t, err)

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("Panic not called.")
		}
	}()
	for i := 0; i <= int(CAP_LAST); i++ {
		c := CapValue(i)
		if _, err := cap.InheritableCapability(c); err != nil {
			t.Fatalf("Error returned %d: %s", i, err)
		}
	}
}

func TestSetFunctions(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	cap, err := NewFromPid(os.Getpid())
	tt.TestExpectSuccess(t, err)

	cap.Clear()

	val, err := cap.EffectiveCapability(CAP_KILL)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, val, false)

	val, err = cap.InheritableCapability(CAP_KILL)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, val, false)

	val, err = cap.PermittedCapability(CAP_KILL)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, val, false)

	tt.TestExpectSuccess(t, cap.SetEffectiveCapability(CAP_KILL, true))
	tt.TestExpectSuccess(t, cap.SetInheritableCapability(CAP_KILL, true))
	tt.TestExpectSuccess(t, cap.SetPermittedCapability(CAP_KILL, true))

	val, err = cap.EffectiveCapability(CAP_KILL)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, val, true)

	val, err = cap.InheritableCapability(CAP_KILL)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, val, true)

	val, err = cap.PermittedCapability(CAP_KILL)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, val, true)
}

func TestCapStringConverstion(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	cap, err := NewFromPid(os.Getpid())
	tt.TestExpectSuccess(t, err)

	cap.Clear()

	val, err := cap.EffectiveCapability(CAP_KILL)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, val, false)

	err = cap.SetEffectiveCapability(CAP_KILL, true)
	tt.TestExpectSuccess(t, err)

	val, err = cap.EffectiveCapability(CAP_KILL)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, val, true)

	str := cap.String()
	tt.TestEqual(t, str, "= cap_kill+e")
}
