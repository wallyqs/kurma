// Copyright 2015 Apcera Inc. All rights reserved.

package aciremote

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/apcera/util/tempfile"
	"github.com/appc/spec/discovery"
)

var (
	// Client is the http.Client that is used by RetrieveImage to download
	// images.
	Client *http.Client = &http.Client{
		Transport: &http.Transport{},
	}
)

// RetrieveImage can be used to retrieve a remote image, and optionally discover
// an image based on the App Container Image Discovery specification. Supports
// handling local images as well as
func RetrieveImage(imageUri string, insecure bool) (tempfile.ReadSeekCloser, error) {
	u, err := url.Parse(imageUri)
	if err != nil {
		return nil, err
	}

	insecureOption := discovery.InsecureNone
	if insecure {
		insecureOption = discovery.InsecureHttp
	}

	switch u.Scheme {
	case "file":
		// for file:// urls, just load the file and return it
		return os.Open(u.Path)

	case "http", "https":
		// Handle HTTP retrievals, wrapped with a tempfile that cleans up.
		resp, err := Client.Get(imageUri)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
		default:
			return nil, fmt.Errorf("HTTP %d on retrieving %q", resp.StatusCode, imageUri)
		}

		return tempfile.New(resp.Body)

	case "":
		app, err := discovery.NewAppFromString(imageUri)
		if err != nil {
			return nil, err
		}

		endpoints, _, err := discovery.DiscoverEndpoints(*app, nil, insecureOption)
		if err != nil {
			return nil, err
		}

		for _, ep := range endpoints.ACIEndpoints {
			r, err := RetrieveImage(ep.ACI, insecure)
			if err != nil {
				continue
			}
			// FIXME should also attempt to validate the signature
			return r, nil
		}
		return nil, fmt.Errorf("failed to find a valid image for %q", imageUri)

	default:
		return nil, fmt.Errorf("%q scheme not supported", u.Scheme)
	}
}
