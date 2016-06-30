// Copyright 2015-2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/apcera/kurma/pkg/capabilities"
	"github.com/apcera/util/proc"
	"github.com/apcera/util/tarhelper"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"

	cnitypes "github.com/containernetworking/cni/pkg/types"
)

const (
	defaultMountFlags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
)

func (pod *Pod) stagerLogPath() string {
	return filepath.Join(pod.directory, "stager.log")
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
				Destination:      "/sys/fs/cgroup",
				Device:           "cgroup",
				PropagationFlags: []int{syscall.MS_PRIVATE},
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

	// apply any raw mounts that aren't relevant to the pod
	if len(pod.options.StagerMounts) > 0 {
		config.Mounts = append(config.Mounts, pod.options.StagerMounts...)
	}
	if len(pod.options.RawVolumes) > 0 {
		pod.manifest.Pod.Volumes = append(pod.manifest.Pod.Volumes, pod.options.RawVolumes...)
	}

	// check if networking needs to be configured on the stager
	if !pod.skipNetworking {
		config.Namespaces.Add(configs.NEWNET, pod.netNsPath)
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

// waitRoutine is used to track when the stager exits and to respond by tearing
// down the pod.
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
		ps, _ := proc.Wait()
		pod.log.Warnf("Stager process has exited: %v", ps.String())

		// If we're in the process of shutting down, just return
		if pod.isShuttingDown() {
			return
		}
		go pod.Stop()
	}()
}

const io_env_format = "STAGER_CONTAINER_%s_%s=%d"

func (pod *Pod) applyIOs() {
	for name, io := range pod.options.ContainerIO {
		applyIndividualIO(pod.stagerProcess, io.Stdin, name, "STDIN")
		applyIndividualIO(pod.stagerProcess, io.Stdin, name, "STDOUT")
		applyIndividualIO(pod.stagerProcess, io.Stdin, name, "STDERR")
	}
}

func applyIndividualIO(process *libcontainer.Process, f *os.File, name, which string) {
	if f == nil {
		return
	}

	process.ExtraFiles = append(process.ExtraFiles, f)
	// offset for stdin/out/err, only 2 since it was already appended
	fd := len(process.ExtraFiles) + 2
	process.Env = append(process.Env, fmt.Sprintf(io_env_format, name, which, fd))
}

// isBlankDNS is used as a workaround for a CNI issue where some of the CNI
// plugins will return "{}" as the DNS section on the results. This will cause
// Go to instantiate a DNS option with all the default values. This iterates all
// the fields and if they're all blank/empty, we'll return that it is blank so
// the entry is skipped.
func isBlankDNS(dns *cnitypes.DNS) bool {
	if dns == nil {
		return true
	}

	if dns.Domain == "" && len(dns.Nameservers) == 0 && len(dns.Options) == 0 && len(dns.Search) == 0 {
		return true
	}

	return false
}
