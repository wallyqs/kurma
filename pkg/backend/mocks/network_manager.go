// Copyright 2016 Apcera Inc. All rights reserved.

package mocks

import (
	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/logray"

	ntypes "github.com/apcera/kurma/pkg/networkmanager/types"
)

type NetworkManager struct {
	SetupFunc       func(drivers []*backend.NetworkDriver) error
	ProvisionFunc   func(pod backend.Pod) (string, []*ntypes.IPResult, error)
	DeprovisionFunc func(pod backend.Pod) error
}

func (nm *NetworkManager) SetLog(log *logray.Logger) {}

func (nm *NetworkManager) Setup(drivers []*backend.NetworkDriver) error {
	return nm.SetupFunc(drivers)
}

func (nm *NetworkManager) Provision(pod backend.Pod) (string, []*ntypes.IPResult, error) {
	return nm.ProvisionFunc(pod)
}

func (nm *NetworkManager) Deprovision(pod backend.Pod) error {
	return nm.DeprovisionFunc(pod)
}
