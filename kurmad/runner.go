// Copyright 2016 Apcera Inc. All rights reserved.

package kurmad

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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

// setupRunner represents the behavior required to handle setting up kurmad.
type setupRunner interface {
	setupSignalHandling()
	loadConfigurationFile() error
	configureLogging()
	createDirectories() error
	createImageManager() error
	prefetchImages()
	createPodManager() error
	createNetworkManager()
	startDaemon() error
	startInitialPods()
}

// runner is an object that is used to handle the startup of the system.
// It will take of the running of the process once init.Run() is invoked.
type runner struct {
	config         *Config
	configFile     string
	log            *logray.Logger
	podManager     backend.PodManager
	imageManager   backend.ImageManager
	networkManager backend.NetworkManager
}

// setupSignalHandling sets up the callbacks for signals to cleanly shutdown.
func (r *runner) setupSignalHandling() {
	signalc := make(chan os.Signal, 1)
	signal.Notify(signalc, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Watch the channel and handle any signals that come in.
	go func() {
		for sig := range signalc {
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
				r.log.Infof("Received %s. Shutting down.", sig.String())
				if r.podManager != nil {
					r.podManager.Shutdown()
				}
				r.log.Flush()
				fmt.Fprintln(os.Stderr, "Shutdown complete, exiting")
				os.Exit(0)
			default:
				r.log.Warnf("Received %s. Ignoring.", sig.String())
			}
		}
	}()

	return
}

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
func (r *runner) configureLogging() {
	if r.config.Debug {
		logray.ResetDefaultLogLevel(logray.ALL)
	}
	return
}

// createDirectories ensures the specified storage paths for pods and volumes
// exist.
func (r *runner) createDirectories() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine current working directory: %v", err)
	}

	// Ensure directories are absolute paths
	if !filepath.IsAbs(r.config.ImagesDirectory) {
		r.config.ImagesDirectory = filepath.Join(wd, r.config.ImagesDirectory)
	}
	if !filepath.IsAbs(r.config.PodsDirectory) {
		r.config.PodsDirectory = filepath.Join(wd, r.config.PodsDirectory)
	}
	if !filepath.IsAbs(r.config.VolumesDirectory) {
		r.config.VolumesDirectory = filepath.Join(wd, r.config.VolumesDirectory)
	}

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

// prefetchImages is used to fetch specified images on start up to pre-load
// them.
func (r *runner) prefetchImages() {
	for _, aci := range r.config.PrefetchImages {
		_, _, err := aciremote.LoadImage(aci, true, r.imageManager)
		if err != nil {
			r.log.Warnf("Failed to fetch image %q: %v", aci, err)
			continue
		}
		r.log.Debugf("Fetched image %s", aci)
	}
	return
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
	// retrieve the default stager
	if r.config.DefaultStagerImage == "" {
		return fmt.Errorf("a defaultStagerImage setting must be specified")
	}
	stagerHash, _, err := aciremote.LoadImage(r.config.DefaultStagerImage, true, r.imageManager)
	if err != nil {
		return fmt.Errorf("failed to fetch default stager image %q: %v", r.config.DefaultStagerImage, err)
	}

	mopts := &podmanager.Options{
		PodDirectory:          r.config.PodsDirectory,
		LibcontainerDirectory: filepath.Join(r.config.PodsDirectory, "libcontainer"),
		VolumeDirectory:       r.config.VolumesDirectory,
		ParentCgroupName:      r.config.ParentCgroupName,
		DefaultStagerHash:     stagerHash,
		Log:                   r.log.Clone(),
		Debug:                 r.config.Debug,
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
func (r *runner) createNetworkManager() {
	networkManager, err := networkmanager.New(r.podManager)
	if err != nil {
		r.log.Errorf("Skipping networking because creation of network manager failed: %s", err)
		return
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
	return
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
func (r *runner) startInitialPods() {
	for d, ip := range r.config.InitialPods {
		name, podManifest, err := ip.Process(r.imageManager)
		if name == "" {
			name = fmt.Sprintf("initial-pod-%d", d+1)
		}
		if err != nil {
			r.log.Errorf("Failed to configure pod %q, skipping: %s", name, err)
			continue
		}

		pod, err := r.podManager.Create(name, podManifest, nil)
		if err != nil {
			r.log.Errorf("Failed to launch pod %q: %v", name, err)
			continue
		}
		l := r.log.Clone()
		l.SetField("pod", pod.UUID())
		l.Infof("Launched initial pod %q.", pod.Name())
	}
	return
}
