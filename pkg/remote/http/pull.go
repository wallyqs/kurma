// Copyright 2015-2016 Apcera Inc. All rights reserved.

package http

import (
	"fmt"
	"io"
	"net/http"

	"github.com/apcera/kurma/pkg/image"
)

// A client represents a client for pulling remote images over HTTP.
type client struct {
	*http.Client
}

// New creates a new HTTP image pull client.
func New() image.Puller {
	return &client{
		Client: http.DefaultClient,
	}
}

// Pull fetches a remote Docker image from the configured registry.
func (c *client) Pull(imageURI string) (io.ReadCloser, error) {
	resp, err := c.Client.Get(imageURI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // TODO: needed?

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("HTTP %d on retrieving %q", resp.StatusCode, imageURI)
	}

	return resp.Body, nil
}
