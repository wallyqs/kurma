// Copyright 2016 Apcera Inc. All rights reserved.

package podmanager

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/backend/mocks"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer/configs"

	ntypes "github.com/apcera/kurma/network/types"
	kschema "github.com/apcera/kurma/schema"
	tt "github.com/apcera/util/testtool"
)

func TestHostNetworkNamespaceIsolator(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	pod := createPod(t, manager)

	// Add a namespace isolator to use the host's networking
	pod.manifest = &backend.StagerManifest{
		Pod: &schema.PodManifest{},
	}
	isolatorJson := `[{"name":%q,"value":{"net":"host"}}]`
	err := json.Unmarshal([]byte(fmt.Sprintf(isolatorJson, kschema.LinuxNamespacesName)), &pod.manifest.Pod.Isolators)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, len(pod.manifest.Pod.Isolators), 1)
	tt.TestEqual(t, pod.manifest.Pod.Isolators[0].Name.String(), kschema.LinuxNamespacesName)

	// Run the setup function and validate it
	tt.TestExpectSuccess(t, pod.setupLinuxNamespaceIsolator())
	tt.TestEqual(t, pod.skipNetworking, true)

	// Get the libcontainer config and validate it
	config, err := pod.generateContainerConfig()
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, len(config.Networks), 0)
	tt.TestEqual(t, config.Namespaces.Contains(configs.NEWNET), false)

	// Call the network setup operation and ensure it never called into the
	// network manager. The provision call on the mock network manager should
	// return an error.
	manager.networkManager = &mocks.NetworkManager{
		ProvisionFunc: func(pod backend.Pod) ([]*ntypes.IPResult, error) { return nil, fmt.Errorf("SHOULD NOT BE CALLED") },
	}
	tt.TestExpectSuccess(t, pod.startingNetwork())
}

func TestHostPriviledgeIsolator(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := createManager(t)
	manager.Options.VolumeDirectory = tt.TempDir(t)
	pod := createPod(t, manager)

	// Add an isolator to have host priviledge
	pod.manifest = &backend.StagerManifest{
		Pod: &schema.PodManifest{
			Apps: []schema.RuntimeApp{
				schema.RuntimeApp{
					Name: types.ACName("example"),
					App:  &types.App{},
				},
			},
		},
	}

	isolatorJson := `[{"name":%q,"value":true}]`
	err := json.Unmarshal([]byte(fmt.Sprintf(isolatorJson, kschema.HostPrivilegedName)), &pod.manifest.Pod.Apps[0].App.Isolators)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, len(pod.manifest.Pod.Apps[0].App.Isolators), 1)
	tt.TestEqual(t, pod.manifest.Pod.Apps[0].App.Isolators[0].Name.String(), kschema.HostPrivilegedName)

	// Run the setup functions
	tt.TestExpectSuccess(t, pod.startingBaseDirectories())
	tt.TestExpectSuccess(t, pod.startingApplyIsolators())
	tt.TestExpectSuccess(t, pod.startingInitializeContainer())

	// Validate state
	tt.TestEqual(t, len(pod.hostVolumes), 3)
	tt.TestEqual(t, len(pod.manifest.Pod.Apps[0].Mounts), 3)
	tt.TestEqual(t, len(pod.manifest.Pod.Volumes), 3)

	// Validate the container configuration
	config := pod.stagerContainer.Config()
	tt.TestNotEqual(t, config, nil)
	tt.TestEqual(t, len(config.Mounts) > 3, true, "Should have at least 3 mounts")

	mount := config.Mounts[len(config.Mounts)-3]
	tt.TestEqual(t, mount.Destination, "/volumes/example-host-pods")

	mount = config.Mounts[len(config.Mounts)-2]
	tt.TestEqual(t, mount.Destination, "/volumes/example-host-proc")

	mount = config.Mounts[len(config.Mounts)-1]
	tt.TestEqual(t, mount.Destination, "/volumes/example-host-volumes")

	tt.TestEqual(t, pod.manifest.Pod.Apps[0].Mounts[0].Volume.String(), "example-host-pods")
	tt.TestEqual(t, pod.manifest.Pod.Apps[0].Mounts[0].Path, "/host/pods")

	tt.TestEqual(t, pod.manifest.Pod.Apps[0].Mounts[1].Volume.String(), "example-host-proc")
	tt.TestEqual(t, pod.manifest.Pod.Apps[0].Mounts[1].Path, "/host/proc")

	tt.TestEqual(t, pod.manifest.Pod.Apps[0].Mounts[2].Volume.String(), "example-host-volumes")
	tt.TestEqual(t, pod.manifest.Pod.Apps[0].Mounts[2].Path, "/host/volumes")
}
