// Copyright 2015 Apcera Inc. All rights reserved.

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	kschema "github.com/apcera/kurma/schema"
	"github.com/apcera/kurma/stage3/client"
)

// setupLinuxNamespaceIsolator handles configuring the container for the
// namespaces it is intended to have.
func (c *Container) setupLinuxNamespaceIsolator(launcher *client.Launcher) error {
	// Configure which linux namespaces to create
	nsisolators := false
	if iso := c.image.App.Isolators.GetByName(kschema.LinuxNamespacesName); iso != nil {
		if niso, ok := iso.Value().(*kschema.LinuxNamespaces); ok {
			launcher.NewIPCNamespace = niso.IPC()
			launcher.NewNetworkNamespace = niso.Net()
			launcher.NewPIDNamespace = niso.PID()
			launcher.NewUserNamespace = niso.User()
			launcher.NewUTSNamespace = niso.UTS()
			nsisolators = true
		}
	}
	if !nsisolators {
		// set some defaults if no namespace isolator was given
		launcher.NewIPCNamespace = true
		launcher.NewPIDNamespace = true
		launcher.NewUTSNamespace = true
	}
	return nil
}

// setupHostPrivilegeIsolator instruments the host access, if the container has
// the host access isolator.
func (c *Container) setupHostPrivilegeIsolator(launcher *client.Launcher) error {
	// Check for a privileged isolator
	if iso := c.image.App.Isolators.GetByName(kschema.HostPrivilegedName); iso != nil {
		if piso, ok := iso.Value().(*kschema.HostPrivileged); ok {
			if *piso {
				launcher.HostPrivileged = true

				// create the mount point
				podsDest, err := c.ensureContainerPathExists("host/pods")
				if err != nil {
					return err
				}
				procDest, err := c.ensureContainerPathExists("host/proc")
				if err != nil {
					return err
				}

				podsMount := strings.Replace(podsDest, c.storage.HostRoot(), client.DefaultChrootPath, 1)
				procMount := strings.Replace(procDest, c.storage.HostRoot(), client.DefaultChrootPath, 1)

				// create the mount point definitions for host access
				launcher.MountPoints = []*client.MountPoint{
					// Add the pods mount
					&client.MountPoint{
						Source:      c.manager.Options.ContainerDirectory,
						Destination: podsMount,
						Flags:       syscall.MS_BIND,
					},
					// Make the pods mount read only. This cannot be done all in one, and
					// needs MS_BIND included to avoid "resource busy" and to ensure we're
					// only making the bind location read-only, not the parent.
					&client.MountPoint{
						Source:      podsMount,
						Destination: podsMount,
						Flags:       syscall.MS_BIND | syscall.MS_REMOUNT | syscall.MS_RDONLY,
					},

					// Add the host's proc filesystem under host/proc. This can be done
					// for diagnostics of the host's state, and can also be used to get
					// access to the host's filesystem (via /host/proc/1/root/...). This
					// is not read-only because making it read only isn't effective. You
					// can still traverse into .../root/... partitions due to the magic
					// that is proc and namespaces. Using proc is more useful than root
					// because it ensures more consistent access to process's actual
					// filesystem state as it crosses namespaces. Direct bind mounts tend
					// to miss some child mounts, even when trying to ensure everything is
					// shared.
					&client.MountPoint{
						Source:      "/proc",
						Destination: procMount,
						Flags:       syscall.MS_BIND,
					},
				}

				// If a volume directory is defined, then map it in as well.
				if c.manager.Options.VolumeDirectory != "" {
					volumesDest, err := c.ensureContainerPathExists("host/volumes")
					if err != nil {
						return err
					}
					volumesMount := strings.Replace(volumesDest, c.storage.HostRoot(), client.DefaultChrootPath, 1)
					launcher.MountPoints = append(launcher.MountPoints,
						&client.MountPoint{
							Source:      c.manager.Options.VolumeDirectory,
							Destination: volumesMount,
							Flags:       syscall.MS_BIND,
						})
				}
			}
		}
	}

	return nil
}

// setupHostApiAccessIsolator configures the container for API access over
// Kurma's unix socket by bind mounting it into the container.
func (c *Container) setupHostApiAccessIsolator(launcher *client.Launcher) error {
	// Check if the container should have host API access via the socket file
	if iso := c.image.App.Isolators.GetByName(kschema.HostApiAccessName); iso != nil {
		if piso, ok := iso.Value().(*kschema.HostApiAccess); ok {
			if *piso {
				// find the relative path to the container, ensure it exists
				dest, err := c.ensureContainerPathExists("var/lib")
				if err != nil {
					return err
				}

				// create the destination file that we'll bind mount
				f, err := os.Create(filepath.Join(dest, "kurma.sock"))
				if err != nil {
					return err
				}
				f.Close()

				// stat the socket file, get its group, and add it to supplemental GIDs
				// for the container.
				fi, err := os.Stat(c.manager.HostSocketFile)
				if err != nil {
					return fmt.Errorf("failed to stat the server socket file: %v", err)
				}
				c.image.App.SupplementaryGIDs = append(c.image.App.SupplementaryGIDs,
					int(fi.Sys().(*syscall.Stat_t).Gid))

				// find the container relative path, pre-chroot, and setup the mount
				m := strings.Replace(dest, c.storage.HostRoot(), client.DefaultChrootPath, 1)
				launcher.Debug = true
				launcher.MountPoints = append(launcher.MountPoints,
					&client.MountPoint{
						Source:      c.manager.HostSocketFile,
						Destination: filepath.Join(m, "kurma.sock"),
						Flags:       syscall.MS_BIND,
					})
			}
		}
	}

	return nil
}
