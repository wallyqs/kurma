// Copyright 2016 Apcera Inc. All rights reserved.

package kurmad

import (
	"fmt"
	"testing"
)

type dummyLoadConfigError struct {
	ErrorCode int
}

func (e *dummyLoadConfigError) Error() string {
	return fmt.Sprintf("failed to load config (code: %d)", e.ErrorCode)
}

type dummyCreatePodManagerError struct {
	ErrorCode int
}

func (e *dummyCreatePodManagerError) Error() string {
	return fmt.Sprintf("failed to create a pod manager (code: %d)", e.ErrorCode)
}

type dummyFailRunner struct{}

func (r *dummyFailRunner) setupSignalHandling() {}
func (r *dummyFailRunner) loadConfigurationFile() error {
	return &dummyLoadConfigError{ErrorCode: 42}
}
func (r *dummyFailRunner) configureLogging()         {}
func (r *dummyFailRunner) createDirectories() error  { return nil }
func (r *dummyFailRunner) createImageManager() error { return nil }
func (r *dummyFailRunner) prefetchImages()           {}
func (r *dummyFailRunner) createPodManager() error {
	return &dummyCreatePodManagerError{ErrorCode: 90}
}
func (r *dummyFailRunner) createNetworkManager() {}
func (r *dummyFailRunner) startDaemon() error    { return nil }
func (r *dummyFailRunner) startInitialPods()     {}

func TestBootstrapFailure(t *testing.T) {
	r := &dummyFailRunner{}
	err := bootstrap(r)
	if err == nil {
		t.Fatal("expected for bootstrap to have failed")
	}
	if loadConfigErr, ok := err.(*dummyLoadConfigError); !ok {
		t.Fatal("expected to have a load config error")
	} else {
		got := loadConfigErr.ErrorCode
		expected := 42
		if got != expected {
			t.Fatalf("expected error code to have been %d. got: %d", expected, got)
		}
	}
}

type dummyOKRunner struct{}

func (r *dummyOKRunner) setupSignalHandling()         {}
func (r *dummyOKRunner) loadConfigurationFile() error { return nil }
func (r *dummyOKRunner) configureLogging()            {}
func (r *dummyOKRunner) createDirectories() error     { return nil }
func (r *dummyOKRunner) createImageManager() error    { return nil }
func (r *dummyOKRunner) prefetchImages()              {}
func (r *dummyOKRunner) createPodManager() error      { return nil }
func (r *dummyOKRunner) createNetworkManager()        {}
func (r *dummyOKRunner) startDaemon() error           { return nil }
func (r *dummyOKRunner) startInitialPods()            {}

func TestBootstrapOK(t *testing.T) {
	r := &dummyOKRunner{}
	err := bootstrap(r)
	if err != nil {
		t.Fatal("expected no issues during bootstrap")
	}
}
