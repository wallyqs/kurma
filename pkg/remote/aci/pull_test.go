// Copyright 2016 Apcera Inc. All rights reserved.

package aci

import (
	"testing"
)

func TestACIPull_ImageNotFound(t *testing.T) {
	puller := New(true, nil)

	imageURI := "aci://fake"

	_, err := puller.Pull(imageURI)
	if err == nil {
		t.Fatal("Expected an error, got none")
	}
}
