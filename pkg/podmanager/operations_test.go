// Copyright 2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/logray"
	"github.com/apcera/util/uuid"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"

	ntypes "github.com/apcera/kurma/pkg/networkmanager/types"
	tt "github.com/apcera/util/testtool"
	cnitypes "github.com/containernetworking/cni/pkg/types"
)

func createPod(t *testing.T, manager *Manager) *Pod {
	return &Pod{
		uuid:    uuid.Variant4().String(),
		name:    uuid.Variant4().String(),
		log:     logray.New(),
		manager: manager,
		options: &backend.PodOptions{},
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

func TestStartingResolvConf(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	pod := createPod(t, manager)
	pod.stagerPath = tt.TempDir(t)
	pod.manifest = &backend.StagerManifest{
		Pod: schema.BlankPodManifest(),
	}

	// setup some mock networking
	pod.networkResults = []*ntypes.IPResult{
		&ntypes.IPResult{
			DNS: &cnitypes.DNS{
				Search:      []string{"cluster.example.local"},
				Nameservers: []string{"1.2.3.4", "5.6.7.8"},
			},
		},
	}

	tt.TestExpectSuccess(t, pod.startingBaseDirectories())
	tt.TestExpectSuccess(t, pod.startingResolvConf())

	resolvConf, err := ioutil.ReadFile(filepath.Join(pod.stagerRootPath(), "etc", "resolv.conf"))
	tt.TestExpectSuccess(t, err)

	lines := strings.Split(strings.TrimSpace(string(resolvConf)), "\n")
	tt.TestEqual(t, len(lines), 3)
	tt.TestEqual(t, lines[0], "search cluster.example.local")
	tt.TestEqual(t, lines[1], "nameserver 1.2.3.4")
	tt.TestEqual(t, lines[2], "nameserver 5.6.7.8")
}

func TestStartingResolvConf_UsingHosts(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	pod := createPod(t, manager)
	pod.stagerPath = tt.TempDir(t)
	pod.manifest = &backend.StagerManifest{
		Pod: schema.BlankPodManifest(),
	}

	tt.TestExpectSuccess(t, pod.startingBaseDirectories())
	tt.TestExpectSuccess(t, pod.startingResolvConf())

	// read the resolv conf from the host
	hostDNS, err := dnsReadConfig("/etc/resolv.conf")
	tt.TestExpectSuccess(t, err)
	tt.TestNotEqual(t, len(hostDNS.Nameservers), 0, "the host should have >0 nameservers")

	// read the stager's resolv conf
	stagerDNS, err := dnsReadConfig(filepath.Join(pod.stagerRootPath(), "etc", "resolv.conf"))
	tt.TestExpectSuccess(t, err)

	// stager's nameservers should match the host's
	tt.TestEqual(t, hostDNS.Nameservers, stagerDNS.Nameservers)
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
	tt.TestEqual(t, err.Error(), "failed to retrieve the pid of the stager process: invalid process")
}
