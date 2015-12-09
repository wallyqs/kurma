// Copyright 2015 Apcera Inc. All rights reserved.

package client

import (
	"github.com/appc/spec/schema"
)

type Container struct {
	UUID  string                `json:"uuid"`
	Image *schema.ImageManifest `json:"image"`
	Pod   *schema.PodManifest   `json:"pod"`
	State string                `json:"state"`
}

type Image struct {
	Hash     string                `json:"hash"`
	Manifest *schema.ImageManifest `json:"manifest"`
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
