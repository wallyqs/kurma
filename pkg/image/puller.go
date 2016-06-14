// Copyright 2016 Apcera Inc. All rights reserved.

package image

import (
	"github.com/apcera/util/tempfile"
)

// A Puller pulls container images.
type Puller interface {
	// Pull fetches an image.
	Pull(uri string) (tempfile.ReadSeekCloser, error)
}
