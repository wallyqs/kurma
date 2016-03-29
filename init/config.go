// Copyright 2015-2016 Apcera Inc. All rights reserved.

package init

import (
	"fmt"

	"github.com/apcera/kurma/kurmad"
	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/networkmanager/types"
	"github.com/appc/spec/schema"
)

type kurmaConfig struct {
	Debug              bool                         `json:"debug,omitempty"`
	SuccessfulBoot     *string                      `json:"-"`
	OEMConfig          *OEMConfig                   `json:"oemConfig"`
	Datasources        []string                     `json:"datasources,omitempty"`
	Hostname           string                       `json:"hostname,omitempty"`
	NetworkConfig      kurmaNetworkConfig           `json:"networkConfig,omitempty"`
	Modules            []string                     `json:"modules,omitmepty"`
	Disks              []*kurmaDiskConfiguration    `json:"disks,omitempty"`
	ParentCgroupName   string                       `json:"parentCgroupName,omitempty"`
	DefaultStagerImage string                       `json:"defaultStagerImage,omitempty"`
	PrefetchImages     []string                     `json:"prefetchImages,omitempty"`
	InitialPods        []*kurmad.InitialPodManifest `json:"initialPods,omitempty"`
	PodNetworks        []*types.NetConf             `json:"podNetworks,omitempty"`
	Console            kurmaConsoleService          `json:"console,omitempty"`
}

type OEMConfig struct {
	Device     string `json:"device"`
	ConfigPath string `json:"configPath"`
}

type kurmaNetworkConfig struct {
	DNS        []string                 `json:"dns,omitempty"`
	Gateway    string                   `json:"gateway,omitempty"`
	Interfaces []*kurmaNetworkInterface `json:"interfaces,omitempty"`
	ProxyURL   string                   `json:"proxyUrl,omitempty"`
}

type kurmaNetworkInterface struct {
	Device    string   `json:"device"`
	DHCP      bool     `json:"dhcp,omitmepty"`
	Address   string   `json:"address,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	MTU       int      `json:"mtu,omitmepty"`
}

type kurmaDiskConfiguration struct {
	Device  string           `json:"device"`
	FsType  string           `json:"fstype,omitempty"`
	Options string           `json:"options,omitempty"`
	Format  *bool            `json:"format,omitempty"`
	Usage   []kurmaPathUsage `json:"usage"`
	Resize  bool             `json:"resize"`
}

type kurmaPathUsage string

const (
	kurmaPathImages  = kurmaPathUsage("images")
	kurmaPathPods    = kurmaPathUsage("pods")
	kurmaPathVolumes = kurmaPathUsage("volumes")

	kurmaPath      = "/var/kurma"
	mountPath      = "/mnt"
	systemPodsPath = "/var/kurma/system"
)

type kurmaConsoleService struct {
	Enabled     *bool                      `json:"enabled,omitempty"`
	ACI         *kurmad.InitialPodManifest `json:"aci,omitempty"`
	PodManifest *kurmad.InitialPodManifest `json:"podManifest,omitempty"`
	Password    *string                    `json:"password,omitmepty"`
	SSHKeys     []string                   `json:"sshKeys,omitempty"`
}

func (s *kurmaConsoleService) Process(imageManager backend.ImageManager) (string, *schema.PodManifest, error) {
	if s.ACI != nil && s.PodManifest != nil {
		return "", nil, fmt.Errorf(`both "aci" and "podManifest" cannot be set at the same time`)
	}
	if s.ACI == nil && s.PodManifest == nil {
		return "", nil, fmt.Errorf(`must set either "aci" or "podManifest"`)
	}
	if s.ACI != nil {
		return s.ACI.Process(imageManager)
	}
	return s.PodManifest.Process(imageManager)
}

func (cfg *kurmaConfig) mergeConfig(o *kurmaConfig) {
	if o == nil {
		return
	}

	if o.SuccessfulBoot != nil {
		cfg.SuccessfulBoot = o.SuccessfulBoot
	}

	// FIXME datasources

	// oem config
	if o.OEMConfig != nil {
		cfg.OEMConfig = o.OEMConfig
	}

	// replace hostname
	if o.Hostname != "" {
		cfg.Hostname = o.Hostname
	}

	// replace dns
	if len(o.NetworkConfig.DNS) > 0 {
		cfg.NetworkConfig.DNS = o.NetworkConfig.DNS
	}
	// replace gateway
	if o.NetworkConfig.Gateway != "" {
		cfg.NetworkConfig.Gateway = o.NetworkConfig.Gateway
	}
	// replace interfaces
	if len(o.NetworkConfig.Interfaces) > 0 {
		cfg.NetworkConfig.Interfaces = o.NetworkConfig.Interfaces
	}

	// append modules
	if len(o.Modules) > 0 {
		cfg.Modules = append(cfg.Modules, o.Modules...)
	}

	// replace disks
	if len(o.Disks) > 0 {
		cfg.Disks = o.Disks
	}

	if o.ParentCgroupName != "" {
		cfg.ParentCgroupName = o.ParentCgroupName
	}
	if o.DefaultStagerImage != "" {
		cfg.DefaultStagerImage = o.DefaultStagerImage
	}

	// append init pods
	if len(o.InitialPods) > 0 {
		cfg.InitialPods = append(cfg.InitialPods, o.InitialPods...)
	}

	// Console
	if o.Console.Enabled != nil {
		cfg.Console.Enabled = o.Console.Enabled
	}
	if o.Console.ACI != nil {
		cfg.Console.ACI = o.Console.ACI
		cfg.Console.PodManifest = nil
	} else if o.Console.PodManifest != nil {
		cfg.Console.PodManifest = o.Console.PodManifest
		cfg.Console.ACI = nil
	}
	if o.Console.Password != nil {
		cfg.Console.Password = o.Console.Password
	}
	if len(o.Console.SSHKeys) > 0 {
		cfg.Console.SSHKeys = o.Console.SSHKeys
	}

	// pod networks
	if len(o.PodNetworks) > 0 {
		cfg.PodNetworks = append(cfg.PodNetworks, o.PodNetworks...)
	}
}
