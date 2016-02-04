// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"fmt"
	"net/http"

	"github.com/apcera/kurma/stage1"
	"github.com/apcera/kurma/stage1/client"
)

type ContainerService struct {
	server *Server
}

func (s *ContainerService) Create(r *http.Request, req *client.ContainerCreateRequest, resp *client.ContainerResponse) error {
	c, err := s.server.options.ContainerManager.Create(req.Name, req.Image, req.ImageHash)
	if err != nil {
		return err
	}
	resp.Container = exportContainer(c)
	return nil
}

func (s *ContainerService) List(r *http.Request, args *client.None, resp *client.ContainerListResponse) error {
	cs := s.server.options.ContainerManager.Containers()
	resp.Containers = make([]*client.Container, len(cs))
	for i, c := range cs {
		resp.Containers[i] = exportContainer(c)
	}
	return nil
}

func (s *ContainerService) Get(r *http.Request, uuid *string, resp *client.ContainerResponse) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	c := s.server.options.ContainerManager.Container(*uuid)
	if c == nil {
		return fmt.Errorf("specified container was not found")
	}
	resp.Container = exportContainer(c)
	return nil
}

func (s *ContainerService) Destroy(r *http.Request, uuid *string, ret *client.None) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	container := s.server.options.ContainerManager.Container(*uuid)
	if container == nil {
		return fmt.Errorf("specified container was not found")
	}
	return container.Stop()
}

func exportContainer(c stage1.Container) *client.Container {
	return &client.Container{
		UUID:  c.UUID(),
		Image: c.ImageManifest(),
		Pod:   c.PodManifest(),
		State: client.State(c.State().String()),
	}
}
