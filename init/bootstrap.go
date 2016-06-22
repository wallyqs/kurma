// Copyright 2015-2016 Apcera Inc. All rights reserved.

package init

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/capabilities"
	"github.com/apcera/kurma/pkg/daemon"
	"github.com/apcera/kurma/pkg/devices"
	"github.com/apcera/kurma/pkg/fetch"
	"github.com/apcera/kurma/pkg/imagestore"
	"github.com/apcera/kurma/pkg/local/aci"
	"github.com/apcera/kurma/pkg/networkmanager"
	"github.com/apcera/kurma/pkg/podmanager"
	"github.com/apcera/logray"
	"github.com/apcera/util/proc"
	"github.com/apcera/util/tarhelper"
	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/vishvananda/netlink"

	kschema "github.com/apcera/kurma/schema"
)

// switchRoot handles copying all of the files off the initial initramfs root
// and onto a tmpfs mount, then moving the root to it.
func (r *runner) switchRoot() error {
	// create the newroot on a tmpfs
	if err := os.Mkdir(newRoot, os.FileMode(0755)); err != nil {
		return err
	}
	if err := syscall.Mount("none", newRoot, "tmpfs", 0, ""); err != nil {
		return err
	}

	// Stream the root over to the new location with tarhelper. This is simpler
	// than walking the directories, copying files, checking for symlinks, etc.
	pr, pw := io.Pipe()
	tar := tarhelper.NewTar(pw, "/")
	tar.IncludeOwners = true
	tar.IncludePermissions = true
	// ExcludePaths is stupid, leave off the leading slash.
	tar.ExcludePath(newRoot[1:] + ".*")
	tar.Compression = tarhelper.NONE
	wg := sync.WaitGroup{}
	wg.Add(1)
	var archiveErr error
	go func() {
		defer wg.Done()
		archiveErr = tar.Archive()
	}()
	untar := tarhelper.NewUntar(pr, newRoot)
	untar.AbsoluteRoot = newRoot
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

	// move the root to newroot
	if err := syscall.Chdir(newRoot); err != nil {
		return err
	}
	if err := syscall.Mount(newRoot, "/", "", syscall.MS_MOVE, ""); err != nil {
		return err
	}
	if err := syscall.Chroot("."); err != nil {
		return err
	}
	if err := syscall.Chdir("/"); err != nil {
		return err
	}
	return nil
}

// loadConfigurationFile loads the configuration for the process. It will take
// the default coded configuration, merge it with the base configuration file
// within the initrd filesystem, and then check for the OEM configuration to
// merge in as well.
func (r *runner) loadConfigurationFile() error {
	// first, load the config from the local filesystem in the initrd
	diskConfig, err := getConfigurationFromFile(configurationFile)
	if err != nil {
		return err
	}
	if diskConfig != nil {
		r.config.mergeConfig(diskConfig)
	}

	// second, load from the boot cmdline
	if bootConfig := getConfigFromCmdline(); bootConfig != nil {
		r.config.mergeConfig(bootConfig)
	}

	// FIXME likely need to move OEMConfig to after loading modules and udev

	// if an OEM config is specified, attempt to find it
	if r.config.OEMConfig != nil {
		device := devices.ResolveDevice(r.config.OEMConfig.Device)
		if device == "" {
			r.log.Warnf("Unable to resolve oem config device %q, skipping", r.config.OEMConfig.Device)
			return nil
		}
		fstype, _ := devices.GetFsType(device)

		// FIXME check fstype against currently supported types

		// mount the disk
		diskPath := filepath.Join(mountPath, strings.Replace(device, "/", "_", -1))
		if err := handleMount(device, diskPath, fstype, 0, ""); err != nil {
			r.log.Errorf("failed to mount oem config disk %q: %v", device, err)
			return nil
		}

		// attempt to load the configuration
		configPath := filepath.Join(diskPath, r.config.OEMConfig.ConfigPath)
		diskConfig, err := getConfigurationFromFile(configPath)
		if err != nil {
			r.log.Errorf("Failed to load oem config: %v", err)
			return nil
		}
		if diskConfig != nil {
			r.log.Infof("Loading OEM config: %q", configPath)
			r.config.mergeConfig(diskConfig)
		}
	}

	return nil
}

// configureLogging is used to enable tracing logging, if it is turned on in the
// configuration.
func (r *runner) configureLogging() error {
	if r.config.Debug {
		logray.ResetDefaultLogLevel(logray.ALL)
	}
	return nil
}

// createSystemMounts configured the default mounts for the host. Since kurma is
// running as PID 1, there is no /etc/fstab, therefore it must mount them
// itself.
func (r *runner) createSystemMounts() error {
	// Default mounts to handle on boot. Note that order matters, they should be
	// alphabetical by mount location. Elements are: mount location, source,
	// fstype.
	systemMounts := [][]string{
		[]string{"/dev", "devtmpfs", "devtmpfs"},
		[]string{"/dev/pts", "none", "devpts"},
		[]string{"/proc", "none", "proc"},
		[]string{"/sys", "none", "sysfs"},
		[]string{"/tmp", "none", "tmpfs"},
		[]string{kurmaPath, "none", "tmpfs"},
		[]string{mountPath, "none", "tmpfs"},

		// put cgroups in a tmpfs so we can create the subdirectories
		[]string{cgroupsMount, "none", "tmpfs"},
	}

	r.log.Info("Creating system mounts")

	// Check if the /proc/mounts file exists to see if there are mounts that
	// already exist. This is primarily to support testing bootstrapping with
	// kurma launched by kurma (yes, meta)
	var existingMounts map[string]*proc.MountPoint
	if _, err := os.Lstat(proc.MountProcFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if %q existed: %v", proc.MountProcFile, err)
	} else if os.IsNotExist(err) {
		// really are freshly booted, /proc isn't mounted, so make this blank
		existingMounts = make(map[string]*proc.MountPoint)
	} else {
		// Get existing mount points.
		existingMounts, err = proc.MountPoints()
		if err != nil {
			return fmt.Errorf("failed to read existing mount points: %v", err)
		}
	}

	for _, mount := range systemMounts {
		location, source, fstype := mount[0], mount[1], mount[2]

		// check if it exists
		if _, exists := existingMounts[location]; exists {
			r.log.Tracef("- skipping %q, already mounted", location)
			continue
		}

		// perform the mount
		r.log.Tracef("- mounting %q (type %q) to %q", source, fstype, location)
		if err := handleMount(source, location, fstype, 0, ""); err != nil {
			return fmt.Errorf("failed to mount %q: %v", location, err)
		}
	}

	// Now that proc is mounted, refresh the capabilities list so it can read the
	// cap_last_cap from /proc.
	capabilities.RefreshCapabilities()

	return nil
}

// configureEnvironment sets environment variables that will be necessary for
// the process.
func (r *runner) configureEnvironment() error {
	os.Setenv("TMPDIR", "/tmp")
	os.Setenv("PATH", "/bin:/sbin")
	return nil
}

// mountCgroups handles creating the individual cgroup endpoints that are
// necessary.
func (r *runner) mountCgroups() error {
	r.log.Info("Setting up cgroups")

	// Check for available cgroup types and mount them
	cgroupTypes := make([]string, 0)
	err := proc.ParseSimpleProcFile("/proc/cgroups", nil,
		func(line, index int, elem string) error {
			if index != 0 {
				return nil
			}
			if strings.HasPrefix(elem, "#") {
				return nil
			}
			cgroupTypes = append(cgroupTypes, elem)
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to get list of available cgroups: %v", err)
	}

	// mount the cgroups
	for _, cgrouptype := range cgroupTypes {
		location := filepath.Join(cgroupsMount, cgrouptype)
		r.log.Tracef("- mounting cgroup %q to %q", cgrouptype, location)
		if err := handleMount("none", location, "cgroup", 0, cgrouptype); err != nil {
			return fmt.Errorf("failed to mount cgroup %q: %v", cgrouptype, err)
		}

		// if this is the memory mount, need to set memory.use_hierarchy = 1
		if cgrouptype == "memory" {
			err := func() error {
				hpath := filepath.Join(location, "memory.use_hierarchy")
				f, err := os.OpenFile(hpath, os.O_WRONLY|os.O_TRUNC, os.FileMode(0644))
				if err != nil {
					return fmt.Errorf("Failed to configure memory hierarchy: %v", err)
				}
				defer f.Close()
				if _, err := f.WriteString("1\n"); err != nil {
					return fmt.Errorf("Failed to configure memory heirarchy: %v", err)
				}
				return nil
			}()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// loadModules handles loading all of the kernel modules that are specified in
// the configuration.
func (r *runner) loadModules() error {
	if len(r.config.Modules) == 0 {
		return nil
	}

	r.log.Infof("Loading specified modules [%s]", strings.Join(r.config.Modules, ", "))
	for _, mod := range r.config.Modules {
		if b, err := exec.Command("modprobe", mod).CombinedOutput(); err != nil {
			r.log.Errorf("- Failed to load module %q: %s", mod, string(b))
		}
	}
	return nil
}

// mountDisks handles walking the disk configuration to configure the specified
// disks, mount them, and make them accessible at the right locations.
func (r *runner) mountDisks() error {
	// Walk the disks to validate that usage entries aren't in multiple
	// records. Do this before making any changes to the disks.
	usages := make(map[kurmaPathUsage]bool, 0)
	for _, disk := range r.config.Disks {
		for _, u := range disk.Usage {
			if usages[u] {
				return fmt.Errorf("multiple disk entries cannot specify the same usage [%s]", string(u))
			}
			usages[u] = true
		}
	}

	// do the stuff
	for _, disk := range r.config.Disks {
		device := devices.ResolveDevice(disk.Device)
		if device == "" {
			r.log.Warnf("Unable to resolve device %q, skipping", disk.Device)
			continue
		}
		fstype, _ := devices.GetFsType(device)

		// FIXME check fstype against currently supported types

		// format it, if needed
		if shouldFormatDisk(disk, fstype) {
			r.log.Infof("Formatting disk %s to %s", device, disk.FsType)
			if err := formatDisk(device, disk.FsType); err != nil {
				r.log.Errorf("failed to format disk %q: %v", device, err)
				continue
			}
			fstype = disk.FsType
		}

		// resize it, but only if ext4 for now
		if strings.HasPrefix(fstype, "ext") && disk.Resize {
			output, err := exec.Command("/bin/resizefs", device).CombinedOutput()
			if err != nil {
				r.log.Warnf("failed to resize disk %q: %v - %q", device, err, string(output))
			}
		}

		// mount it
		diskPath := filepath.Join(mountPath, strings.Replace(device, "/", "_", -1))
		if err := handleMount(device, diskPath, fstype, 0, disk.Options); err != nil {
			r.log.Errorf("failed to mount disk %q: %v", device, err)
			continue
		}

		// setup usages
		for _, usage := range disk.Usage {
			usagePath := filepath.Join(diskPath, string(usage))

			// ensure the directory exists
			if err := os.MkdirAll(usagePath, os.FileMode(0755)); err != nil {
				r.log.Errorf("failed to create mount point: %v", err)
				continue
			}

			// bind mount it to the kurma path
			kurmaUsagePath := filepath.Join(kurmaPath, string(usage))
			if err := bindMount(usagePath, kurmaUsagePath); err != nil {
				r.log.Errorf("failed to bind mount the selected volume: %v", err)
				continue
			}
		}
	}

	return nil
}

// loadAvailableImages will import all the available images into the Image
// Manager.
func (r *runner) loadAvailableImages() error {
	files, err := ioutil.ReadDir("/acis")
	if err != nil {
		return fmt.Errorf("Failed to read available ACI images: %v", err)
	}

	for _, fi := range files {
		if filepath.Ext(fi.Name()) != ".aci" {
			continue
		}

		err := func(name string) error {
			f, err := os.Open(name)
			if err != nil {
				return err
			}
			defer os.Remove(name)
			defer f.Close()

			_, _, err = r.imageManager.CreateImage(f)
			if err != nil {
				return err
			}
			return nil
		}(filepath.Join("/acis", fi.Name()))
		if err != nil {
			r.log.Warnf("Failed to import image %q: %v", fi.Name(), err)
		}
	}

	return nil
}

// cleanOldPods removes the directories for any pods remaining from a previous
// run. If the host is booting up, those pods are obviously dead and stale.
func (r *runner) cleanOldPods() error {
	podsPath := filepath.Join(kurmaPath, string(kurmaPathPods))
	fis, err := ioutil.ReadDir(podsPath)
	if err != nil {
		r.log.Errorf("failed to check for existing pods: %v", err)
		return nil
	}

	for _, fi := range fis {
		if err := os.RemoveAll(filepath.Join(podsPath, fi.Name())); err != nil {
			r.log.Errorf("failed to cleanup existing pods: %v", err)
		}
	}
	return nil
}

// configureHostname calls to set the hostname to the one provided via
// configuration.
func (r *runner) configureHostname() error {
	if r.config.Hostname == "" {
		return nil
	}

	r.log.Infof("Setting hostname: %s", r.config.Hostname)
	if err := syscall.Sethostname([]byte(r.config.Hostname)); err != nil {
		r.log.Errorf("- Failed to set hostname: %v", err)
	}
	return nil
}

// configureNetwork handles iterating the local interfaces, matching it to an
// interface configuration, and configuring it. It will also handle configuring
// the default gateway after all interfaces are configured.
func (r *runner) configureNetwork() error {
	r.log.Info("Configuring network...")

	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("failed to list network interfaces: %v", err)
	}

	for _, link := range links {
		linkName := link.Attrs().Name
		r.log.Debugf("Configuring %s...", linkName)

		// look for a matching network config entry
		var netconf *kurmaNetworkInterface
		for _, n := range r.config.NetworkConfig.Interfaces {
			if linkName == n.Device {
				netconf = n
				break
			}
			if match, _ := regexp.MatchString(n.Device, linkName); match {
				netconf = n
				break
			}
		}

		// handle if none are found
		if netconf == nil {
			r.log.Warn("- no matching network configuraton found")
			continue
		}

		// configure it
		if err := configureInterface(link, netconf); err != nil {
			r.log.Warnf("- %s", err.Error())
		}
	}

	// configure the gateway
	if r.config.NetworkConfig.Gateway != "" {
		gateway := net.ParseIP(r.config.NetworkConfig.Gateway)
		if gateway == nil {
			r.log.Warnf("Failed to configure gatway to %q", r.config.NetworkConfig.Gateway)
		}

		route := &netlink.Route{
			Scope: netlink.SCOPE_UNIVERSE,
			Gw:    gateway,
		}
		if err := netlink.RouteAdd(route); err != nil {
			r.log.Warnf("Failed to configure gateway: %v", err)
			return nil
		}
		r.log.Infof("Configured gatway to %s", r.config.NetworkConfig.Gateway)
	}

	// configure DNS
	if len(r.config.NetworkConfig.DNS) > 0 {
		// write the resolv.conf
		if err := os.RemoveAll("/etc/resolv.conf"); err != nil {
			r.log.Errorf("failed to cleanup old resolv.conf: %v", err)
			return nil
		}
		f, err := os.OpenFile("/etc/resolv.conf", os.O_CREATE, os.FileMode(0644))
		if err != nil {
			r.log.Errorf("failed to open /etc/resolv.conf: %v", err)
			return nil
		}
		defer f.Close()
		for _, ns := range r.config.NetworkConfig.DNS {
			if _, err := fmt.Fprintf(f, "nameserver %s\n", ns); err != nil {
				r.log.Errorf("failed to write to resolv.conf: %v", err)
				return nil
			}
		}
	}

	return nil
}

// createDirectories ensures the specified storage paths for pods and volumes
// exist.
func (r *runner) createDirectories() error {
	imagesPath := filepath.Join(kurmaPath, string(kurmaPathImages))
	podsPath := filepath.Join(kurmaPath, string(kurmaPathPods))
	volumesPath := filepath.Join(kurmaPath, string(kurmaPathVolumes))

	if err := os.MkdirAll(imagesPath, os.FileMode(0755)); err != nil {
		return fmt.Errorf("failed to create images directory: %v", err)
	}
	if err := os.MkdirAll(podsPath, os.FileMode(0755)); err != nil {
		return fmt.Errorf("failed to create pods directory: %v", err)
	}
	if err := os.MkdirAll(volumesPath, os.FileMode(0755)); err != nil {
		return fmt.Errorf("failed to create volumes directory: %v", err)
	}
	if err := os.MkdirAll(systemPodsPath, os.FileMode(0755)); err != nil {
		return fmt.Errorf("failed to create system pods directory: %v", err)
	}
	return nil
}

// rootReadonly makes the root parition read only.
func (r *runner) rootReadonly() error {
	return syscall.Mount("", "/", "", syscall.MS_REMOUNT|syscall.MS_RDONLY, "")
}

// displayNetwork will print out the current IP configuration of the ethernet
// devices.
func (r *runner) displayNetwork() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get all interfaces: %v", err)
	}

	r.log.Info(strings.Repeat("-", 30))
	defer r.log.Info(strings.Repeat("-", 30))
	r.log.Info("Network Information:")
	for _, in := range interfaces {
		ad, err := in.Addrs()
		if err != nil {
			return fmt.Errorf("failed to get addresses on interface %q: %v", in.Name, err)
		}
		addresses := make([]string, len(ad))
		for i, a := range ad {
			addresses[i] = a.String()
		}

		r.log.Infof("- %s: %s", in.Name, strings.Join(addresses, ", "))
	}

	return nil
}

// createImageManager creates the image manager that is used to store and
// handles provisioning of new pod mount namespaces.
func (r *runner) createImageManager() error {
	iopts := &imagestore.Options{
		Directory: filepath.Join(kurmaPath, string(kurmaPathImages)),
		Log:       r.log.Clone(),
	}
	imageManager, err := imagestore.New(iopts)
	if err != nil {
		return fmt.Errorf("failed to create the image manager: %v", err)
	}
	r.imageManager = imageManager
	return nil
}

// createPodManager creates the pod manager to allow pods to be
// launched.
func (r *runner) createPodManager() error {
	// retrieve the default stager
	if r.config.DefaultStagerImage == "" {
		return fmt.Errorf("a defaultStagerImage setting must be specified")
	}
	stagerHash, _, err := aci.Load(r.config.DefaultStagerImage, true, r.imageManager)
	if err != nil {
		return fmt.Errorf("failed to fetch default stager image %q: %v", r.config.DefaultStagerImage, err)
	}

	mopts := &podmanager.Options{
		PodDirectory:          filepath.Join(kurmaPath, string(kurmaPathPods)),
		LibcontainerDirectory: filepath.Join(kurmaPath, string(kurmaPathPods), "libcontainer"),
		VolumeDirectory:       filepath.Join(kurmaPath, string(kurmaPathVolumes)),
		ParentCgroupName:      r.config.ParentCgroupName,
		DefaultStagerHash:     stagerHash,
		Log:                   r.log.Clone(),
	}
	m, err := podmanager.NewManager(r.imageManager, nil, mopts)
	if err != nil {
		return fmt.Errorf("failed to create the pod manager: %v", err)
	}
	r.podManager = m
	r.log.Trace("Pod Manager has been initialized.")

	os.Chdir(kurmaPath)
	return nil
}

// createNetworkManager creates the network manager which launches the network
// provisioner pods.
func (r *runner) createNetworkManager() error {
	networkManager, err := networkmanager.New(r.podManager)
	if err != nil {
		r.log.Errorf("Failed to create network manager: %v", err)
		return nil
	}
	networkManager.SetLog(r.log.Clone())
	r.networkManager = networkManager
	r.podManager.SetNetworkManager(networkManager)

	networkDrivers := make([]*backend.NetworkDriver, 0, len(r.config.PodNetworks))

	for _, podNet := range r.config.PodNetworks {
		hash, _, err := aci.Load(podNet.ACI, true, r.imageManager)
		if err != nil {
			r.log.Warnf("Failed to load image for network %q: %v", podNet.Name, err)
			continue
		}

		imageID, err := types.NewHash(hash)
		if err != nil {
			r.log.Warnf("Failed to generate image hash for %q: %v", podNet.Name, err)
			continue
		}

		driver := &backend.NetworkDriver{
			Image: schema.RuntimeImage{
				ID: *imageID,
			},
			Configuration: podNet,
		}
		networkDrivers = append(networkDrivers, driver)
	}

	if err := r.networkManager.Setup(networkDrivers); err != nil {
		r.log.Errorf("Failed to set up the networking pod: %v", err)
	}
	return nil
}

// startSignalHandling configures the necessary signal handlers for the init
// process.
func (r *runner) startSignalHandling() error {
	// configure SIGCHLD
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGCHLD)
	go r.handleSIGCHLD(ch)
	return nil
}

// markBootSuccessful sets the GPT flags for a successful boot on the partition
// associated with the kernel
func (r *runner) markBootSuccessful() error {
	if r.config.SuccessfulBoot == nil {
		return nil
	}

	device := devices.ResolveDevice(*r.config.SuccessfulBoot)
	if device == "" {
		r.log.Warnf("Failed to resolve boot device of %q", *r.config.SuccessfulBoot)
		return nil
	}

	rawdev := devices.GetRawDevice(device)
	partnum := devices.GetPartitionNumber(device)
	if rawdev == "" || partnum == "" {
		r.log.Warnf("Failed to resolve device/partition: %q / %q", rawdev, partnum)
		return nil
	}

	out, err := exec.Command("/bin/cgpt", "add", rawdev, "-i", partnum, "-S1", "-T0").CombinedOutput()
	if err != nil {
		r.log.Warnf("Failed to mark boot successful: %s", string(out))
		return nil
	}
	out, err = exec.Command("/bin/cgpt", "prioritize", rawdev, "-i", partnum).CombinedOutput()
	if err != nil {
		r.log.Warnf("Failed to prioritize boot device: %s", string(out))
		return nil
	}
	return nil
}

// startServer begins the main Kurma RPC server and will take over execution.
func (r *runner) startServer() error {
	perms := os.FileMode(0770)
	group := 200

	opts := &daemon.Options{
		ImageManager:      r.imageManager,
		PodManager:        r.podManager,
		SocketFile:        filepath.Join(kurmaPath, "socket"),
		SocketPermissions: &perms,
		SocketGroup:       &group,
	}

	s := daemon.New(opts)
	if err := s.Start(); err != nil {
		r.log.Errorf("Error with Kurma server: %v", err)
		return err
	}
	return nil
}

// startInitialPods launches the initial pods that are specified in the
// configuration.
func (r *runner) startInitialPods() error {
	for d, ip := range r.config.InitialPods {
		name, podManifest, err := ip.Process(r.imageManager)
		if name == "" {
			name = fmt.Sprintf("pod%d", d+1)
		}
		if err != nil {
			r.log.Errorf("Failed to configure pod %q: %v", name, err)
			continue
		}

		pod, err := r.podManager.Create(name, podManifest, nil)
		if err != nil {
			r.log.Errorf("Failed to launch pod %q: %v", name, err)
			continue
		}
		r.log.Infof("Launched pod %q.", pod.Name())
	}
	return nil
}

// runUdev handles launching the udev service.
func (r *runner) runUdev() error {
	output, err := exec.Command("/bin/udev-setup.sh").CombinedOutput()
	if err != nil {
		r.log.Warnf("Failed to run udev: %s", string(output))
	}
	return nil
}

// startConsole handles launching the udev service.
func (r *runner) startConsole() error {
	if r.config.Console.Enabled == nil || !*r.config.Console.Enabled {
		r.log.Trace("Skipping console")
		return nil
	}

	// Get the pod manifest
	_, podManifest, err := r.config.Console.Process(r.imageManager)
	if err != nil {
		r.log.Warnf("Failed to fetch console pod: %v", err)
		return nil
	}

	// Change the pod manifest to add a host namespace isolator
	i, err := kschema.GenerateHostNamespaceIsolator()
	if err != nil {
		r.log.Warnf("Failed to configure console to use host namespaces: %v", err)
		return nil
	}
	podManifest.Isolators = append(podManifest.Isolators, *i)

	// send in the configuration information
	for i, runtimeApp := range podManifest.Apps {
		if runtimeApp.App == nil {
			imageManifest := r.imageManager.GetImage(runtimeApp.Image.ID.String())
			if imageManifest == nil {
				r.log.Warn("Failed to locate console image")
				return nil
			}
			runtimeApp.App = imageManifest.App
		}
		if r.config.Console.Password != nil {
			runtimeApp.App.Environment.Set(
				"CONSOLE_PASSWORD", *r.config.Console.Password)
		}
		runtimeApp.App.Environment.Set(
			"CONSOLE_KEYS", strings.Join(r.config.Console.SSHKeys, "\n"))
		podManifest.Apps[i] = runtimeApp
	}

	if _, err := r.podManager.Create("console", podManifest, nil); err != nil {
		return fmt.Errorf("Failed to start console: %v", err)
	}
	r.log.Debug("Started console")
	return nil
}

func (r *runner) setupDiscoveryProxy() error {
	if r.config.NetworkConfig.ProxyURL == "" {
		return nil
	}
	uri, err := url.ParseRequestURI(r.config.NetworkConfig.ProxyURL)
	if err != nil {
		r.log.Warnf("Failed to parse proxy url: %v", err)
		return nil
	}

	// discovery requests
	transport, ok := discovery.Client.Transport.(*http.Transport)
	if !ok {
		r.log.Warnf("Failed to configure discovery proxy, transport was not the expected type: %T",
			discovery.Client.Transport)
		return nil
	}
	transport.Proxy = http.ProxyURL(uri)

	// actual download requests
	transport, ok = aciremote.Client.Transport.(*http.Transport)
	if !ok {
		r.log.Warnf("Failed to configure remote download proxy, transport was not the expected type: %T",
			aciremote.Client.Transport)
		return nil
	}
	transport.Proxy = http.ProxyURL(uri)

	return nil
}

// prefetchImages is used to fetch specified images on start up to pre-load
// them.
func (r *runner) prefetchImages() error {
	for _, aci := range r.config.PrefetchImages {
		// TODO: is `r.imageManager` needed?
		// TODO: configurable `insecure` option
		_, _, err := fetch.FetchAndLoad(img, nil, true, r.imageManager)
		if err != nil {
			r.log.Warnf("Failed to fetch image %q: %v", img, err)
			continue
		}
		r.log.Debugf("Fetched image %s", aci)
	}
	return nil
}
