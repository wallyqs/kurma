// Copyright 2016 Apcera Inc. All rights reserved.

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/apcera/kurma/stager/container/common"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"

	kschema "github.com/apcera/kurma/schema"
)

const (
	defaultMountFlags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
)

var (
	defaultContainerMounts = []*configs.Mount{
		{
			Source:      "proc",
			Destination: "/proc",
			Device:      "proc",
			Flags:       defaultMountFlags,
		},
		{
			Source:      "tmpfs",
			Destination: "/dev",
			Device:      "tmpfs",
			Flags:       syscall.MS_NOSUID | syscall.MS_STRICTATIME,
			Data:        "mode=755",
		},
		{
			Source:      "devpts",
			Destination: "/dev/pts",
			Device:      "devpts",
			Flags:       syscall.MS_NOSUID | syscall.MS_NOEXEC,
			Data:        "newinstance,ptmxmode=0666,mode=0620,gid=5",
		},
		{
			Device:      "tmpfs",
			Source:      "shm",
			Destination: "/dev/shm",
			Data:        "mode=1777,size=65536k",
			Flags:       defaultMountFlags,
		},
		{
			Source:      "mqueue",
			Destination: "/dev/mqueue",
			Device:      "mqueue",
			Flags:       defaultMountFlags,
		},
		{
			Source:      "sysfs",
			Destination: "/sys",
			Device:      "sysfs",
			Flags:       defaultMountFlags | syscall.MS_RDONLY,
		},
		{
			Source:      "cgroup",
			Destination: "/sys/fs/cgroup",
			Device:      "cgroup",
			Flags:       defaultMountFlags | syscall.MS_RDONLY,
		},
	}
)

func (cs *containerSetup) getInitContainerConfig() (*configs.Config, error) {
	config := &configs.Config{
		ParentDeathSignal: int(syscall.SIGTERM),
		Rootfs:            "/init",
		RootPropagation:   syscall.MS_PRIVATE,
		Cgroups: &configs.Cgroup{
			Path: "pod/init",
			Resources: &configs.Resources{
				AllowAllDevices: false,
				AllowedDevices:  configs.DefaultAllowedDevices,
			},
		},
		MaskPaths: []string{
			"/proc/kcore",
		},
		ReadonlyPaths: []string{
			"/proc/asound",
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger",
		},
		Devices: configs.DefaultAutoCreatedDevices,
		Mounts:  defaultContainerMounts,
	}

	// Always require a mount and pid namespace.
	config.Namespaces = []configs.Namespace{
		{Type: configs.NEWNS},
		{Type: configs.NEWPID},
	}

	// Check for namespace isolator and apply it.
	nsiso := getNamespaceIsolator(cs.manifest.Pod)
	if nsiso == nil {
		cs.applyDefaultNamespaces(config)
	} else {
		cs.applyNamespacesIsolator(config, nsiso)
	}

	return config, nil
}

func (cs *containerSetup) getAppContainerConfig(runtimeApp schema.RuntimeApp) (*configs.Config, error) {
	name := runtimeApp.Name.String()

	initPid, err := cs.initProcess.Pid()
	if err != nil {
		return nil, fmt.Errorf("failed to get init container's pid: %v", err)
	}

	config := &configs.Config{
		Capabilities: []string{
			"CAP_CHOWN",
			"CAP_DAC_OVERRIDE",
			"CAP_FSETID",
			"CAP_FOWNER",
			"CAP_MKNOD",
			"CAP_NET_RAW",
			"CAP_SETGID",
			"CAP_SETUID",
			"CAP_SETFCAP",
			"CAP_SETPCAP",
			"CAP_NET_BIND_SERVICE",
			"CAP_SYS_CHROOT",
			"CAP_KILL",
			"CAP_AUDIT_WRITE",
		},
		ParentDeathSignal: int(syscall.SIGTERM),
		Rootfs:            filepath.Join("/apps", name),
		RootPropagation:   syscall.MS_PRIVATE,
		Namespaces: []configs.Namespace{
			{Type: configs.NEWNS},
			{Type: configs.NEWIPC, Path: fmt.Sprintf("/proc/%d/ns/ipc", initPid)},
			{Type: configs.NEWNET, Path: fmt.Sprintf("/proc/%d/ns/net", initPid)},
			{Type: configs.NEWPID, Path: fmt.Sprintf("/proc/%d/ns/pid", initPid)},
			{Type: configs.NEWUTS, Path: fmt.Sprintf("/proc/%d/ns/uts", initPid)},
		},
		Cgroups: &configs.Cgroup{
			Path: filepath.Join("pod", name),
			Resources: &configs.Resources{
				AllowAllDevices: false,
				AllowedDevices:  configs.DefaultAllowedDevices,
			},
		},
		MaskPaths: []string{
			"/proc/kcore",
		},
		ReadonlyPaths: []string{
			"/proc/asound",
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger",
		},
		Devices: configs.DefaultAutoCreatedDevices,
		Mounts:  defaultContainerMounts,
	}

	app := cs.getPodApp(runtimeApp)

	// apply isolators to the pod
	if err := cs.applyIsolators(app, config); err != nil {
		return nil, err
	}

	return config, nil
}

func (cs *containerSetup) getPodApp(runtimeApp schema.RuntimeApp) *types.App {
	if runtimeApp.App != nil {
		return runtimeApp.App
	}
	return cs.manifest.Images[runtimeApp.Image.ID.String()].App
}

func (cs *containerSetup) isShuttingDown() bool {
	cs.stateMutex.Lock()
	defer cs.stateMutex.Unlock()
	return cs.state.State == common.StagerStateTeardown
}

// getNamespaceIsolator checks the pod manifest to see is a linux namespace
// isolator is specified. If not, it will simply return nil.
func getNamespaceIsolator(pod *schema.PodManifest) *kschema.LinuxNamespaces {
	for _, iso := range pod.Isolators {
		if iso.Name.String() == kschema.LinuxNamespacesName {
			if niso, ok := iso.Value().(*kschema.LinuxNamespaces); ok {
				return niso
			}
		}
	}
	return nil
}

// needNewNamespace returns whether or not a new namespace is needed on the
// launcher object based on the LinuxNamespaceValue.
func needNewNamespace(val kschema.LinuxNamespaceValue) bool {
	if val == kschema.LinuxNamespaceHost {
		return false
	}
	return true
}

// applyNamespacesIsolator will apply the settings from the namespace isolator
// to the pod, and optionally configure the IPC, UTS, and Network settings.
func (cs *containerSetup) applyNamespacesIsolator(config *configs.Config, nsiso *kschema.LinuxNamespaces) {
	if needNewNamespace(nsiso.IPC()) {
		config.Namespaces.Add(configs.NEWIPC, "")
	}
	if needNewNamespace(nsiso.UTS()) {
		config.Namespaces.Add(configs.NEWUTS, "")
		config.Hostname = cs.manifest.Name
	}
}

// applyDefaultNamespaces adds the default namespace configuration settings for
// a pod.
func (cs *containerSetup) applyDefaultNamespaces(config *configs.Config) {
	config.Namespaces.Add(configs.NEWIPC, "")
	config.Namespaces.Add(configs.NEWUTS, "")
	config.Hostname = cs.manifest.Name
}

// applyIO is used to check if any inputs/outputs were provided with the stager
// for the given app.
func (cs *containerSetup) applyIO(appname string, process *libcontainer.Process) {
	if f := checkSpecificIO(appname, "STDIN"); f != nil {
		process.Stdin = f
	}
	if f := checkSpecificIO(appname, "STDOUT"); f != nil {
		process.Stdout = f
	}
	if f := checkSpecificIO(appname, "STDERR"); f != nil {
		process.Stderr = f
	}
}

const io_env_format = "STAGER_CONTAINER_%s_%s"

func checkSpecificIO(appname, which string) *os.File {
	fdstr := os.Getenv(fmt.Sprintf(io_env_format, appname, which))
	if fdstr == "" {
		return nil
	}

	fd, err := strconv.Atoi(fdstr)
	// just return nil, a standard one will be used
	if err != nil {
		return nil
	}
	if fd < 0 {
		return nil
	}
	return os.NewFile(uintptr(fd), fmt.Sprintf("%s_%s", appname, which))
}
