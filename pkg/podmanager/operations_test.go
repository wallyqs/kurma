// Copyright 2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/logray"
	"github.com/apcera/util/uuid"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"

	tt "github.com/apcera/util/testtool"
)

func createPod(t *testing.T, manager *Manager) *Pod {
	return &Pod{
		uuid:    uuid.Variant4().String(),
		name:    uuid.Variant4().String(),
		log:     logray.New(),
		manager: manager,
	}
}

func TestStartingBaseDirectories(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	pod := createPod(t, manager)

	tt.TestExpectSuccess(t, pod.startingBaseDirectories())

	tt.TestNotEqual(t, pod.directory, "")

	_, err := os.Stat(pod.directory)
	tt.TestExpectSuccess(t, err)
	_, err = os.Stat(pod.stagerRootPath())
	tt.TestExpectSuccess(t, err)
	_, err = os.Stat(filepath.Join(pod.stagerRootPath(), "tmp"))
	tt.TestExpectSuccess(t, err)
}

func TestStartingWriteManifest(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	pod := createPod(t, manager)

	pod.stagerPath = tt.TempDir(t)
	pod.manifest = &backend.StagerManifest{
		Pod:          schema.BlankPodManifest(),
		StagerConfig: []byte(`{}`),
	}
	pod.manifest.Pod.Apps = []schema.RuntimeApp{
		schema.RuntimeApp{
			Name: types.ACName("sample"),
			Image: schema.RuntimeImage{
				ID: *types.NewHashSHA512(nil),
			},
		},
	}

	tt.TestExpectSuccess(t, pod.startingBaseDirectories())
	tt.TestExpectSuccess(t, pod.startingWriteManifest())

	_, err := os.Stat(filepath.Join(pod.stagerRootPath(), "manifest"))
	tt.TestExpectSuccess(t, err)
}

func TestStartingInitializeContainer(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	pod := createPod(t, manager)
	pod.stagerPath = tt.TempDir(t)
	pod.manifest = &backend.StagerManifest{
		Pod: schema.BlankPodManifest(),
	}

	factory := manager.factory.(*mockFactory)
	tt.TestEqual(t, len(factory.containers), 0)

	tt.TestExpectSuccess(t, pod.startingInitializeContainer())

	tt.TestEqual(t, len(factory.containers), 1)
}

func TestLaunchStager(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	pod := createPod(t, manager)
	pod.stagerPath = tt.TempDir(t)
	pod.stagerImage = &schema.ImageManifest{
		App: &types.App{
			WorkingDirectory: "/",
			Exec:             []string{"/bin/stager"},
		},
	}
	pod.manifest = &backend.StagerManifest{
		Pod: schema.BlankPodManifest(),
	}

	tt.TestExpectSuccess(t, pod.startingBaseDirectories())
	tt.TestExpectSuccess(t, pod.startingInitializeContainer())

	// Note: this will fail. Need to find a sane way to unit test around
	// libcontainer.Process. There is no way to set its process operations from a
	// mock factory/container implementation.
	err := pod.launchStager()
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "failed to retrieve the pid of the stager process: [7] No process operations: invalid process")
}
