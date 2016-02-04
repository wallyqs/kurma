// Copyright 2015 Apcera Inc. All rights reserved.

package client

import (
	kschema "github.com/apcera/kurma/schema"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

type Container struct {
	UUID  string                `json:"uuid"`
	Image *schema.ImageManifest `json:"image"`
	Pod   *kschema.PodManifest  `json:"pod"`
	State State                 `json:"state"`
}

type Image struct {
	Hash     string                `json:"hash"`
	Manifest *schema.ImageManifest `json:"manifest"`
	Size     int64                 `json:"size"`
}

type ContainerCreateRequest struct {
	Name      string                `json:"name"`
	ImageHash string                `json:"image_hash"`
	Image     *schema.ImageManifest `json:"image"`
}

type ContainerListResponse struct {
	Containers []*Container `json:"containers"`
}

type ContainerResponse struct {
	Container *Container `json:"container"`
}

type ContainerEnterRequest struct {
	UUID    string   `json:"uuid"`
	Command []string `json:"command"`
}

type ImageListResponse struct {
	Images []*Image `json:"images"`
}

type ImageResponse struct {
	Image *Image `json:"image"`
}

type None struct{}

type State string

const (
	STATE_NEW      = State("NEW")
	STATE_STARTING = State("STARTING")
	STATE_RUNNING  = State("RUNNING")
	STATE_STOPPING = State("STOPPING")
	STATE_STOPPED  = State("STOPPED")
	STATE_EXITED   = State("EXITED")
)

type HostInfo struct {
	Hostname      string       `json:"hostname"`
	Cpus          int          `json:"cpus"`
	Memory        int64        `json:"memory"`
	Platform      string       `json:"platform"`
	Arch          string       `json:"arch"`
	ACVersion     types.SemVer `json:"ac_version"`
	KurmaVersion  types.SemVer `json:"kurma_version"`
	KernelVersion string       `json:"kernel_version"`
}
