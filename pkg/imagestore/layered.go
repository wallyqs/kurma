// Copyright 2015-2016 Apcera Inc. All rights reserved.

package imagestore

import (
	"fmt"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

type layerProcessor struct {
	manager *Manager
	layers  []string
	refs    map[string]bool
}

// processLayers is used to resolve the layers of the specified image hash down
// to the bottom and return the full ordered set. It also does some validation
// to ensure there are no circular references.
func (m *Manager) processLayers(hash string) ([]string, error) {
	lp := &layerProcessor{
		manager: m,
		layers:  []string{hash},
		refs:    map[string]bool{hash: true},
	}

	// loop up the top level manifest
	manifest := m.GetImage(hash)
	if manifest == nil {
		return nil, fmt.Errorf("unable to locate hash %q", hash)
	}

	// process the dependencies of the starting image
	if err := lp.processDependencies(manifest); err != nil {
		return nil, err
	}
	return lp.layers, nil
}

// processDependencies will look the dependencies on the specified image
// manifest and append the image hashes to the list of layers.
func (lp *layerProcessor) processDependencies(manifest *schema.ImageManifest) error {
	for _, dep := range manifest.Dependencies {
		dephash, depmanifest := lp.findImageDependency(dep)

		// check the image
		if depmanifest == nil {
			version, _ := dep.Labels.Get("version")
			return fmt.Errorf("failed to locate dependent image %s:%s", dep.ImageName.String(), version)
		}

		// Validate that it already hasn't showed up in resolution. If it has, skip it.
		if lp.refs[dephash] {
			continue
		}

		// add it to the layers and walk its dependendencies
		lp.layers = append(lp.layers, dephash)
		lp.refs[dephash] = true
		if err := lp.processDependencies(depmanifest); err != nil {
			return err
		}
	}
	return nil
}

func (lp *layerProcessor) findImageDependency(dep types.Dependency) (string, *schema.ImageManifest) {
	if dep.ImageID != nil {
		dephash := dep.ImageID.String()
		return dephash, lp.manager.GetImage(dephash)
	}

	version, _ := dep.Labels.Get("version")
	return lp.manager.FindImage(dep.ImageName.String(), version)
}
