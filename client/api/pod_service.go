// Copyright 2015 Apcera Inc. All rights reserved.

package api

import (
	"fmt"
	"net/http"

	"github.com/apcera/kurma/stage1/client"
)

type PodService struct {
	server *Server
}

func (s *PodService) Create(r *http.Request, req *client.PodCreateRequest, resp *client.PodResponse) error {
	// locally validate the manifest to gate remote vs local container functionality
	if err := validatePodManifest(req.Pod); err != nil {
		return fmt.Errorf("image manifest is not valid: %v", err)
	}

	c, err := s.server.client.CreatePod(req.Name, req.Pod)
	if err != nil {
		return err
	}
	resp.Pod = c
	return nil
}

func (s *PodService) List(r *http.Request, args *client.None, resp *client.PodListResponse) error {
	containers, err := s.server.client.ListPods()
	if err != nil {
		return err
	}
	resp.Pods = containers
	return nil
}

func (s *PodService) Get(r *http.Request, uuid *string, resp *client.PodResponse) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	container, err := s.server.client.GetPod(*uuid)
	if err != nil {
		return err
	}
	resp.Pod = container
	return nil
}

func (s *PodService) Destroy(r *http.Request, uuid *string, ret *client.None) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	return s.server.client.DestroyPod(*uuid)
}
