// Copyright 2016 Apcera Inc. All rights reserved.

package networkmanager

import (
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

	i, err := kschema.GenerateHostNamespaceIsolator()
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
