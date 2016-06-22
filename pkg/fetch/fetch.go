// Copyright 2015-2016 Apcera Inc. All rights reserved.

package fetch

import (
	"fmt"
	"net/url"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/local/file"
	"github.com/apcera/kurma/pkg/remote/aci"
	"github.com/apcera/kurma/pkg/remote/docker"
	"github.com/apcera/kurma/pkg/remote/http"

	"github.com/apcera/util/tempfile"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

// FetchAndLoad retrieves a container image and loads it for use within kurmad.
// TODO: refactor out `labels`, `insecure` opts to a config struct. This can
// live as a method on that struct.
func FetchAndLoad(imageURI string, labels map[types.ACIdentifier]string, insecure bool, imageManager backend.ImageManager) (
	string, *schema.ImageManifest, error) {
	f, err := fetch(imageURI, labels, insecure)
	if err != nil {
		return "", nil, err
	}

	hash, manifest, err := imageManager.CreateImage(f)
	if err != nil {
		return "", nil, err
	}
	return hash, manifest, nil
}

// fetch retrieves a container image. Images may be sourced from the local
// machine, or may be retrieved from a remote server.
func fetch(imageURI string, labels map[types.ACIdentifier]string, insecure bool) (tempfile.ReadSeekCloser, error) {
	u, err := url.Parse(imageURI)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "file":
		r, err := file.Load(imageURI)
		if err != nil {
			return nil, err
		}

		return tempfile.New(r)
	case "http", "https":
		puller := http.New()

		r, err := puller.Pull(imageURI)
		if err != nil {
			return nil, err
		}
		return tempfile.New(r)
	case "docker":
		puller := docker.New(insecure)

		r, err := puller.Pull(imageURI)
		if err != nil {
			return nil, err
		}
		return tempfile.New(r)
	case "aci", "":
		puller := aci.New(insecure, labels)

		r, err := puller.Pull(imageURI)
		if err != nil {
			return nil, err
		}
		return tempfile.New(r)
	default:
		return nil, fmt.Errorf("%q scheme not supported", u.Scheme)
	}
}
