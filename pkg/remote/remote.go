// Copyright 2015-2016 Apcera Inc. All rights reserved.

package remote

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	"github.com/apcera/util/tempfile"
	"github.com/kurma/remote/aci"
	"github.com/kurma/remote/docker"
	"github.com/kurma/remote/http"

	docker2aci "github.com/appc/docker2aci/lib"
	docker2acicommon "github.com/appc/docker2aci/lib/common"
)

// Fetch retrieves a remote image.
func Fetch(imageURI string, labels map[types.ACIdentifier]string, insecure bool) (tempfile.ReadSeekCloser, error) {
	u, err := url.Parse(imageURI)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "http", "https":
		puller := http.New()

		return puller.Pull(imageURI)
	case "docker":
		puller := docker.New(insecure)

		return puller.Pull(imageURI)
	case "aci", "":
		puller := aci.New(insecure, labels)

		return puller.Pull(imageURI)
	default:
		return nil, fmt.Errorf("%q scheme not supported", u.Scheme)
	}
}
