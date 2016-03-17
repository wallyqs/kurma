// Copyright 2015-2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer/configs"

	kschema "github.com/apcera/kurma/schema"
)

// setupLinuxNamespaceIsolator handles checking if the namespace isolator is
// used and primarily checks if the stager should get the host's network
// namespace.
func (pod *Pod) setupLinuxNamespaceIsolator() error {
	var nsiso *kschema.LinuxNamespaces

	for _, iso := range pod.manifest.Pod.Isolators {
		if iso.Name.String() == kschema.LinuxNamespacesName {
			if niso, ok := iso.Value().(*kschema.LinuxNamespaces); ok {
				nsiso = niso
				break
			}
		}
	}

	// Return ifit wasn't referenced at all.
	if nsiso == nil {
		return nil
	}

	// If it wants the host's network namespace, then set the flag to skip setting
	// up networking.
	if nsiso.Net() == kschema.LinuxNamespaceHost {
		pod.skipNetworking = true
		return nil
	}

	return nil
}

func (pod *Pod) setupHostPrivilegeIsolator(runtimeApp *schema.RuntimeApp) {
	app := runtimeApp.App
	if app == nil {
		app = pod.manifest.Images[runtimeApp.Image.ID.String()].App
	}

	var wantsPrivilege bool

	// Look for the isolator
	if iso := app.Isolators.GetByName(kschema.HostPrivilegedName); iso != nil {
		if piso, ok := iso.Value().(*kschema.HostPrivileged); ok {
			wantsPrivilege = bool(*piso)
		}
	}

	// Return if they don't want it.
	if !wantsPrivilege {
		return
	}

	// Apply the necessary mount points to the app
	runtimeApp.Mounts = append(runtimeApp.Mounts,
		schema.Mount{
			Volume: types.ACName(fmt.Sprintf("%s-host-pods", runtimeApp.Name.String())),
			Path:   "/host/pods",
		},
		schema.Mount{
			Volume: types.ACName(fmt.Sprintf("%s-host-proc", runtimeApp.Name.String())),
			Path:   "/host/proc",
		},
	)

	// Add the volumes
	trueVal := true
	pod.options.RawVolumes = append(pod.options.RawVolumes,
		types.Volume{
			Name:     types.ACName(fmt.Sprintf("%s-host-pods", runtimeApp.Name.String())),
			Kind:     "host",
			Source:   pod.manager.Options.PodDirectory,
			ReadOnly: &trueVal,
		},
		types.Volume{
			Name:   types.ACName(fmt.Sprintf("%s-host-proc", runtimeApp.Name.String())),
			Kind:   "host",
			Source: "/proc",
		},
	)
	pod.options.StagerMounts = append(pod.options.StagerMounts,
		&configs.Mount{
			Source:      pod.manager.Options.PodDirectory,
			Destination: filepath.Join("/volumes", fmt.Sprintf("%s-host-pods", runtimeApp.Name.String())),
			Device:      "bind",
			Flags:       syscall.MS_BIND | syscall.MS_RDONLY,
		},
		&configs.Mount{
			Source:      "/proc",
			Destination: filepath.Join("/volumes", fmt.Sprintf("%s-host-proc", runtimeApp.Name.String())),
			Device:      "bind",
			Flags:       syscall.MS_BIND,
		},
	)

	// Add volumes, if a volume directory is configured
	if pod.manager.Options.VolumeDirectory != "" {
		name := types.ACName(fmt.Sprintf("%s-host-volumes", runtimeApp.Name.String()))
		runtimeApp.Mounts = append(runtimeApp.Mounts, schema.Mount{
			Volume: name,
			Path:   "/host/volumes",
		})
		pod.options.RawVolumes = append(pod.options.RawVolumes, types.Volume{
			Name:   name,
			Kind:   "host",
			Source: pod.manager.Options.VolumeDirectory,
		})
		pod.options.StagerMounts = append(pod.options.StagerMounts, &configs.Mount{
			Source:      pod.manager.Options.VolumeDirectory,
			Destination: filepath.Join("/volumes", name.String()),
			Device:      "bind",
			Flags:       syscall.MS_BIND,
		})
	}
}

// setupHostApiAccessIsolator configures the container for API access over
// Kurma's unix socket by bind mounting it into the container.
func (pod *Pod) setupHostApiAccessIsolator(runtimeApp *schema.RuntimeApp) error {
	app := runtimeApp.App
	if app == nil {
		app = pod.manifest.Images[runtimeApp.Image.ID.String()].App
	}

	// Check if the pod should have host API access via the socket file
	wantsAccess := false
	if iso := app.Isolators.GetByName(kschema.HostApiAccessName); iso != nil {
		if piso, ok := iso.Value().(*kschema.HostApiAccess); ok {
			wantsAccess = bool(*piso)
		}
	}

	// If access isn't wanted, just return
	if !wantsAccess {
		return nil
	}

	// Create a blank directory with just the socket so we're sure just that is
	// mapped in.
	dest := filepath.Join(pod.stagerRootPath(), "volumes", fmt.Sprintf("%s-kurma-socket", runtimeApp.Name))
	err := mkdirs([]string{filepath.Join(pod.stagerRootPath(), "volumes"), dest}, os.FileMode(0755), false)
	if err != nil {
		return fmt.Errorf("failed to create kurma-socket volume: %v", err)
	}

	// Update the runtime app and pod
	runtimeApp.Mounts = append(runtimeApp.Mounts, schema.Mount{
		Volume: types.ACName(fmt.Sprintf("%s-kurma-socket", runtimeApp.Name.String())),
		Path:   "/var/lib/kurma",
	})
	pod.options.StagerMounts = append(pod.options.StagerMounts, &configs.Mount{
		Source:      pod.manager.HostSocketFile,
		Destination: filepath.Join("/volumes", fmt.Sprintf("%s-kurma-socket", runtimeApp.Name), "kurma.sock"),
		Device:      "bind",
		Flags:       syscall.MS_BIND,
	})
	pod.options.RawVolumes = append(pod.options.RawVolumes, types.Volume{
		Name:   types.ACName(fmt.Sprintf("%s-kurma-socket", runtimeApp.Name.String())),
		Kind:   "host",
		Source: "/var/lib/kurma",
	})

	return nil
}
