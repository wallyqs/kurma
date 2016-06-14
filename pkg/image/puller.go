// Copyright 2016 Apcera Inc. All rights reserved.

package image

import (
	"io"
)

// A Puller pulls container images.
type Puller interface {
	// Pull fetches an image.
	Pull(uri string) (io.ReadCloser, error)
}
