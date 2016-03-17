// Copyright 2016 Apcera Inc. All rights reserved.

package container

import (
	"fmt"
	"syscall"

	"github.com/apcera/kurma/pkg/capabilities"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"

	kschema "github.com/apcera/kurma/schema"
)

var (
	isolatorFuncs = map[string]func(*containerSetup, types.IsolatorValue, *types.App, *configs.Config) error{
		kschema.LinuxPrivilegedName: (*containerSetup).applyPrivilegedIsolator,
	}
)

func (cs *containerSetup) applyIsolators(app *types.App, container *configs.Config) error {
	for _, iso := range app.Isolators {
		if f, exists := isolatorFuncs[iso.Name.String()]; exists {
			if err := f(cs, iso.Value(), app, container); err != nil {
				return fmt.Errorf("failed to apply %q isolator: %v", iso.Name.String(), err)
			}
		}
	}
	return nil
}

func (cs *containerSetup) applyPrivilegedIsolator(iso types.IsolatorValue, app *types.App, container *configs.Config) error {
	niso, ok := iso.(*kschema.LinuxPrivileged)
	if !ok {
		return nil
	}
	if !bool(*niso) {
		return nil
	}

	container.Capabilities = capabilities.GetAllCapabilities()

	container.ReadonlyPaths = nil
	container.MaskPaths = nil

	for i := range container.Mounts {
		// clear readonly for /sys and cgroups
		switch container.Mounts[i].Device {
		case "sysfs", "cgroup":
			container.Mounts[i].Flags &= ^syscall.MS_RDONLY
		}
	}

	container.Cgroups.Resources.AllowAllDevices = true

	devices, err := devices.HostDevices()
	if err != nil {
		return fmt.Errorf("failed to list host devices: %v", err)
	}
	container.Devices = devices
	return nil
}
