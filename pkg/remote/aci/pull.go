// Copyright 2015-2016 Apcera Inc. All rights reserved.

package aci

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/util/tempfile"
	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"

	docker2aci "github.com/appc/docker2aci/lib"
	docker2acicommon "github.com/appc/docker2aci/lib/common"
)

// An aciPuller allows for discovering and downloading a new app container
// image.
type aciPuller struct {
	// insecure, if true, will pull the image in an insecure manner, skipping
	// signature verification.
	insecure bool

	// labels are a set of app container labels.
	labels map[types.ACIdentifier]string
}

// New creates a new aciPuller to fetch an App Container Image.
func New(insecure bool, labels map[types.ACIdentifier]string) image.Puller {
	return &aciPuller{
		insecure: insecure,
		labels:   labels,
	}, nil
}

// Pull can be used to retrieve a remote image, and optionally discover
// an image based on the App Container Image Discovery specification.
func (a *aciPuller) Pull(aci string) (tempfile.ReadSeekCloser, error) {
	app, err := discovery.NewAppFromString(aci)
	if err != nil {
		return nil, err
	}
	for k, v := range a.labels {
		app.labels[k] = v
	}

	endpoints, _, err := discovery.DiscoverEndpoints(*app, nil, insecureOption)
	if err != nil {
		return nil, err
	}

	for _, ep := range endpoints.ACIEndpoints {
		r, err := a.Pull(ep.ACI, nil, a.insecure)
		if err != nil {
			continue
		}
		// TODO: verify signature of downloaded ACI.
		return r, nil
	}
	return nil, fmt.Errorf("failed to find a valid image for %q", aci)
}
