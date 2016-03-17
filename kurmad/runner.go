// Copyright 2016 Apcera Inc. All rights reserved.

package kurmad

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/apcera/kurma/pkg/aciremote"
	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/daemon"
	"github.com/apcera/kurma/pkg/imagestore"
	"github.com/apcera/kurma/pkg/networkmanager"
	"github.com/apcera/kurma/pkg/podmanager"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/ghodss/yaml"
)

// loadConfigurationFile is used to parse the provided configuration file.
func (r *runner) loadConfigurationFile() error {
	if r.configFile == "" {
		return fmt.Errorf("FIXME: must specify a configuration file right now")
	}

	var unmarshalFunc func([]byte, interface{}) error
	switch filepath.Ext(r.configFile) {
	case ".json":
		unmarshalFunc = json.Unmarshal
	case ".yml", ".yaml":
		unmarshalFunc = yaml.Unmarshal
	default:
		return fmt.Errorf("Unrecognized configation file format, please use JSON or YAML")
	}

	f, err := os.Open(r.configFile)
	if err != nil {
		return fmt.Errorf("failed to open configuration file: %v", err)
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %v", err)
	}

	if err := unmarshalFunc(b, &r.config); err != nil {
		return fmt.Errorf("failed to parse configuration file: %v", err)
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

// createDirectories ensures the specified storage paths for pods and volumes
// exist.
func (r *runner) createDirectories() error {
	if err := os.MkdirAll(r.config.ImagesDirectory, os.FileMode(0755)); err != nil {
		return fmt.Errorf("failed to create images directory: %v", err)
	}
	if err := os.MkdirAll(r.config.PodsDirectory, os.FileMode(0755)); err != nil {
		return fmt.Errorf("failed to create pods directory: %v", err)
	}
	if err := os.MkdirAll(r.config.VolumesDirectory, os.FileMode(0755)); err != nil {
		return fmt.Errorf("failed to create pods directory: %v", err)
	}
	return nil
}

// createImageManager creates the image manager that is used to store and
// handles provisioning of new pod mount namespaces.
func (r *runner) createImageManager() error {
	iopts := &imagestore.Options{
		Directory: r.config.ImagesDirectory,
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
	mopts := &podmanager.Options{
		PodDirectory:          r.config.PodsDirectory,
		LibcontainerDirectory: filepath.Join(r.config.PodsDirectory, "libcontainer"),
		VolumeDirectory:       r.config.VolumesDirectory,
		ParentCgroupName:      r.config.ParentCgroupName,
		Log:                   r.log.Clone(),
	}
	m, err := podmanager.NewManager(r.imageManager, nil, mopts)
	if err != nil {
		return fmt.Errorf("failed to create the pod manager: %v", err)
	}
	r.podManager = m
	r.log.Trace("Pod Manager has been initialized.")
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
		hash, _, err := aciremote.LoadImage(podNet.ACI, true, r.imageManager)
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

// startDaemon begins the main Kurma RPC server and will take over execution.
func (r *runner) startDaemon() error {
	perms := os.FileMode(0666)
	if r.config.SocketPermissions != nil {
		perms = os.FileMode(*r.config.SocketPermissions)
	}

	opts := &daemon.Options{
		ImageManager:         r.imageManager,
		PodManager:           r.podManager,
		SocketRemoveIfExists: true,
		SocketFile:           r.config.SocketPath,
		SocketPermissions:    &perms,
	}

	s := daemon.New(opts)
	if err := s.Start(); err != nil {
		r.log.Errorf("Error with Kurma server: %v", err)
		return err
	}
	r.log.Infof("kurmad now ready at path unix://%s", r.config.SocketPath)
	return nil
}

// startInitialPods runs the initial pods from the configuration file.
func (r *runner) startInitialPods() error {
	for d, ip := range r.config.InitialPods {
		name, podManifest, err := ip.process(r)
		if name == "" {
			name = fmt.Sprintf("initial-pod-%d", d+1)
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
