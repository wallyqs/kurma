// Copyright 2015 Apcera Inc. All rights reserved.

package overlay

import "C"

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/apcera/kurma/stage1/graphstorage"
	"github.com/apcera/kurma/stage2/client"
	"github.com/apcera/kurma/util/cgroups"
	"github.com/apcera/util/proc"
)

type overlayProvisioner struct {
	cgroup *cgroups.Cgroup
}

type overlayPod struct {
	def     *overlayDefinition
	process *os.Process
	cgroup  *cgroups.Cgroup
}

type overlayDefinition struct {
	LowerDirectories     []string
	UpperDirectory       string
	WorkDirectory        string
	DestinationDirectory string
	finishedDirectory    string
}

// New returns a new graph storage provisioner that uses the overlay filesystem
// module for doing a union fileysstem.
func New() (graphstorage.StorageProvisioner, error) {
	// ensure overlay filesystem is available
	if err := loadOverlaySupport(); err != nil {
		return nil, err
	}

	cgroup, err := cgroups.New("kurma-overlay")
	if err != nil {
		return nil, err
	}

	return &overlayProvisioner{cgroup: cgroup}, nil
}

// Create will trigger the creation of an overlay mount at the specified
// location and with the included base containers in a new mount namespace. It
// will return a PodStorage object on success, or an error on any failures.
func (o *overlayProvisioner) Create(podDirectory string, imagedefintion []string) (graphstorage.PodStorage, error) {
	// create the defition and marshal it
	def := &overlayDefinition{
		LowerDirectories:     imagedefintion,
		UpperDirectory:       filepath.Join(podDirectory, "upper"),
		WorkDirectory:        filepath.Join(podDirectory, "work"),
		DestinationDirectory: filepath.Join(podDirectory, "dest"),
		finishedDirectory:    filepath.Join(podDirectory, "_overlay_finished"),
	}

	// create a cgroup for it
	cgroup, err := o.cgroup.New(filepath.Base(podDirectory))
	if err != nil {
		return nil, err
	}

	// launch the process to create the mount space
	l := client.Launcher{
		Taskfiles:         cgroup.TasksFiles(),
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
		Stdin:             nil,
		NewMountNamespace: true,
		Environment:       []string{"STORAGE_OVERLAY_INTERCEPT=1"},
	}

	// get the executable path to ourself
	self, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return nil, err
	}

	// populate the arguments
	args := []string{
		self,
		"--upperdir", def.UpperDirectory,
		"--workdir", def.WorkDirectory,
		"--destdir", def.DestinationDirectory,
		"--finishdir", def.finishedDirectory,
	}
	for _, l := range def.LowerDirectories {
		args = append(args, "--lowerdir", l)
	}

	// handle cleanup if it fails
	success := false
	defer func() {
		if !success {
			cgroup.SignalAll(syscall.SIGKILL)
		}
	}()

	process, err := l.Run(args...)
	if err != nil {
		return nil, err
	}

	if err := waitForFinished(def.finishedDirectory); err != nil {
		return nil, err
	}

	success = true

	// return the pod
	return &overlayPod{def: def, process: process, cgroup: cgroup}, nil
}

func waitForFinished(path string) error {
	start := time.Now()
	startupDeadline := start.Add(time.Second * 2)
	for ; ; time.Sleep(time.Millisecond * 10) {
		if time.Now().After(startupDeadline) {
			return fmt.Errorf("Pod storage failed to be configured properly before timeout.")
		}
		if _, err := os.Stat(path); err == nil {
			break
		}
	}

	os.Remove(path)
	return nil
}

// NS returns the pid to use for the mount namespace.
func (p *overlayPod) NS() int {
	return p.process.Pid
}

// Returns the directory that the host has visibility of and can write to.
func (p *overlayPod) HostRoot() string {
	return p.def.UpperDirectory
}

// Returns the directory that references the root location.
func (p *overlayPod) Root() string {
	return p.def.DestinationDirectory
}

// MarkRunning is used for any semi-cleanup operations needed once the pod for
// the filesystem is running and health.
func (p *overlayPod) MarkRunning() {
	p.process.Release()
	p.cgroup.SignalAll(syscall.SIGKILL)
}

// Cleanup is used once a pod has been torn down and is no longer running.
func (p *overlayPod) Cleanup() error {
	p.process.Release()
	p.cgroup.SignalAll(syscall.SIGKILL)
	return nil
}

// loadOverlaySupport will ensure the overlay filesystem is available for
// use. It will return an error if it is unavailable or fails to load the
// associated kernel module.
func loadOverlaySupport() error {
	// Check to see if overlay is already available. It could be compiled into the
	// kernel, or the module could be already loaded.
	avail, err := checkIfOverlayIsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if overlay filesystem was supported: %v", err)
	}
	if avail {
		return nil
	}

	// It is not available yet, so load the module.
	if b, err := exec.Command("modprobe", "overlay").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load the overlay module: %s", string(b))
	}

	// recheck that it is available
	avail, err = checkIfOverlayIsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if overlay filesystem was supported: %v", err)
	}
	if !avail {
		return fmt.Errorf("overlay filesystem support unavailable after loading the module")
	}
	return nil
}

// checkIfOverlayIsAvailable scans the /proc/filesystems file to see if overlay
// is listed as a filesystem type that is available.
func checkIfOverlayIsAvailable() (bool, error) {
	available := false
	err := proc.ParseSimpleProcFile("/proc/filesystems", nil,
		func(line, index int, elem string) error {
			if elem == "overlay" {
				available = true
			}
			return nil
		},
	)
	return available, err
}
