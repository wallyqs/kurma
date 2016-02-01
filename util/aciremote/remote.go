// Copyright 2015-2016 Apcera Inc. All rights reserved.

package aciremote

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/apcera/kurma/stage1/image"
	"github.com/apcera/util/tempfile"
	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
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

// LoadImage is used to retrieve the specified imageUri and load it into the
// Image Manager, returning the hash, manifest, or an error on failure. In the
// case of AppC discovery format, it will check to see if the image already
// exists before retrieving.
func LoadImage(imageUri string, insecure bool, imageManager *image.Manager) (string, *schema.ImageManifest, error) {
	u, err := url.Parse(imageUri)
	if err != nil {
		return "", nil, err
	}

	// Currently only supports loading from existing on AppC discovery format
	switch u.Scheme {
	case "":
		app, err := discovery.NewAppFromString(imageUri)
		if err != nil {
			return "", nil, err
		}

		version := app.Labels[types.ACIdentifier("version")]
		hash, manifest := imageManager.FindImage(app.Name.String(), version)
		if hash != "" {
			return hash, manifest, nil
		}
	}

	f, err := RetrieveImage(imageUri, insecure)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	return imageManager.CreateImage(f)
}
