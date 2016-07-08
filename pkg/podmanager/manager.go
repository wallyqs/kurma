// Copyright 2015-2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/apcera/kurma/pkg/apiclient"
	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/logray"
	"github.com/apcera/util/uuid"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer"

	kschema "github.com/apcera/kurma/schema"
)

// Options contains settings that are used by the Pod Manager and
// Pods running on the host.
type Options struct {
	ParentCgroupName      string
	PodDirectory          string
	LibcontainerDirectory string
	VolumeDirectory       string
	DefaultStagerHash     string
	RequiredNamespaces    []string
	Log                   *logray.Logger
	FactoryFunc           func(root string) (libcontainer.Factory, error)
	Debug                 bool
}

func defaultFactory(root string) (libcontainer.Factory, error) {
	return libcontainer.New(root)
}

// Manager handles the management of the pods running and available on the
// current host.
type Manager struct {
	log     *logray.Logger
	Options *Options

	imageManager   backend.ImageManager
	networkManager backend.NetworkManager
	volumeLock     sync.Mutex

	// libconatiner related objects
	factory libcontainer.Factory

	pods     map[string]backend.Pod
	podNames map[string]string
	podsLock sync.RWMutex

	HostSocketFile string
}

// NewManager creates a new Manager with the provided options. It will ensure
// the manager is setup and ready to create pods with the provided
// configuration.
func NewManager(imageManager backend.ImageManager, networkManager backend.NetworkManager, opts *Options) (backend.PodManager, error) {
	// defaults
	if opts.Log == nil {
		opts.Log = logray.New()
	}
	if opts.FactoryFunc == nil {
		opts.FactoryFunc = defaultFactory
	}

	// create the libcontainer factory
	factory, err := opts.FactoryFunc(opts.LibcontainerDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to create the libcontainer factory: %v", err)
	}

	m := &Manager{
		log:            opts.Log,
		Options:        opts,
		factory:        factory,
		imageManager:   imageManager,
		networkManager: networkManager,
		pods:           make(map[string]backend.Pod),
		podNames:       make(map[string]string),
	}

	return m, nil
}

// SetHostSocketFile sets the path to the host's socket file for granting API
// access.
func (manager *Manager) SetHostSocketFile(hostSocketFile string) {
	manager.HostSocketFile = hostSocketFile
}

// SetNetworkManager sets the network manager that should be used to configure
// networking for pods.
func (manager *Manager) SetNetworkManager(networkManager backend.NetworkManager) {
	manager.networkManager = networkManager
}

// validate will ensure that the pod manifest provided is valid to be run on the
// system. It will return nil if it is valid, or will return an error if
// something is invalid.
func (manager *Manager) validate(manifest *schema.PodManifest) error {
	if len(manifest.Apps) == 0 {
		return fmt.Errorf("the manifest must specify an App")
	}

	// Validate each application
	for _, runtimeApp := range manifest.Apps {
		// Ensure we have the image already
		imageManifest := manager.imageManager.GetImage(runtimeApp.Image.ID.String())
		if imageManifest == nil {
			return fmt.Errorf("unable to load the ImageManifest for app %q", runtimeApp.Name)
		}

		// See if the runtimeApp specifies an app, or the image manifest
		if runtimeApp.App != nil || imageManifest.App != nil {
			continue
		}

		return fmt.Errorf("no App sets in the pod or image manifest for app %q", runtimeApp.Name)
	}

	// If the namespaces isolator is specified, validate a minimum set of namespaces
	for _, iso := range manifest.Isolators {
		if iso.Name != kschema.LinuxNamespacesName {
			continue
		}
		if niso, ok := iso.Value().(*kschema.LinuxNamespaces); ok {
			checks := map[string]func() kschema.LinuxNamespaceValue{
				"ipc":  niso.IPC,
				"net":  niso.Net,
				"pid":  niso.PID,
				"user": niso.User,
				"uts":  niso.UTS,
			}
			for _, ns := range manager.Options.RequiredNamespaces {
				f, exists := checks[ns]
				if !exists {
					return fmt.Errorf("Internal server error")
				}
				if f() == kschema.LinuxNamespaceHost {
					return fmt.Errorf("the manifest %s isolator must require the %s namespace", kschema.LinuxNamespacesName, ns)
				}
			}
		}
	}

	return nil
}

// Create begins launching a pod with the provided image manifest and
// reader as the source of the ACI.
func (manager *Manager) Create(name string, manifest *schema.PodManifest, options *backend.PodOptions) (backend.Pod, error) {
	// revalidate the image
	if err := manager.validate(manifest); err != nil {
		return nil, err
	}

	if options == nil {
		options = &backend.PodOptions{}
	}

	if options.StagerHash == "" {
		options.StagerHash = manager.Options.DefaultStagerHash
	}

	// populate the pod
	pod := &Pod{
		manager:        manager,
		log:            manager.log.Clone(),
		uuid:           uuid.Variant4().String(),
		name:           name,
		options:        options,
		shuttingDownCh: make(chan struct{}),
		waitch:         make(chan bool),
		layerPaths:     make(map[string]string),
		manifest: &backend.StagerManifest{
			KurmaVersion:  apiclient.KurmaVersion.String(),
			Name:          name,
			Pod:           manifest,
			Images:        make(map[string]*schema.ImageManifest),
			AppImageOrder: make(map[string][]string),
			StagerConfig:  []byte(`{}`),
		},
	}
	pod.log.SetField("pod", pod.uuid)

	// add it to the manager's map
	manager.podsLock.Lock()

	// Validate the name isn't taken right before we added. Want to ensure no
	// races happen between checking and creating.
	if _, exists := manager.podNames[pod.name]; exists {
		manager.podsLock.Unlock()
		return nil, fmt.Errorf("a pod with the name %q already exists", pod.name)
	}

	manager.pods[pod.uuid] = pod
	manager.podNames[pod.name] = pod.uuid
	manager.podsLock.Unlock()

	// begin the startup sequence
	pod.log.Debugf("Launching pod %q", pod.name)
	pod.start()

	return pod, nil
}

// Shutdown requests that the pod manager shut down running pods to prepare to
// exit.
func (manager *Manager) Shutdown() {
	manager.podsLock.Lock()
	var networkPod backend.Pod
	networkPodUuid := manager.podNames["kurma-networking"]
	if networkPodUuid != "" {
		networkPod = manager.pods[networkPodUuid]
	}
	pods := manager.pods
	manager.podsLock.Unlock()

	wg := sync.WaitGroup{}
	for _, pod := range pods {
		if pod.UUID() == networkPodUuid {
			continue
		}
		wg.Add(1)
		go func(pod backend.Pod) {
			defer wg.Done()
			pod.Stop()
		}(pod)
	}

	wg.Wait()

	if networkPod != nil {
		networkPod.Stop()
	}
}

// removes a child pod from the Pod Manager.
func (manager *Manager) remove(pod *Pod) {
	manager.podsLock.Lock()
	pod.mutex.Lock()
	delete(manager.pods, pod.uuid)
	delete(manager.podNames, pod.name)
	pod.mutex.Unlock()
	manager.podsLock.Unlock()
}

// Pods returns a slice of the current pods on the host.
func (manager *Manager) Pods() []backend.Pod {
	manager.podsLock.RLock()
	defer manager.podsLock.RUnlock()
	pods := make([]backend.Pod, 0, len(manager.pods))
	for _, c := range manager.pods {
		pods = append(pods, c)
	}
	return pods
}

// Pod returns a specific pod matching the provided UUID, or nil if
// a pod with the UUID does not exist.
func (manager *Manager) Pod(uuid string) backend.Pod {
	manager.podsLock.RLock()
	defer manager.podsLock.RUnlock()
	return manager.pods[uuid]
}

// getVolumePath will get the absolute path on the host to the named volume. It
// will also ensure that the volume name exists within the volumes directory.
func (manager *Manager) getVolumePath(name string) (string, error) {
	if !types.ValidACName.MatchString(name) {
		return "", fmt.Errorf("invalid characters present in volume name")
	}

	volumePath := filepath.Join(manager.Options.VolumeDirectory, name)

	manager.volumeLock.Lock()
	defer manager.volumeLock.Unlock()

	if err := os.Mkdir(volumePath, os.FileMode(0755)); err != nil && !os.IsExist(err) {
		return "", err
	}
	return volumePath, nil
}
