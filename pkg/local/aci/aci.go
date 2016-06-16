// Copyright 2015-2016 Apcera Inc. All rights reserved.

package aci

import (
	"net/url"
	"runtime"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/remote/aci"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

// Load is used to retrieve the specified imageURI and load it into the
// Image Manager, returning the hash, manifest, or an error on failure. In the
// case of AppC discovery format, it will check to see if the image already
// exists before retrieving.
func Load(imageURI string, insecure bool, imageManager backend.ImageManager) (string, *schema.ImageManifest, error) {
	u, err := url.Parse(imageURI)
	if err != nil {
		return "", nil, err
	}

	// Currently only supports loading from existing on AppC discovery format.
	switch u.Scheme {
	case "":
		app, err := discovery.NewAppFromString(imageURI)
		if err != nil {
			return "", nil, err
		}

		version := app.Labels[types.ACIdentifier("version")]
		hash, manifest := imageManager.FindImage(app.Name.String(), version)
		if hash != "" {
			return hash, manifest, nil
		}
	}

	labels := make(map[types.ACIdentifier]string)
	labels[types.ACIdentifier("os")] = runtime.GOOS
	labels[types.ACIdentifier("arch")] = runtime.GOARCH

	puller := aci.New(insecure, labels)

	f, err := puller.Pull(imageURI)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	return imageManager.CreateImage(f)
}
