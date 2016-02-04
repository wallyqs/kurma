// Copyright 2016 Apcera Inc. All rights reserved.

package network

import (
	"encoding/json"
	"fmt"

	kschema "github.com/apcera/kurma/schema"
	"github.com/appc/spec/schema"
	atypes "github.com/appc/spec/schema/types"
)

// updateManifestForNamesapces takes an existing image manifest and either
// updates or applies the necessary namespace isolator settings for the
// networking plugin to work.
func updateManifestForNamespaces(imageManifest *schema.ImageManifest) error {
	if imageManifest.App == nil {
		imageManifest.App = &atypes.App{
			User:  "0",
			Group: "0",
		}
	}

	if iso := imageManifest.App.Isolators.GetByName(kschema.LinuxNamespacesName); iso != nil {
		if niso, ok := iso.Value().(*kschema.LinuxNamespaces); ok {
			niso.SetIPC(kschema.LinuxNamespaceHost)
			niso.SetNet(kschema.LinuxNamespaceHost)
			niso.SetPID(kschema.LinuxNamespaceHost)
			niso.SetUser(kschema.LinuxNamespaceHost)
			niso.SetUTS(kschema.LinuxNamespaceHost)
		} else {
			return fmt.Errorf("failed to process the namespace isolator that was found")
		}
	} else {
		i, err := generateNewIsolator()
		if err != nil {
			return err
		}
		if imageManifest.App.Isolators != nil {
			imageManifest.App.Isolators = append(imageManifest.App.Isolators, *i)
		} else {
			imageManifest.App.Isolators = atypes.Isolators([]atypes.Isolator{*i})
		}
	}

	return nil
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
	niso.SetPID(kschema.LinuxNamespaceHost)
	niso.SetUser(kschema.LinuxNamespaceHost)
	niso.SetUTS(kschema.LinuxNamespaceHost)

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
