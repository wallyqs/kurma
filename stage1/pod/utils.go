// Copyright 2015-2016 Apcera Inc. All rights reserved.

package pod

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/apcera/kurma/util/capabilities"
	"github.com/apcera/util/proc"
	"github.com/apcera/util/tarhelper"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	defaultMountFlags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
)

func (pod *Pod) stagerLogPath() string {
	return filepath.Join(pod.directory, "stager.log")
}

func (pod *Pod) stagerNetPath() string {
	return filepath.Join(pod.directory, "netns")
}

func (pod *Pod) stagerRootPath() string {
	return filepath.Join(pod.directory, "stager")
}

func (pod *Pod) generateContainerConfig() (*configs.Config, error) {
	root := pod.stagerRootPath()

	config := &configs.Config{
		// Settings that vary per container
		Rootfs: root,
		Cgroups: &configs.Cgroup{
			Path: filepath.Join(pod.manager.Options.ParentCgroupName, pod.ShortName()),
			Resources: &configs.Resources{
				AllowAllDevices: true,
			},
		},

		// Static configuration
		Capabilities:    capabilities.GetAllCapabilities(),
		RootPropagation: syscall.MS_PRIVATE,
		Namespaces: []configs.Namespace{
			configs.Namespace{Type: configs.NEWNS},
		},
		Mounts: []*configs.Mount{
			{
				Source:      "/dev",
				Destination: "/dev",
				Device:      "bind",
				Flags:       syscall.MS_BIND | syscall.MS_REC,
			},
			{
				Source:      "proc",
				Destination: "/proc",
				Device:      "proc",
				Flags:       defaultMountFlags,
			},
			{
				Source:      "sysfs",
				Destination: "/sys",
				Device:      "sysfs",
				Flags:       defaultMountFlags | syscall.MS_RDONLY,
			},
			{
				Destination: "/sys/fs/cgroup",
				Device:      "cgroup",
			},
		},
	}

	// Add the layer mounts
	for layer, layerPath := range pod.layerPaths {
		dst := filepath.Join("/layers", layer)

		config.Mounts = append(config.Mounts, &configs.Mount{
			Source:      layerPath,
			Destination: dst,
			Device:      "bind",
			Flags:       syscall.MS_BIND | syscall.MS_RDONLY,
		})
	}

	// Add in the volume mounts
	for _, volume := range pod.manifest.Pod.Volumes {
		hostPath, err := pod.manager.getVolumePath(volume.Name.String())
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve volume for %q: %v", volume.Name, err)
		}
		dst := filepath.Join("/volumes", volume.Name.String())

		m := &configs.Mount{
			Source:      hostPath,
			Destination: dst,
			Device:      "bind",
			Flags:       syscall.MS_BIND,
		}

		if volume.ReadOnly != nil && *volume.ReadOnly {
			m.Flags |= syscall.MS_RDONLY
		}

		config.Mounts = append(config.Mounts, m)
	}

	//
	// ISOLATORS
	//

	if err := pod.setupHostApiAccessIsolator(config); err != nil {
		return nil, fmt.Errorf("failed to configured API access isolator: %v", err)
	}

	// check if networking needs to be conofigured on the stager
	if !pod.skipNetworking {
		config.Namespaces.Add(configs.NEWNET, pod.stagerNetPath())
		config.Networks = []*configs.Network{
			{
				Type:    "loopback",
				Address: "127.0.0.1/0",
				Gateway: "localhost",
			},
		}
	}

	return config, nil
}

func copypath(src, dst string) error {
	// Stream the root over to the new location with tarhelper. This is simpler
	// than walking the directories, copying files, checking for symlinks, etc.
	pr, pw := io.Pipe()
	tar := tarhelper.NewTar(pw, src)
	tar.IncludeOwners = true
	tar.IncludePermissions = true
	// ExcludePaths is stupid, leave off the leading slash.
	tar.ExcludePath(dst[1:] + ".*")
	tar.Compression = tarhelper.NONE
	wg := sync.WaitGroup{}
	wg.Add(1)
	var archiveErr error
	go func() {
		defer wg.Done()
		archiveErr = tar.Archive()
	}()
	untar := tarhelper.NewUntar(pr, dst)
	untar.AbsoluteRoot = dst
	untar.PreserveOwners = true
	untar.PreservePermissions = true
	if err := untar.Extract(); err != nil {
		return err
	}

	// ensure we check that the archive call did not error out
	wg.Wait()
	if archiveErr != nil {
		return archiveErr
	}
	return nil
}

func mkdirs(dirs []string, mode os.FileMode, existOk bool) error {
	for i := range dirs {
		// Make sure that this directory doesn't currently exist if existOk
		// is false.
		if stat, err := os.Lstat(dirs[i]); err == nil {
			if !existOk {
				return fmt.Errorf("lstat: path already exists: %s", dirs[i])
			} else if !stat.IsDir() {
				return fmt.Errorf("lstat: %s is not a directory.", dirs[i])
			}
		} else if !os.IsNotExist(err) {
			return err
		} else if err := os.Mkdir(dirs[i], mode); err != nil {
			return fmt.Errorf("mkdir: %s", err)
		}

		// Ensure that the mode is applied by running chmod against it. We
		// need to do this because Mkdir will apply umask which might screw
		// with the permissions.
		if err := os.Chmod(dirs[i], mode); err != nil {
			return fmt.Errorf("chmod: %s", err)
		}
	}
	return nil
}

func unmountDirectories(path string) error {
	// Get the list of mount points that are under this pod's directory
	// and then attempt to unmount them in reverse order. This is required
	// so that all mounts are unmounted before a parent is unmounted.
	mountPoints := make([]string, 0, 100)
	root := path + string(os.PathSeparator)
	err := proc.ParseSimpleProcFile(
		proc.MountProcFile,
		nil,
		func(line int, index int, elem string) error {
			switch {
			case index != 1:
			case elem == path:
				mountPoints = append(mountPoints, elem)
			case strings.HasPrefix(elem, root):
				mountPoints = append(mountPoints, elem)
			}
			return nil
		})
	if err != nil {
		return err
	}

	// Now walk the list in reverse order unmounting each point one at a time.
	for i := len(mountPoints) - 1; i >= 0; i-- {
		if err := syscall.Unmount(mountPoints[i], syscall.MNT_FORCE); err != nil {
			return fmt.Errorf("failed to unmount %q: %v", mountPoints[i], err)
		}
	}

	return nil
}

func convertACIdentifierToACName(name types.ACIdentifier) (*types.ACName, error) {
	parts := strings.Split(name.String(), "/")
	n, err := types.SanitizeACName(parts[len(parts)-1])
	if err != nil {
		return nil, err
	}
	return types.NewACName(n)
}

func (pod *Pod) waitRoutine() {
	proc := pod.stagerProcess
	if proc == nil {
		return
	}

	ch := make(chan struct{})
	pod.stagerWaitCh = ch

	go func() {
		defer close(ch)

		// Wait for the stager process to exit
		ps, err := proc.Wait()

		// If we're in the process of shutting down, just return
		if pod.isShuttingDown() {
			return
		}

		pod.log.Errorf("Stager process has exited: %v - %v", ps.String(), err)
		go pod.Stop()
	}()
}
