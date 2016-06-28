// Copyright 2015-2016 Apcera Inc. All rights reserved.

package http

import (
	"fmt"
	"io"
	"net/http"

	"github.com/apcera/kurma/pkg/remote"
)

var (
	// Client is the http.Client that is used by RetrieveImage to download
	// images.
	// TODO: this is only exported/global to support setting a Proxy on the
	// Transport. Instead, we can read the config file on demand here.
	Client *http.Client = &http.Client{
		Transport: &http.Transport{},
	}
)

// A client represents a client for pulling remote images over HTTP.
type client struct {
	*http.Client
}

// New creates a new HTTP image pull client.
func New() remote.Puller {
	return &client{
		Client: Client,
	}
}

// Pull fetches a remote image.
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
