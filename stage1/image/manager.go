// Copyright 2015-2016 Apcera Inc. All rights reserved.

package image

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/apcera/kurma/stage1/graphstorage"
	"github.com/apcera/kurma/util/cgroups"
	"github.com/apcera/logray"
	"github.com/apcera/util/hashutil"
	"github.com/apcera/util/tarhelper"
	"github.com/apcera/util/tempfile"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

type GCPolicy int

const (
	GC_NEVER = GCPolicy(iota)
	GC_DURATION
	GC_IMMEDIATE
)

// Options contains settings that are used by the Image Manager and
// Containers running on the host.
type Options struct {
	Directory       string
	DefaultGCPolicy GCPolicy
}

// Manager handles the management of the containers running and available on the
// current host.
type Manager struct {
	Log *logray.Logger

	Options *Options

	images     map[string]*schema.ImageManifest
	imagesLock sync.RWMutex

	provisioner graphstorage.StorageProvisioner
}

// New will create and return a new Manager for managing images.
func New(provisioner graphstorage.StorageProvisioner, options *Options) (*Manager, error) {
	m := &Manager{
		Log:         logray.New(),
		Options:     options,
		provisioner: provisioner,
	}

	// load the list of existing image manifests
	if err := m.Rescan(); err != nil {
		return nil, err
	}

	return m, nil
}

// Rescan will reset the list of current images and reload it from disk.
func (m *Manager) Rescan() error {
	m.imagesLock.Lock()
	m.images = make(map[string]*schema.ImageManifest)
	m.imagesLock.Unlock()

	contents, err := ioutil.ReadDir(m.Options.Directory)
	if err != nil {
		return err
	}

	for _, fi := range contents {
		if !fi.IsDir() {
			continue
		}
		if _, err := m.loadFile(fi); err != nil {
			m.Log.Warnf("Failed to load existing manifest at %s: %v", fi.Name(), err)
			os.RemoveAll(filepath.Join(m.Options.Directory, fi.Name()))
		}
	}

	return nil
}

// CreateImage will process the provided reader to extract the image and make it
// available for containers. It will return the image hash ID, image manifest
// from within the image, or an error on any failures.
func (m *Manager) CreateImage(reader io.Reader) (string, *schema.ImageManifest, error) {
	hr := hashutil.NewSha512(reader)
	f, err := tempfile.New(hr)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	hash := fmt.Sprintf("sha512-%s", hr.Sha512())

	// double check we don't already have it
	m.imagesLock.RLock()
	manifest, exists := m.images[hash]
	m.imagesLock.RUnlock()
	if exists {
		return hash, manifest, nil
	}

	dest := filepath.Join(m.Options.Directory, hash)
	if err := os.Mkdir(dest, os.FileMode(0755)); err != nil {
		return "", nil, err
	}

	successful := false
	defer func() {
		if !successful {
			os.RemoveAll(dest)
		}
	}()

	fi, err := os.Stat(dest)
	if err != nil {
		return "", nil, err
	}

	// untar the file
	tarfile := tarhelper.NewUntar(f, dest)
	tarfile.PreserveOwners = true
	tarfile.PreservePermissions = true
	tarfile.Compression = tarhelper.DETECT
	tarfile.AbsoluteRoot = dest
	if err := tarfile.Extract(); err != nil {
		return "", nil, fmt.Errorf("failed to extract image filesystem: %v", err)
	}

	// load the manifest and return it
	manifest, err = m.loadFile(fi)
	if err != nil {
		return "", nil, err
	}
	successful = true
	return hash, manifest, nil
}

// ListImages returns a map of the image hash to image manifest for all images
// that are available.
func (m *Manager) ListImages() map[string]*schema.ImageManifest {
	m.imagesLock.RLock()
	defer m.imagesLock.RUnlock()
	return m.images
}

// GetImage will return the image manifest for the provided image hash.
func (m *Manager) GetImage(hash string) *schema.ImageManifest {
	m.imagesLock.RLock()
	defer m.imagesLock.RUnlock()
	return m.images[hash]
}

// FindImage will find the image manifest and hash for the specified name and
// version label.
func (m *Manager) FindImage(name, version string) (string, *schema.ImageManifest) {
	m.imagesLock.RLock()
	defer m.imagesLock.RUnlock()

	for hash, manifest := range m.images {
		if manifest.Name.String() != name {
			continue
		}

		v, _ := manifest.Labels.Get("version")
		if v == version || version == "" {
			return hash, manifest
		}
	}

	return "", nil
}

// GetImageSize will return the on disk size of the image.
func (m *Manager) GetImageSize(hash string) (int64, error) {
	path := filepath.Join(m.Options.Directory, hash)
	if _, err := os.Stat(path); err != nil {
		return 0, fmt.Errorf("failed to locate image path: %v", err)
	}

	return (*cgroups.Cgroup)(nil).DiskUsed(path)
}

// DeleteImage will remove the specified image hash from disk.
func (m *Manager) DeleteImage(hash string) error {
	if hash == "" {
		return nil
	}
	m.imagesLock.Lock()
	delete(m.images, hash)
	m.imagesLock.Unlock()
	return os.RemoveAll(filepath.Join(m.Options.Directory, hash))
}

// ProvisionPod will create a PodStorage handler for the specified image hash
// and at the specified destination pod directory. This will include resolving
// all of the dependencies and launch a unionfs mount in a new mount namespace
// in the PodStorage.
func (m *Manager) ProvisionPod(hash, podDirectory string) (graphstorage.PodStorage, error) {
	// resolve the tree
	layers, err := m.processLayers(hash)
	if err != nil {
		return nil, err
	}

	// convert the list of hashes into a list of directories
	for i := range layers {
		layers[i] = filepath.Join(m.Options.Directory, layers[i], "rootfs")
	}

	// generate the pod storage
	return m.provisioner.Create(podDirectory, layers)
}

// loadFile will populate the manager's manifest data with the image directory
// specified by the FileInfo.
func (m *Manager) loadFile(fi os.FileInfo) (*schema.ImageManifest, error) {
	if !fi.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}

	if _, err := types.NewHash(fi.Name()); err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(m.Options.Directory, fi.Name(), "manifest"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var manifest *schema.ImageManifest
	if err := json.NewDecoder(f).Decode(&manifest); err != nil {
		return nil, err
	}

	m.imagesLock.Lock()
	m.images[fi.Name()] = manifest
	m.imagesLock.Unlock()
	return manifest, nil
}
