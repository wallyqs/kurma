// Copyright 2015-2016 Apcera Inc. All rights reserved.

package fetch

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestFetch_LocalFile(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "localACi")
	if err != nil {
		t.Fatalf("Error creating temp file: %s", err)
	}
	defer f.Close()

	uri := "file://" + f.Name()

	reader, err := Fetch(uri, nil, false)
	if err != nil {
		t.Fatalf("Expected no error retrieving %s; got %s", uri, err)
	}
	reader.Close()
}

func TestFetch_UnsupportedScheme(t *testing.T) {
	uri := "fakescheme://google.com"

	_, err := Fetch(uri, nil, false)
	if err == nil {
		t.Fatalf("Expected error with URI %q, got none", uri)
	}
}
