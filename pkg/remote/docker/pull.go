// Copyright 2015-2016 Apcera Inc. All rights reserved.

package docker

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/apcera/kurma/pkg/remote"

	docker2aci "github.com/appc/docker2aci/lib"
	docker2acicommon "github.com/appc/docker2aci/lib/common"
)

// A dockerPuller represents allows for fetching Docker images to run as Kurma
// containers.
type dockerPuller struct {
	// insecure, if true, will pull a Docker in an insecure manner, skipping
	// signature verification.
	insecure bool

	// convertToACI, if true, will convert Docker images into App Container
	// Images.
	convertToACI bool

	// squashLayers, if true, will squash together all of the layers of the
	// Docker image into a single tarball.
	squashLayers bool
}

// New creates a new dockerPuller to pull a remote Docker image.
func New(insecure bool) remote.Puller {
	return &dockerPuller{
		insecure:     insecure,
		convertToACI: true,
		squashLayers: true,
	}
}

// Pull fetches a remote Docker image.
func (d *dockerPuller) Pull(dockerImageURI string) (io.ReadCloser, error) {
	if d.convertToACI && d.squashLayers {
		return d.pullAsACI(dockerImageURI)
	}
	return nil, errors.New("only ACI-squash-pull currently supported")
}

// pullAsACI fetches a Docker image and converts it into an ACI.
func (d *dockerPuller) pullAsACI(dockerImageURI string) (io.ReadCloser, error) {
	if !d.convertToACI {
		return nil, errors.New("not configured to convert to ACI")
	}
	dockerName := dockerImageURI[9:]

	tmpdir, err := ioutil.TempDir(os.TempDir(), "docker2aci")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp path to handle Docker image conversion: %s", err)
	}
	defer os.RemoveAll(tmpdir)

	acis, err := docker2aci.ConvertRemoteRepo(dockerName, docker2aci.RemoteConfig{
		CommonConfig: docker2aci.CommonConfig{
			Squash:      d.squashLayers,
			OutputDir:   tmpdir,
			TmpDir:      tmpdir,
			Compression: docker2acicommon.NoCompression,
		},
		Insecure: d.insecure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert Docker image: %s", err)
	}

	if d.squashLayers && len(acis) != 1 {
		return nil, fmt.Errorf("fetched %d layer(s), expected 1", len(acis))
	}

	f, err := os.Open(acis[0])
	if err != nil {
		return nil, fmt.Errorf("failed to open converted Docker image: %s", err)
	}
	return f, nil
}
