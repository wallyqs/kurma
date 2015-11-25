// Copyright 2015 Apcera Inc. All rights reserved.

package api

import (
	"fmt"
	"net/http"

	"github.com/apcera/kurma/stage1/client"
)

type ContainerService struct {
	server *Server
}

func (s *ContainerService) Create(r *http.Request, req *client.ContainerCreateRequest, resp *client.ContainerResponse) error {
	// locally validate the manifest to gate remote vs local container functionality
	if err := validateImageManifest(req.Image); err != nil {
		return fmt.Errorf("image manifest is not valid: %v", err)
	}

	c, err := s.server.client.CreateContainer(req.Name, req.ImageHash, req.Image)
	if err != nil {
		return err
	}
	resp.Container = c
	return nil
}

func (s *ContainerService) List(r *http.Request, args *client.None, resp *client.ContainerListResponse) error {
	containers, err := s.server.client.ListContainers()
	if err != nil {
		return err
	}
	resp.Containers = containers
	return nil
}

func (s *ContainerService) Get(r *http.Request, uuid *string, resp *client.ContainerResponse) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	container, err := s.server.client.GetContainer(*uuid)
	if err != nil {
		return err
	}
	resp.Container = container
	return nil
}

func (s *ContainerService) Destroy(r *http.Request, uuid *string, ret *client.None) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	return s.server.client.DestroyContainer(*uuid)
}
