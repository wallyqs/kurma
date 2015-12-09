// Copyright 2015 Apcera Inc. All rights reserved.

package api

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
	image, err := s.client.CreateImage(req.Body)
	if err != nil {
		s.log.Errorf("Failed create image: %v", err)
		http.Error(w, "Failed to create image", 500)
		return
	}

	w.WriteHeader(http.StatusCreated)
	resp := &client.ImageResponse{Image: image}
	json.NewEncoder(w).Encode(resp)
}

func (s *ImageService) List(r *http.Request, args *client.None, resp *client.ImageListResponse) error {
	images, err := s.server.client.ListImages()
	if err != nil {
		return err
	}
	resp.Images = images
	return nil
}

func (s *ImageService) Get(r *http.Request, hash *string, resp *client.ImageResponse) error {
	if hash == nil {
		return fmt.Errorf("no image hash was specified")
	}
	image, err := s.server.client.GetImage(*hash)
	if err != nil {
		return err
	}
	resp.Image = image
	return nil
}

func (s *ImageService) Delete(r *http.Request, hash *string, resp *client.ImageResponse) error {
	if hash == nil {
		return fmt.Errorf("no image hash was specified")
	}
	return s.server.client.DeleteImage(*hash)
}
