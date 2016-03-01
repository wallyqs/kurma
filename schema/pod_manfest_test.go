// Copyright 2016 Apcera Inc. All rights reserved.

package schema

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/apcera/kurma/network/types"
	tt "github.com/apcera/util/testtool"
	cnitypes "github.com/appc/cni/pkg/types"
	appcschema "github.com/appc/spec/schema"
	appctypes "github.com/appc/spec/schema/types"
)

var (
	expectedPodManifest = `{"acVersion":"0.7.4","acKind":"PodManifest","apps":[{"name":"container","image":{"id":"sha512-cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"}}],"networks":[{"name":"bridge","containerInterface":"eth0","ip4":{"ip":"192.168.1.2/24","gateway":"192.168.1.1"}}]}`
)

func TestPodManifestMarshal(t *testing.T) {
	pod := BlankPodManifest()
	pod.Apps = appcschema.AppList([]appcschema.RuntimeApp{
		appcschema.RuntimeApp{
			Name: appctypes.ACName("container"),
			Image: appcschema.RuntimeImage{
				ID: *appctypes.NewHashSHA512(nil),
			},
		},
	})
	pod.Networks = []*types.IPResult{
		&types.IPResult{
			Name:               "bridge",
			ContainerInterface: "eth0",
			IP4: &cnitypes.IPConfig{
				Gateway: net.ParseIP("192.168.1.1"),
				IP: net.IPNet{
					IP:   net.ParseIP("192.168.1.2"),
					Mask: net.IPv4Mask(255, 255, 255, 0),
				},
			},
		},
	}

	b, err := json.Marshal(pod)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, string(b), expectedPodManifest)
}

func TestPodManifestUnmarshal(t *testing.T) {
	var pod *PodManifest
	err := json.Unmarshal([]byte(expectedPodManifest), &pod)
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, len(pod.Apps), 1)
	tt.TestEqual(t, pod.Apps[0].Name.String(), "container")
	tt.TestEqual(t, len(pod.Networks), 1)
	network := pod.Networks[0]
	tt.TestEqual(t, network.Name, "bridge")
	tt.TestEqual(t, network.ContainerInterface, "eth0")
	tt.TestEqual(t, network.IP4.IP.IP.String(), "192.168.1.2")
}
