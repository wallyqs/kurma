// Copyright 2016 Apcera Inc. All rights reserved.

package schema

import (
	"github.com/apcera/kurma/network/types"
	appcschema "github.com/appc/spec/schema"
	appctypes "github.com/appc/spec/schema/types"
)

type PodManifest struct {
	ACVersion   appctypes.SemVer        `json:"acVersion"`
	ACKind      appctypes.ACKind        `json:"acKind"`
	Apps        appcschema.AppList      `json:"apps,omitempty"`
	Volumes     []appctypes.Volume      `json:"volumes,omitempty"`
	Isolators   []appctypes.Isolator    `json:"isolators,omitempty"`
	Annotations appctypes.Annotations   `json:"annotations,omitempty"`
	Ports       []appctypes.ExposedPort `json:"ports,omitempty"`

	// Networks contains the IP information for the networks the pod has been
	// configured on.
	Networks []*types.IPResult `json:"networks,omitempty"`
}

type podManifest PodManifest

func BlankPodManifest() *PodManifest {
	return &PodManifest{ACKind: appcschema.PodManifestKind, ACVersion: appcschema.AppContainerVersion}
}

func (pm *PodManifest) AppcPodManifest() *appcschema.PodManifest {
	return &appcschema.PodManifest{
		ACKind:      pm.ACKind,
		ACVersion:   pm.ACVersion,
		Apps:        pm.Apps,
		Volumes:     pm.Volumes,
		Isolators:   pm.Isolators,
		Annotations: pm.Annotations,
		Ports:       pm.Ports,
	}
}
