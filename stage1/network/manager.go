// Copyright 2016 Apcera Inc. All rights reserved.

package network

import (
	"fmt"
	"sync"
	"time"

	"github.com/apcera/kurma/networking/types"
	"github.com/apcera/kurma/stage1"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"
	atypes "github.com/appc/spec/schema/types"
)

// Manager handles the management of the containers running and available on the
// current host.
type Manager struct {
	log *logray.Logger

	drivers      map[string]*networkDriver
	driversMutex sync.RWMutex

	containerManager stage1.ContainerManager
}

// New will create and return a new Manager for managing network plugins.
func New(containerManager stage1.ContainerManager) (stage1.NetworkManager, error) {
	m := &Manager{
		log:              logray.New(),
		drivers:          make(map[string]*networkDriver),
		containerManager: containerManager,
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
		config:              config,
		containerInterfaces: make(map[string]string),
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

	// launch it
	container, err := m.containerManager.Create(fmt.Sprintf("network-%s", config.Name), imageManifest, imageHash)
	if err != nil {
		return fmt.Errorf("failed to launch network driver container: %v", err)
	}
	driver.uuid = container.UUID()
	defer func() {
		if !success {
			container.Stop()
		}
	}()

	// wait for the container to be ready
	ready := false
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if container.State() == stage1.RUNNING {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("container failed to be ready within timeout: %v", container.State())
	}

	// call the setup function
	if err := driver.call(container, callSetup, nil, nil); err != nil {
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

	// get the container and stop it
	container := m.containerManager.Container(driver.uuid)
	if container != nil {
		container.Stop()
	}

	// remove it from the map
	delete(m.drivers, name)
	return nil
}

// Provision handles setting up the networking for a new container. It is
// responsible for instrumenting the necessary network plugins for the
// container.
func (m *Manager) Provision(container stage1.Container) ([]*types.IPResult, error) {
	m.driversMutex.RLock()
	defer m.driversMutex.RUnlock()

	results := make([]*types.IPResult, 0)

	for _, driver := range m.drivers {
		iface, err := driver.generateInterfaceName(container, results)
		if err != nil {
			m.log.Warnf("Failed to generate interface name: %v", err)
			continue
		}
		driver.containerInterfacesMutex.Lock()
		driver.containerInterfaces[container.UUID()] = iface
		driver.containerInterfacesMutex.Unlock()

		var result *types.IPResult
		if err := m.processDriver(driver, container, callAdd, &result); err != nil {
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

// Deprovision is called when a container is shutting down to handle any
// deallocation or cleanup processes that are necessary.
func (m *Manager) Deprovision(container stage1.Container) error {
	m.driversMutex.RLock()
	defer m.driversMutex.RUnlock()

	for _, driver := range m.drivers {
		if err := m.processDriver(driver, container, callDel, nil); err != nil {
			if err == callTimeout {
				m.log.Warnf("Teardown call on %q timed out", driver.config.Name)
			} else {
				m.log.Error(err.Error())
			}
		}
		driver.containerInterfacesMutex.Lock()
		delete(driver.containerInterfaces, container.UUID())
		driver.containerInterfacesMutex.Unlock()
	}

	return nil
}

// processDriver handles calling into a individual network plugin to
// provision/deprovision networking.
func (m *Manager) processDriver(driver *networkDriver, container stage1.Container, callCmd string, result interface{}) error {
	driverContainer := m.containerManager.Container(driver.uuid)
	if driverContainer == nil {
		return fmt.Errorf("Container for driver %q is missing", driver.config.Name)
	}

	args, err := driver.generateArgs(container)
	if err != nil {
		return fmt.Errorf("Failed to generate arguments for %q: %v", driver.config.Name, err)
	}

	if err := driver.call(driverContainer, callCmd, args, result); err != nil {
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
	config                   *types.NetConf
	uuid                     string
	containerInterfaces      map[string]string
	containerInterfacesMutex sync.RWMutex
}

// generateArgs creates the relevant command line arguments that need to be
// passed to the driver.
func (d *networkDriver) generateArgs(targetContainer stage1.Container) ([]string, error) {
	pid, err := targetContainer.Pid()
	if err != nil {
		return nil, fmt.Errorf("failed to get pid for container's namespace: %v", err)
	}

	uuid := targetContainer.UUID()
	d.containerInterfacesMutex.RLock()
	iface := d.containerInterfaces[uuid]
	d.containerInterfacesMutex.RUnlock()
	return []string{fmt.Sprintf("/proc/%d/ns/net", pid), uuid, iface}, nil
}
