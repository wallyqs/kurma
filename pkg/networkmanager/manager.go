// Copyright 2016 Apcera Inc. All rights reserved.

package networkmanager

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/networkmanager/types"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"

	kschema "github.com/apcera/kurma/schema"
	atypes "github.com/appc/spec/schema/types"
)

const (
	netNsVolumeName    = "kurma-network-ns"
	netNsContainerPath = "/var/lib/kurma/netns"
)

// Manager handles the management of the pods running and available on the
// current host.
type Manager struct {
	log *logray.Logger

	networkPod backend.Pod

	netNsPath string

	drivers      []*networkDriver
	driversMutex sync.RWMutex

	podManager backend.PodManager
}

// New will create and return a new Manager for managing network plugins.
func New(podManager backend.PodManager) (backend.NetworkManager, error) {
	netnsPath, err := ioutil.TempDir(os.TempDir(), "kurma-netns")
	if err != nil {
		return nil, fmt.Errorf("failed to generate network namespace temp path: %v", err)
	}
	if err := os.Chmod(netnsPath, os.FileMode(0770)); err != nil {
		return nil, fmt.Errorf("failed to chmod network namespace temp path: %v", err)
	}
	if err := syscall.Mount(netnsPath, netnsPath, "", syscall.MS_BIND, ""); err != nil {
		return nil, fmt.Errorf("failed to bind mount network namespace temp path: %v", err)
	}
	if err := syscall.Mount("none", netnsPath, "", syscall.MS_SHARED, ""); err != nil {
		return nil, fmt.Errorf("failed to make network namespace temp path shared: %v", err)
	}

	m := &Manager{
		log:        logray.New(),
		netNsPath:  netnsPath,
		drivers:    make([]*networkDriver, 0),
		podManager: podManager,
	}
	return m, nil
}

// SetLog sets the logger to be used by the manager.
func (m *Manager) SetLog(log *logray.Logger) {
	m.log = log
}

// Setup is used to launch the networking pod with the provided set of plugin
// drivers.
func (m *Manager) Setup(drivers []*backend.NetworkDriver) error {
	networkPodManifest, podOptions, err := m.defaultNetworkPod()
	if err != nil {
		return fmt.Errorf("failed to generate the default PodManifest: %v", err)
	}

	// Populate the pod with the apps
	networkPodManifest.Apps = make([]schema.RuntimeApp, len(drivers))
	for i, driver := range drivers {
		networkPodManifest.Apps[i].Name = atypes.ACName(driver.Configuration.Name)
		networkPodManifest.Apps[i].Image = driver.Image
		networkPodManifest.Apps[i].Mounts = []schema.Mount{
			schema.Mount{
				Volume: atypes.ACName(netNsVolumeName),
				Path:   netNsContainerPath,
			},
		}
		// FIXME REMOVE!
		networkPodManifest.Apps[i].App = &atypes.App{
			Exec:  []string{"/bin/sleep", "600001"},
			User:  "0",
			Group: "0",
			Isolators: []atypes.Isolator{
				atypes.Isolator{
					Name:     atypes.ACIdentifier(kschema.LinuxPrivilegedName),
					ValueRaw: rawValue("true"),
				},
			},
		}

		// generate the local object
		d := &networkDriver{
			manager:       m,
			config:        driver.Configuration,
			podInterfaces: make(map[string]string),
		}
		m.driversMutex.Lock()
		m.drivers = append(m.drivers, d)
		m.driversMutex.Unlock()
	}

	// launch it
	networkPod, err := m.podManager.Create("kurma-networking", networkPodManifest, podOptions)
	if err != nil {
		return fmt.Errorf("failed to launch network pod: %v", err)
	}
	if err := networkPod.WaitForState(time.Minute, backend.RUNNING, backend.STOPPED, backend.ERRORED); err != nil {
		return fmt.Errorf("failed to wait for network pod to start: %v", err)
	}
	if state := networkPod.State(); state != backend.RUNNING {
		return fmt.Errorf("network pod failed to be running, is in the %v state", state)
	}
	m.networkPod = networkPod
	m.log.Tracef("Network pod provisioned and running: %s", m.networkPod.UUID())

	return nil
}

// Provision handles setting up the networking for a new pod. It is
// responsible for instrumenting the necessary network plugins for the
// pod.
func (m *Manager) Provision(pod backend.Pod) (string, []*types.IPResult, error) {
	m.driversMutex.RLock()
	defer m.driversMutex.RUnlock()

	netNsPath := filepath.Join(m.netNsPath, pod.UUID())
	if err := createNetworkNamespace(netNsPath); err != nil {
		return "", nil, fmt.Errorf("failed to create network namespace: %v", err)
	}

	if m.networkPod == nil {
		m.log.Tracef("Network provisioning skipped on %q, network pod is nil", pod.UUID())
		return "", nil, nil
	}

	results := make([]*types.IPResult, 0)

	for _, driver := range m.drivers {
		iface, err := driver.generateInterfaceName(pod, results)
		if err != nil {
			m.log.Warnf("Failed to generate interface name: %v", err)
			continue
		}
		driver.podInterfacesMutex.Lock()
		driver.podInterfaces[pod.UUID()] = iface
		driver.podInterfacesMutex.Unlock()

		var result *types.IPResult
		if err := m.processDriver(driver, pod, callAdd, &result); err != nil {
			if err == callTimeout {
				m.log.Warnf("Provision call on %q timed out", driver.config.Name)
			} else {
				m.log.Error(err.Error())
			}
			continue
		}

		result.Name = driver.config.Name
		result.ContainerInterface = iface
		m.log.Tracef("Provisioned networking. driver: %q, container: %q", result.Name, result.ContainerInterface)
		results = append(results, result)
	}

	return netNsPath, results, nil
}

// Deprovision is called when a pod is shutting down to handle any
// deallocation or cleanup processes that are necessary.
func (m *Manager) Deprovision(pod backend.Pod) error {
	m.driversMutex.RLock()
	defer m.driversMutex.RUnlock()

	if m.networkPod != nil {
		for _, driver := range m.drivers {
			if err := m.processDriver(driver, pod, callDel, nil); err != nil {
				if err == callTimeout {
					m.log.Warnf("Teardown call on %q timed out", driver.config.Name)
				} else {
					m.log.Error(err.Error())
				}
			}
			driver.podInterfacesMutex.Lock()
			delete(driver.podInterfaces, pod.UUID())
			driver.podInterfacesMutex.Unlock()
		}
	}

	netNsPath := filepath.Join(m.netNsPath, pod.UUID())
	if err := deleteNetworkNamespace(netNsPath); err != nil {
		return fmt.Errorf("failed to cleanup network namespace: %v", err)
	}

	return nil
}

// processDriver handles calling into a individual network plugin to
// provision/deprovision networking.
func (m *Manager) processDriver(driver *networkDriver, pod backend.Pod, callCmd string, result interface{}) error {
	if err := driver.call(callCmd, driver.generateArgs(pod), result); err != nil {
		if err == callTimeout {
			return err
		}
		return fmt.Errorf("Failed to provision network with %q: %v", driver.config.Name, err)
	}
	return nil
}
