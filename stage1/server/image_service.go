// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/apcera/kurma/stage1/client"
)

type ImageService struct {
	server *Server
}

func (s *Server) imageCreateRequest(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	hash, manifest, err := s.options.ImageManager.CreateImage(req.Body)
	if err != nil {
		s.log.Errorf("Failed create image: %v", err)
		http.Error(w, "Failed to create image", 500)
		return
	}

	w.WriteHeader(http.StatusCreated)
	resp := &client.ImageResponse{Image: &client.Image{Hash: hash, Manifest: manifest}}
	json.NewEncoder(w).Encode(resp)
}

func (s *ImageService) List(r *http.Request, args *client.None, resp *client.ImageListResponse) error {
	images := s.server.options.ImageManager.ListImages()
	resp.Images = make([]*client.Image, 0, len(images))
	for hash, image := range images {
		imageSize, err := s.server.options.ImageManager.GetImageSize(hash)
		if err != nil {
			s.server.log.Warnf("Failed to get image size %s: %v", hash, err)
			continue
		}
		resp.Images = append(resp.Images, &client.Image{Hash: hash, Manifest: image, Size: imageSize})
	}
	return nil
}

func (s *ImageService) Get(r *http.Request, hash *string, resp *client.ImageResponse) error {
	if hash == nil {
		return fmt.Errorf("no image hash was specified")
	}
	image := s.server.options.ImageManager.GetImage(*hash)
	if image == nil {
		return fmt.Errorf("specified image not found")
	}
	imageSize, err := s.server.options.ImageManager.GetImageSize(*hash)
	if err != nil {
		return err
	}
	resp.Image = &client.Image{Hash: *hash, Manifest: image, Size: imageSize}
	return nil
}

func (s *ImageService) Delete(r *http.Request, hash *string, resp *client.ImageResponse) error {
	if hash == nil {
		return fmt.Errorf("no image hash was specified")
	}
	return s.server.options.ImageManager.DeleteImage(*hash)
}
