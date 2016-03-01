// Copyright 2015 Apcera Inc. All rights reserved.

package pod

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	kschema "github.com/apcera/kurma/schema"
	"github.com/opencontainers/runc/libcontainer/configs"
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

// setupHostApiAccessIsolator configures the container for API access over
// Kurma's unix socket by bind mounting it into the container.
func (pod *Pod) setupHostApiAccessIsolator(config *configs.Config) error {
	// Check if the pod should have host API access via the socket file
	wantsAccess := false
	for _, iso := range pod.manifest.Pod.Isolators {
		if iso.Name.String() == kschema.HostApiAccessName {
			wantsAccess = true
			break
		}
	}
	if !wantsAccess {
		// Check each app
		for _, image := range pod.manifest.Images {
			if image.App == nil {
				continue
			}
			if iso := image.App.Isolators.GetByName(kschema.HostApiAccessName); iso != nil {
				if piso, ok := iso.Value().(*kschema.HostApiAccess); ok {
					if *piso {
						wantsAccess = true
						break
					}
				}
			}
		}
	}

	// If access isn't wanted, just return
	if !wantsAccess {
		return nil
	}

	// get the path to a mock volume location for the socket
	dest := filepath.Join(pod.stagerRootPath(), "volumes", "kurma-socket")
	err := mkdirs([]string{dest}, os.FileMode(0755), false)
	if err != nil {
		return fmt.Errorf("failed to create kurma-socket volume: %v", err)
	}

	m := &configs.Mount{
		Source:      pod.manager.HostSocketFile,
		Destination: filepath.Join(dest, "kurma.sock"),
		Device:      "bind",
		Flags:       syscall.MS_BIND,
	}
	config.Mounts = append(config.Mounts, m)

	return nil
}
