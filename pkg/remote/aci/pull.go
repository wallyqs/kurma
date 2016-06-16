// Copyright 2015-2016 Apcera Inc. All rights reserved.

package aci

import (
	"fmt"
	"io"

	"github.com/apcera/kurma/pkg/image"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
)

func init() {
	discovery.Client.Transport = &http.Transport{}
	discovery.ClientInsecureTLS.Transport = &http.Transport{}
}

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
	}
}

// Pull can be used to retrieve a remote image, and optionally discover
// an image based on the App Container Image Discovery specification.
func (a *aciPuller) Pull(aci string) (io.ReadCloser, error) {
	app, err := discovery.NewAppFromString(aci)
	if err != nil {
		return nil, err
	}
	for k, v := range a.labels {
		app.Labels[k] = v
	}

	insecureOption := discovery.InsecureNone
	if a.insecure {
		insecureOption = discovery.InsecureHTTP
	}

	endpoints, _, err := discovery.DiscoverACIEndpoints(*app, nil, insecureOption)
	if err != nil {
		return nil, err
	}

	for _, ep := range endpoints {
		r, err := a.Pull(ep.ACI)
		if err != nil {
			continue
		}
		// TODO: verify signature of downloaded ACI.
		return r, nil
	}
	return nil, fmt.Errorf("failed to find a valid image for %q", aci)
}
