// Copyright 2016 Apcera Inc. All rights reserved.

package file

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
)

// Load opens a local image file.
func Load(imageURI string) (io.ReadCloser, error) {
	u, err := url.Parse(imageURI)
	if err != nil {
		return nil, err
	}

	filename := u.Path
	if u.Host != "" {
		filename = filepath.Join(u.Host, u.Path)
	}
	return os.Open(filename)
}
