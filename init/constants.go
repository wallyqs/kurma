// Copyright 2015-2016 Apcera Inc. All rights reserved.

package init

var (
	// The setup functions that should be run in order to handle setting up the
	// host system to create and manage pods. These functions focus primarily on
	// runtime actions that must be done each time on boot.
	setupFunctions = []func(*runner) error{
		(*runner).startSignalHandling,
		(*runner).switchRoot,
		(*runner).createSystemMounts,
		(*runner).loadConfigurationFile,
		(*runner).configureLogging,
		(*runner).configureEnvironment,
		(*runner).mountCgroups,
		(*runner).loadModules,
		(*runner).createDirectories,
		(*runner).createImageManager,
		(*runner).createPodManager,
		(*runner).startUdev,
		(*runner).mountDisks,
		(*runner).cleanOldPods,
		(*runner).rescanImages,
		(*runner).loadAvailableImages,
		(*runner).configureHostname,
		(*runner).configureNetwork,
		(*runner).createNetworkManager,
		(*runner).markBootSuccessful,
		(*runner).setupDiscoveryProxy,
		(*runner).startNTP,
		(*runner).startServer,
		(*runner).startInitPods,
		(*runner).displayNetwork,
		(*runner).startConsole,
		(*runner).rootReadonly,
	}
)

const (
	// configurationFile is the source of the initial disk based configuration.
	configurationFile = "/etc/kurma.json"

	// The default location where cgroups should be mounted. This is a constant
	// because it is referenced in multiple functions.
	cgroupsMount = "/sys/fs/cgroup"

	// This is the directory that is used when performing the switch_root, and
	// relocating off of the initramfs. Note, this is expected to include the
	// leading slash.
	newRoot = "/newroot"
)

// defaultConfiguration returns the default codified configuration that is
// applied on boot. This primarily ensures that at a bare minimum, a local
// loopback device will be configured for the network.
func defaultConfiguration() *kurmaConfig {
	return &kurmaConfig{
		NetworkConfig: kurmaNetworkConfig{
			Interfaces: []*kurmaNetworkInterface{
				&kurmaNetworkInterface{
					Device:  "lo",
					Address: "127.0.0.1/8",
				},
			},
		},
	}
}
