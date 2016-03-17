// Copyright 2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"testing"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/imagestore"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer"

	tt "github.com/apcera/util/testtool"
)

func createManager(t *testing.T) *Manager {
	iopts := &imagestore.Options{Directory: tt.TempDir(t)}
	imageManager, err := imagestore.New(iopts)
	tt.TestExpectSuccess(t, err)

	opts := &Options{
		ParentCgroupName: "kurma-test",
		PodDirectory:     tt.TempDir(t),
		FactoryFunc:      func(root string) (libcontainer.Factory, error) { return newMockFactory(), nil },
	}

	manager, err := NewManager(imageManager, nil, opts)
	tt.TestExpectSuccess(t, err)
	return manager.(*Manager)
}

func TestNewManager(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	tt.TestNotEqual(t, manager, nil)
}

func TestCreatePod(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)

	manifest := schema.BlankPodManifest()
	manifest.Apps = []schema.RuntimeApp{
		schema.RuntimeApp{
			Name: types.ACName("sample"),
			Image: schema.RuntimeImage{
				ID: *types.NewHashSHA512(nil),
			},
		},
	}

	pod, err := manager.Create("example", manifest, nil)
	tt.TestExpectSuccess(t, err)
	tt.TestNotEqual(t, pod, nil)

	tt.TestNotEqual(t, pod.UUID(), "")
	tt.TestEqual(t, pod.Name(), "example")
	tt.TestEqual(t, pod.PodManifest(), manifest)
	tt.TestEqual(t, pod.State(), backend.STARTING)

	pods := manager.Pods()
	tt.TestEqual(t, len(pods), 1)
	tt.TestEqual(t, pods[0], pod)
	tt.TestEqual(t, manager.Pod(pod.UUID()), pod)

	pod.Stop()

	tt.TestEqual(t, len(manager.Pods()), 0)
}

func TestCreatePodDuplicateName(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)

	manifest := schema.BlankPodManifest()
	manifest.Apps = []schema.RuntimeApp{
		schema.RuntimeApp{
			Name: types.ACName("sample"),
			Image: schema.RuntimeImage{
				ID: *types.NewHashSHA512(nil),
			},
		},
	}

	pod, err := manager.Create("example", manifest, nil)
	tt.TestExpectSuccess(t, err)
	tt.TestNotEqual(t, pod, nil)

	tt.TestNotEqual(t, pod.UUID(), "")
	tt.TestEqual(t, pod.Name(), "example")

	pods := manager.Pods()
	tt.TestEqual(t, len(pods), 1)

	_, err = manager.Create("example", manifest, nil)
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), `a pod with the name "example" already exists`)

	pods = manager.Pods()
	tt.TestEqual(t, len(pods), 1)
}
