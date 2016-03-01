// Copyright 2016 Apcera Inc. All rights reserved.
// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"encoding/json"

	"github.com/appc/cni/pkg/types"
)

// NetConf describes a network.
type NetConf struct {
	Name               string          `json:"name,omitempty"`
	ACI                string          `json:"aci,omitempty"`
	ContainerInterface string          `json:"containerInterface,omitempty"`
	RawConfig          json.RawMessage `json:"-"`
}

func (n *NetConf) UnmarshalJSON(data []byte) error {
	nc := netConf{}
	if err := json.Unmarshal(data, &nc); err != nil {
		return err
	}

	n.Name = nc.Name
	n.ACI = nc.ACI
	n.ContainerInterface = nc.ContainerInterface
	n.RawConfig = json.RawMessage(data)
	return nil
}

// netConf is internal and used to process the part of the configuration we
// need, while preserving the plugin configuration in NetConf itself.
type netConf struct {
	Name               string `json:"name,omitempty"`
	ACI                string `json:"aci,omitempty"`
	ContainerInterface string `json:"containerInterface,omitempty"`
}

type IPResult struct {
	Name               string          `json:"name"`
	ContainerInterface string          `json:"containerInterface"`
	IP4                *types.IPConfig `json:"ip4,omitempty"`
	IP6                *types.IPConfig `json:"ip6,omitempty"`
	DNS                *types.DNS      `json:"dns,omitempty"`
}
