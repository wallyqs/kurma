// Copyright 2015-2016 Apcera Inc. All rights reserved.

package docker

import (
	"fmt"
	"net/http"

	"github.com/apcera/kurma/pkg/image"
	"github.com/apcera/util/tempfile"
)

// A client represents a client for pulling remote images over HTTP.
type client struct {
	*http.Client
}

// New creates a new HTTP image pull client.
func New() image.Puller {
	return &client{
		Client: http.DefaultClient,
	}, nil
}

// Pull fetches a remote Docker image from the configured registry.
func (c *client) Pull(imageURI string) (tempfile.ReadSeekCloser, error) {
	resp, err := c.client.Get(imageURI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("HTTP %d on retrieving %q", resp.StatusCode, imageURI)
	}

	return tempfile.New(resp.Body)
}
