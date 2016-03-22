// Copyright 2016 Apcera Inc. All rights reserved.

package main

import (
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/apcera/util/tarhelper"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/ghodss/yaml"
)

var (
	manifestFile string
	rootDir      string
	version      string
	outputFile   string
)

func init() {
	flag.StringVar(&manifestFile, "manifest", "", "Base manifest yaml file to use")
	flag.StringVar(&rootDir, "root", "", "The root filesystem to load add to the image")
	flag.StringVar(&version, "version", "", "The version label to add to the manifest")
	flag.StringVar(&outputFile, "output", "", "The target file to write to")
}

func main() {
	flag.Parse()

	f, err := os.Open(manifestFile)
	if err != nil {
		panic(err)
	}

	byt, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	f.Close()

	image := schema.BlankImageManifest()
	if err := yaml.Unmarshal(byt, &image); err != nil {
		panic(err)
	}

	// Add the current os/arch labels, and version if it is set
	image.Labels = append(image.Labels,
		types.Label{Name: types.ACIdentifier("os"), Value: runtime.GOOS})
	image.Labels = append(image.Labels,
		types.Label{Name: types.ACIdentifier("arch"), Value: runtime.GOARCH})
	if version != "" {
		image.Labels = append(image.Labels,
			types.Label{Name: types.ACIdentifier("version"), Value: version})
	}

	// Create a tempdir to map everything in to
	tmpdir, err := ioutil.TempDir(os.TempDir(), "build-aci")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpdir)

	if err := os.Chmod(tmpdir, os.FileMode(0755)); err != nil {
		panic(err)
	}
	if err := os.Mkdir(filepath.Join(tmpdir, "rootfs"), os.FileMode(0755)); err != nil {
		panic(err)
	}

	f, err = os.Create(filepath.Join(tmpdir, "manifest"))
	if err != nil {
		panic(err)
	}
	if err := json.NewEncoder(f).Encode(image); err != nil {
		panic(err)
	}
	f.Close()

	if err := copypath(rootDir, filepath.Join(tmpdir, "rootfs")); err != nil {
		panic(err)
	}

	f, err = os.OpenFile(outputFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(0644))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	tar := tarhelper.NewTar(f, tmpdir)
	tar.IncludeOwners = true
	tar.IncludePermissions = true
	tar.Compression = tarhelper.GZIP
	tar.OwnerMappingFunc = func(_ int) (int, error) { return 0, nil }
	tar.GroupMappingFunc = func(_ int) (int, error) { return 0, nil }
	if err := tar.Archive(); err != nil {
		panic(err)
	}
}

func copypath(src, dst string) error {
	// Stream the root over to the new location with tarhelper. This is simpler
	// than walking the directories, copying files, checking for symlinks, etc.
	pr, pw := io.Pipe()
	tar := tarhelper.NewTar(pw, src)
	tar.IncludeOwners = true
	tar.IncludePermissions = true
	// ExcludePaths is stupid, leave off the leading slash.
	tar.ExcludePath(dst[1:] + ".*")
	tar.Compression = tarhelper.NONE
	wg := sync.WaitGroup{}
	wg.Add(1)
	var archiveErr error
	go func() {
		defer wg.Done()
		archiveErr = tar.Archive()
	}()
	untar := tarhelper.NewUntar(pr, dst)
	untar.AbsoluteRoot = dst
	untar.PreserveOwners = true
	untar.PreservePermissions = true
	if err := untar.Extract(); err != nil {
		return err
	}

	// ensure we check that the archive call did not error out
	wg.Wait()
	if archiveErr != nil {
		return archiveErr
	}
	return nil
}
