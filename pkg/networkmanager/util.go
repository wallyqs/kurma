// Copyright 2016 Apcera Inc. All rights reserved.

package networkmanager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"syscall"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/appc/spec/schema"
	"github.com/opencontainers/runc/libcontainer/configs"

	kschema "github.com/apcera/kurma/schema"
	atypes "github.com/appc/spec/schema/types"
)

func (m *Manager) defaultNetworkPod() (*schema.PodManifest, *backend.PodOptions, error) {
	pod := schema.BlankPodManifest()

	i, err := generateNewIsolator()
	if err != nil {
		return nil, nil, err
	}
	pod.Isolators = []atypes.Isolator{*i}

	options := &backend.PodOptions{
		RawVolumes: []atypes.Volume{
			atypes.Volume{
				Name:   atypes.ACName(netNsVolumeName),
				Kind:   "host",
				Source: m.netNsPath,
			},
		},
		StagerMounts: []*configs.Mount{
			&configs.Mount{
				Source:      m.netNsPath,
				Destination: filepath.Join("/volumes", netNsVolumeName),
				Device:      "bind",
				Flags:       syscall.MS_BIND,
			},
		},
		ContainerIO: make(map[string]*backend.IOs),
	}

	return pod, options, nil
}

// The appc/spec doesn't have a method to generate a new isolator live in
// code. You can instantiate a new one, but it its parsed interface version of
// the object is a private field. To get one programmatically and have it be
// usable, then we need to loop it through json.
func generateNewIsolator() (*atypes.Isolator, error) {
	iso := kschema.NewLinuxNamespace()
	niso, ok := iso.(*kschema.LinuxNamespaces)
	if !ok {
		return nil, fmt.Errorf("internal error generating namespace isolator")
	}

	niso.SetIPC(kschema.LinuxNamespaceHost)
	niso.SetNet(kschema.LinuxNamespaceHost)
	niso.SetUser(kschema.LinuxNamespaceHost)
	niso.SetUTS(kschema.LinuxNamespaceHost)
	niso.SetPID(kschema.LinuxNamespaceHost)

	var interim struct {
		Name  string               `json:"name"`
		Value atypes.IsolatorValue `json:"value"`
	}
	interim.Name = kschema.LinuxNamespacesName
	interim.Value = niso

	b, err := json.Marshal(interim)
	if err != nil {
		return nil, err
	}

	var i atypes.Isolator
	if err := i.UnmarshalJSON(b); err != nil {
		return nil, err
	}

	return &i, nil
}

func rawValue(value string) *json.RawMessage {
	msg := json.RawMessage(value)
	return &msg
}
