// Copyright 2016 Apcera Inc. All rights reserved.

package container

import (
	"fmt"
	"path/filepath"
	"syscall"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer/configs"
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
	}

	initContainerConfig = &configs.Config{
		ParentDeathSignal: int(syscall.SIGKILL),
		Rootfs:            "/init",
		RootPropagation:   syscall.MS_PRIVATE,
		Namespaces: []configs.Namespace{
			configs.Namespace{Type: configs.NEWIPC},
			configs.Namespace{Type: configs.NEWNET},
			configs.Namespace{Type: configs.NEWNS},
			configs.Namespace{Type: configs.NEWPID},
			configs.Namespace{Type: configs.NEWUTS},
		},
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
			"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
		},
		Devices: configs.DefaultAutoCreatedDevices,
		Mounts:  defaultContainerMounts,
		Networks: []*configs.Network{
			{
				Type:    "loopback",
				Address: "127.0.0.1/0",
				Gateway: "localhost",
			},
		},
	}
)

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
		ParentDeathSignal: int(syscall.SIGKILL),
		Rootfs:            filepath.Join("/apps", name),
		RootPropagation:   syscall.MS_PRIVATE,
		Namespaces: []configs.Namespace{
			configs.Namespace{Type: configs.NEWNS},
			configs.Namespace{Type: configs.NEWIPC, Path: fmt.Sprintf("/proc/%d/ns/ipc", initPid)},
			configs.Namespace{Type: configs.NEWNET, Path: fmt.Sprintf("/proc/%d/ns/net", initPid)},
			configs.Namespace{Type: configs.NEWPID, Path: fmt.Sprintf("/proc/%d/ns/pid", initPid)},
			configs.Namespace{Type: configs.NEWUTS, Path: fmt.Sprintf("/proc/%d/ns/uts", initPid)},
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
			"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
		},
		Devices: configs.DefaultAutoCreatedDevices,
		Mounts:  defaultContainerMounts,
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
	return cs.state.State == stagerStateTeardown
}
