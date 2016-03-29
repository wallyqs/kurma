// Copyright 2015-2016 Apcera Inc. All rights reserved.

package init

var (
	// The setup functions that should be run in order to handle setting up the
	// host system to create and manage pods. These functions focus primarily on
	// runtime actions that must be done each time on boot.
	setupFunctions = []func(*runner) error{
		// Basic system startup and configuration
		(*runner).startSignalHandling,
		(*runner).switchRoot,
		(*runner).createSystemMounts,
		(*runner).loadConfigurationFile,
		(*runner).configureLogging,
		(*runner).configureEnvironment,
		(*runner).mountCgroups,
		(*runner).loadModules,

		// Prepping for starting managers
		(*runner).createDirectories,
		(*runner).runUdev,
		(*runner).mountDisks,
		(*runner).cleanOldPods,
		(*runner).createImageManager,
		(*runner).loadAvailableImages,
		(*runner).createPodManager,

		// Final system configuration and mark the boot as successful
		(*runner).configureHostname,
		(*runner).configureNetwork,
		(*runner).markBootSuccessful,

		// Some early image retrieval before starting initial processes
		(*runner).setupDiscoveryProxy,
		(*runner).prefetchImages,

		// Setup networking plugins
		(*runner).createNetworkManager,

		// Launch necessary services
		(*runner).startServer,
		(*runner).rootReadonly,
		(*runner).startInitialPods,
		(*runner).displayNetwork,
		(*runner).startConsole,
	}
)

const (
	// configurationFile is the source of the initial disk based configuration.
	configurationFile = "/etc/kurma.yml"

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
