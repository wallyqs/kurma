// Copyright 2015-2016 Apcera Inc. All rights reserved.

package apiproxy

import (
	"fmt"
	"net/http"

	"github.com/apcera/kurma/pkg/apiclient"
)

type PodService struct {
	server *Server
}

func (s *PodService) Create(r *http.Request, req *apiclient.PodCreateRequest, resp *apiclient.PodResponse) error {
	// locally validate the manifest to gate remote vs local container functionality
	if err := validatePodManifest(req.Pod); err != nil {
		return fmt.Errorf("image manifest is not valid: %v", err)
	}

	c, err := s.server.client.CreatePod(req)
	if err != nil {
		return err
	}
	resp.Pod = c
	return nil
}

func (s *PodService) List(r *http.Request, args *apiclient.None, resp *apiclient.PodListResponse) error {
	containers, err := s.server.client.ListPods()
	if err != nil {
		return err
	}
	resp.Pods = containers
	return nil
}

func (s *PodService) Get(r *http.Request, uuid *string, resp *apiclient.PodResponse) error {
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

func (s *PodService) Destroy(r *http.Request, uuid *string, ret *apiclient.None) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	return s.server.client.DestroyPod(*uuid)
}
