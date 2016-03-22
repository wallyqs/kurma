// Copyright 2016 Apcera Inc. All rights reserved.

package imagestore

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"

	tt "github.com/apcera/util/testtool"
)

func TestNewManager(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	tempdir := tt.TempDir(t)
	opts := &Options{Directory: tempdir}
	st1manager, err := New(opts)
	tt.TestExpectSuccess(t, err)

	manager := st1manager.(*Manager)
	tt.TestNotEqual(t, manager, nil)
}

func createImage(t *testing.T, manifest *schema.ImageManifest) io.Reader {
	// marshal the manifest
	b, err := json.Marshal(manifest)
	tt.TestExpectSuccess(t, err)

	// create a buffer and tar.Writer
	buffer := bytes.NewBufferString("")
	archive := tar.NewWriter(buffer)

	// generate the mock tar
	writeDirectory(t, archive, ".")
	writeFile(t, archive, "./manifest", string(b))
	writeDirectory(t, archive, "./rootfs")
	writeFile(t, archive, "./rootfs/blah", "blah")

	return buffer
}

func TestCreateImage(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	tempdir := tt.TempDir(t)
	opts := &Options{Directory: tempdir}
	manager, err := New(opts)
	tt.TestExpectSuccess(t, err)

	manifest := schema.BlankImageManifest()
	manifest.Name = types.ACIdentifier("example")
	manifest.App = &types.App{
		Exec:  []string{"/bin/blah"},
		User:  "0",
		Group: "0",
		Environment: []types.EnvironmentVariable{
			{
				Name:  "TEST",
				Value: "bar",
			},
		},
	}
	imageReader := createImage(t, manifest)

	hash, retmanifest, err := manager.CreateImage(imageReader)
	tt.TestExpectSuccess(t, err)
	tt.TestNotEqual(t, hash, "")
	tt.TestEqual(t, manifest, retmanifest)
}

func TestCreateImageInvalidImage(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	tempdir := tt.TempDir(t)
	opts := &Options{Directory: tempdir}
	manager, err := New(opts)
	tt.TestExpectSuccess(t, err)

	// create a buffer and tar.Writer
	buffer := bytes.NewBufferString("")
	archive := tar.NewWriter(buffer)

	// generate the mock tar
	writeDirectory(t, archive, ".")
	writeFile(t, archive, "./manifest", "blahh")
	writeDirectory(t, archive, "./rootfs")
	writeFile(t, archive, "./rootfs/blah", "blah")

	hash, retmanifest, err := manager.CreateImage(buffer)
	tt.TestExpectError(t, err)
	tt.TestEqual(t, hash, "")
	tt.TestEqual(t, retmanifest, nil)

	images := manager.ListImages()
	tt.TestEqual(t, len(images), 0)
}

func TestListAndRescanImages(t *testing.T) {
	tempdir := tt.TempDir(t)
	opts := &Options{Directory: tempdir}
	manager, err := New(opts)
	tt.TestExpectSuccess(t, err)

	manifest := schema.BlankImageManifest()
	manifest.Name = types.ACIdentifier("example")
	manifest.App = &types.App{
		Exec:  []string{"/bin/blah"},
		User:  "0",
		Group: "0",
		Environment: []types.EnvironmentVariable{
			{
				Name:  "TEST",
				Value: "bar",
			},
		},
	}
	imageReader := createImage(t, manifest)

	hash, retmanifest, err := manager.CreateImage(imageReader)
	tt.TestExpectSuccess(t, err)
	tt.TestNotEqual(t, hash, "")
	tt.TestEqual(t, manifest, retmanifest)

	images := manager.ListImages()
	tt.TestEqual(t, len(images), 1)
	retmanifest, ok := images[hash]
	tt.TestEqual(t, ok, true, "Images map should have had the returned hash")
	tt.TestEqual(t, manifest, retmanifest)

	err = manager.Rescan()
	tt.TestExpectSuccess(t, err)

	images = manager.ListImages()
	tt.TestEqual(t, len(images), 1)
	retmanifest, ok = images[hash]
	tt.TestEqual(t, ok, true, "Images map should have had the returned hash")
	tt.TestEqual(t, manifest, retmanifest)

	err = manager.DeleteImage(hash)
	tt.TestExpectSuccess(t, err)

	images = manager.ListImages()
	tt.TestEqual(t, len(images), 0)
}

func writeDirectory(t *testing.T, archive *tar.Writer, name string) {
	header := &tar.Header{
		Name:     name + "/",
		Typeflag: tar.TypeDir,
		Mode:     0755,
		ModTime:  time.Now(),
	}
	header.Mode |= tar.TypeDir
	tt.TestExpectSuccess(t, archive.WriteHeader(header))
}

func writeFile(t *testing.T, archive *tar.Writer, name, contents string) {
	b := []byte(contents)
	header := &tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     0644,
		ModTime:  time.Now(),
		Size:     int64(len(b)),
	}
	header.Mode |= tar.TypeReg

	tt.TestExpectSuccess(t, archive.WriteHeader(header))
	_, err := archive.Write(b)
	tt.TestExpectSuccess(t, err)
	tt.TestExpectSuccess(t, archive.Flush())
}
