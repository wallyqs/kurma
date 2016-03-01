// Copyright 2016 Apcera Inc. All rights reserved.

package pod

import (
	"os"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type mockFactory struct {
	containers map[string]*mockContainer
}

func newMockFactory() *mockFactory {
	return &mockFactory{
		containers: make(map[string]*mockContainer),
	}
}

func (mf *mockFactory) Create(id string, config *configs.Config) (libcontainer.Container, error) {
	mc := &mockContainer{
		id:     id,
		config: config,
	}
	mf.containers[id] = mc
	return mc, nil
}

func (mf *mockFactory) Load(id string) (libcontainer.Container, error) {
	return mf.containers[id], nil
}

func (mf *mockFactory) StartInitialization() error {
	return nil
}

func (mf *mockFactory) Type() string {
	return "mockFactory"
}

type mockContainer struct {
	id     string
	config *configs.Config
}

func (mc *mockContainer) ID() string {
	return mc.id
}

func (mc *mockContainer) Status() (libcontainer.Status, error) {
	return libcontainer.Running, nil
}

func (mc *mockContainer) State() (*libcontainer.State, error) {
	return nil, nil
}

func (mc *mockContainer) Config() configs.Config {
	return *mc.config
}

func (mc *mockContainer) Processes() ([]int, error) {
	return nil, nil
}

func (mc *mockContainer) Stats() (*libcontainer.Stats, error) {
	return nil, nil
}

func (mc *mockContainer) Set(config configs.Config) error {
	mc.config = &config
	return nil
}

func (mc *mockContainer) Start(process *libcontainer.Process) error {
	return nil
}

func (mc *mockContainer) Destroy() error {
	return nil
}

func (mc *mockContainer) Signal(s os.Signal) error {
	return nil
}

func (mc *mockContainer) Checkpoint(criuOpts *libcontainer.CriuOpts) error {
	return nil
}

func (mc *mockContainer) Restore(process *libcontainer.Process, criuOpts *libcontainer.CriuOpts) error {
	return nil
}

func (mc *mockContainer) Pause() error {
	return nil
}

func (mc *mockContainer) Resume() error {
	return nil
}

func (mc *mockContainer) NotifyOOM() (<-chan struct{}, error) {
	return nil, nil
}

func (mc *mockContainer) NotifyMemoryPressure(level libcontainer.PressureLevel) (<-chan struct{}, error) {
	return nil, nil
}
