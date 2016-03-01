// Copyright 2016 Apcera Inc. All rights reserved.

package network

import (
	"fmt"
	"sync"
	"time"

	"github.com/apcera/kurma/network/types"
	"github.com/apcera/kurma/stage1"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"
	atypes "github.com/appc/spec/schema/types"
)

// Manager handles the management of the pods running and available on the
// current host.
type Manager struct {
	log *logray.Logger

	drivers      map[string]*networkDriver
	driversMutex sync.RWMutex

	podManager stage1.PodManager
}

// New will create and return a new Manager for managing network plugins.
func New(podManager stage1.PodManager) (stage1.NetworkManager, error) {
	m := &Manager{
		log:        logray.New(),
		drivers:    make(map[string]*networkDriver),
		podManager: podManager,
	}
	return m, nil
}

// SetLog sets the logger to be used by the manager.
func (m *Manager) SetLog(log *logray.Logger) {
	m.log = log
}

// CreateDriver handles the launching of a new networking plugin within the
// system.
func (m *Manager) CreateDriver(imageManifest *schema.ImageManifest, imageHash string, config *types.NetConf) error {
	m.driversMutex.Lock()
	defer m.driversMutex.Unlock()

	// ensure we don't get a duplicate
	if _, exists := m.drivers[config.Name]; exists {
		return fmt.Errorf("network by that name already exists")
	}

	// generate the local object
	driver := &networkDriver{
		config:        config,
		podInterfaces: make(map[string]string),
	}

	// apply the namespace controls to it
	if err := updateManifestForNamespaces(imageManifest); err != nil {
		return err
	}

	// add the networking label onto the image
	label := atypes.Label{Name: atypes.ACIdentifier("network-driver"), Value: config.Name}
	imageManifest.Labels = append(imageManifest.Labels, label)
	imageManifest.App.Exec = nil

	// prepare to unroll if it fails
	success := false

	// create the pod manifest
	podManifest := schema.BlankPodManifest()
	imageID, err := atypes.NewHash(imageHash)
	if err != nil {
		return err
	}

	// create the RuntimeApp
	runtimeApp := schema.RuntimeApp{
		Name: *atypes.MustACName(config.Name),
		Image: schema.RuntimeImage{
			ID: *imageID,
		},
	}
	podManifest.Apps = append(podManifest.Apps, runtimeApp)

	// FIXME apply isolator for namespaces

	// launch it
	pod, err := m.podManager.Create(fmt.Sprintf("network-%s", config.Name), podManifest)
	if err != nil {
		return fmt.Errorf("failed to launch network driver pod: %v", err)
	}
	driver.uuid = pod.UUID()
	defer func() {
		if !success {
			pod.Stop()
		}
	}()

	// wait for the pod to be ready
	ready := false
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if pod.State() == stage1.RUNNING {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("pod failed to be ready within timeout: %v", pod.State())
	}

	// call the setup function
	if err := driver.call(pod, callSetup, nil, nil); err != nil {
		if err == callTimeout {
			return fmt.Errorf("driver setup step timed out")
		}
		return err
	}

	m.drivers[config.Name] = driver
	success = true
	return nil
}

// DeleteDriver handles removing a networking plugin from the system.
func (m *Manager) DeleteDriver(name string) error {
	m.driversMutex.Lock()
	defer m.driversMutex.Unlock()

	// ensure the driver exists, just return if it doesn't
	driver, exists := m.drivers[name]
	if !exists {
		return nil
	}

	// get the pod and stop it
	pod := m.podManager.Pod(driver.uuid)
	if pod != nil {
		pod.Stop()
	}

	// remove it from the map
	delete(m.drivers, name)
	return nil
}

// Provision handles setting up the networking for a new pod. It is
// responsible for instrumenting the necessary network plugins for the
// pod.
func (m *Manager) Provision(pod stage1.Pod) ([]*types.IPResult, error) {
	m.driversMutex.RLock()
	defer m.driversMutex.RUnlock()

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
		results = append(results, result)
	}

	return results, nil
}

// Deprovision is called when a pod is shutting down to handle any
// deallocation or cleanup processes that are necessary.
func (m *Manager) Deprovision(pod stage1.Pod) error {
	m.driversMutex.RLock()
	defer m.driversMutex.RUnlock()

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

	return nil
}

// processDriver handles calling into a individual network plugin to
// provision/deprovision networking.
func (m *Manager) processDriver(driver *networkDriver, pod stage1.Pod, callCmd string, result interface{}) error {
	driverPod := m.podManager.Pod(driver.uuid)
	if driverPod == nil {
		return fmt.Errorf("Pod for driver %q is missing", driver.config.Name)
	}

	args, err := driver.generateArgs(pod)
	if err != nil {
		return fmt.Errorf("Failed to generate arguments for %q: %v", driver.config.Name, err)
	}

	if err := driver.call(driverPod, callCmd, args, result); err != nil {
		if err == callTimeout {
			return err
		}
		return fmt.Errorf("Failed to provision network with %q: %v", driver.config.Name, err)
	}

	return nil
}

// networkDriver captures some of the state information about the configured
// network plugins.
type networkDriver struct {
	config             *types.NetConf
	uuid               string
	podInterfaces      map[string]string
	podInterfacesMutex sync.RWMutex
}

// generateArgs creates the relevant command line arguments that need to be
// passed to the driver.
func (d *networkDriver) generateArgs(targetPod stage1.Pod) ([]string, error) {
	// FIXME!!!
	pid := 1234
	// pid, err := targetPod.Pid()
	// if err != nil {
	// return nil, fmt.Errorf("failed to get pid for pod's namespace: %v", err)
	// }

	uuid := targetPod.UUID()
	d.podInterfacesMutex.RLock()
	iface := d.podInterfaces[uuid]
	d.podInterfacesMutex.RUnlock()
	return []string{fmt.Sprintf("/proc/%d/ns/net", pid), uuid, iface}, nil
}
