// Copyright 2015-2016 Apcera Inc. All rights reserved.

package aciremote

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

var (
	// Client is the http.Client that is used by RetrieveImage to download
	// images.
	Client *http.Client = &http.Client{
		Transport: &http.Transport{},
	}
)

func init() {
	discovery.Client.Transport = &http.Transport{}
	discovery.ClientInsecureTLS.Transport = &http.Transport{}
}

// RetrieveImage can be used to retrieve a remote image, and optionally discover
// an image based on the App Container Image Discovery specification. Supports
// handling local images as well as
func RetrieveImage(imageUri string, labels map[types.ACIdentifier]string, insecure bool) (tempfile.ReadSeekCloser, error) {
	u, err := url.Parse(imageUri)
	if err != nil {
		return nil, err
	}

	insecureOption := discovery.InsecureNone
	if insecure {
		insecureOption = discovery.InsecureHTTP
	}

	switch u.Scheme {
	case "file":
		// for file:// urls, just load the file and return it
		filename := u.Path
		if u.Host != "" {
			filename = filepath.Join(u.Host, u.Path)
		}
		return os.Open(filename)

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

	case "docker":
		dockerName := imageUri[9:]

		// create a temp path for the conversion
		tmpdir, err := ioutil.TempDir(os.TempDir(), "docker2aci")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp path to handle Docker image conversion: %v", err)
		}
		defer os.RemoveAll(tmpdir)

		acis, err := docker2aci.ConvertRemoteRepo(dockerName, docker2aci.RemoteConfig{
			CommonConfig: docker2aci.CommonConfig{
				Squash:      true,
				OutputDir:   tmpdir,
				TmpDir:      tmpdir,
				Compression: docker2acicommon.NoCompression,
			},
			Insecure: insecure,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to convert Docker image: %v", err)
		}

		f, err := os.Open(acis[0])
		if err != nil {
			return nil, fmt.Errorf("failed to open converted Docker image: %v", err)
		}
		return f, nil

	case "":
		app, err := discovery.NewAppFromString(imageUri)
		if err != nil {
			return nil, err
		}
		for k, v := range labels {
			app.Labels[k] = v
		}

		endpoints, _, err := discovery.DiscoverACIEndpoints(*app, nil, insecureOption)
		if err != nil {
			return nil, err
		}

		for _, ep := range endpoints {
			r, err := RetrieveImage(ep.ACI, nil, insecure)
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
func LoadImage(imageUri string, insecure bool, imageManager backend.ImageManager) (string, *schema.ImageManifest, error) {
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

	labels := make(map[types.ACIdentifier]string)
	labels[types.ACIdentifier("os")] = runtime.GOOS
	labels[types.ACIdentifier("arch")] = runtime.GOARCH

	f, err := RetrieveImage(imageUri, labels, insecure)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	return imageManager.CreateImage(f)
}
