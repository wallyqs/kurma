// Copyright 2016 Apcera Inc. All rights reserved.

package mocks

import (
	"io"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/appc/spec/schema"
)

type ImageManager struct {
	RescanFunc       func() error
	CreateImageFunc  func(reader io.Reader) (string, *schema.ImageManifest, error)
	ListImagesFunc   func() map[string]*schema.ImageManifest
	GetImageFunc     func(hash string) *schema.ImageManifest
	FindImageFunc    func(name, version string) (string, *schema.ImageManifest)
	GetImageSizeFunc func(hash string) (int64, error)
	DeleteImageFunc  func(hash string) error
	ResolveTreeFunc  func(hash string) (*backend.ResolutionTree, error)
}

func (im *ImageManager) Rescan() error {
	return im.RescanFunc()
}

func (im *ImageManager) CreateImage(reader io.Reader) (string, *schema.ImageManifest, error) {
	return im.CreateImageFunc(reader)
}

func (im *ImageManager) ListImages() map[string]*schema.ImageManifest {
	return im.ListImagesFunc()
}

func (im *ImageManager) GetImage(hash string) *schema.ImageManifest {
	return im.GetImageFunc(hash)
}

func (im *ImageManager) FindImage(name, version string) (string, *schema.ImageManifest) {
	return im.FindImageFunc(name, version)
}

func (im *ImageManager) GetImageSize(hash string) (int64, error) {
	return im.GetImageSizeFunc(hash)
}

func (im *ImageManager) DeleteImage(hash string) error {
	return im.DeleteImageFunc(hash)
}

func (im *ImageManager) ResolveTree(hash string) (*backend.ResolutionTree, error) {
	return im.ResolveTreeFunc(hash)
}
