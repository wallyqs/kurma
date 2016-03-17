// Copyright 2015-2016 Apcera Inc. All rights reserved.

package daemon

import (
	"fmt"
	"net/http"

	"github.com/apcera/kurma/pkg/apiclient"
	"github.com/apcera/kurma/pkg/backend"
)

type PodService struct {
	server *Server
}

func (s *PodService) Create(r *http.Request, req *apiclient.PodCreateRequest, resp *apiclient.PodResponse) error {
	c, err := s.server.options.PodManager.Create(req.Name, req.Pod, nil)
	if err != nil {
		return err
	}
	resp.Pod = exportPod(c)
	return nil
}

func (s *PodService) List(r *http.Request, args *apiclient.None, resp *apiclient.PodListResponse) error {
	cs := s.server.options.PodManager.Pods()
	resp.Pods = make([]*apiclient.Pod, len(cs))
	for i, c := range cs {
		resp.Pods[i] = exportPod(c)
	}
	return nil
}

func (s *PodService) Get(r *http.Request, uuid *string, resp *apiclient.PodResponse) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	c := s.server.options.PodManager.Pod(*uuid)
	if c == nil {
		return fmt.Errorf("specified container was not found")
	}
	resp.Pod = exportPod(c)
	return nil
}

func (s *PodService) Destroy(r *http.Request, uuid *string, ret *apiclient.None) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	pod := s.server.options.PodManager.Pod(*uuid)
	if pod == nil {
		return fmt.Errorf("specified pod was not found")
	}
	return pod.Stop()
}

func exportPod(c backend.Pod) *apiclient.Pod {
	return &apiclient.Pod{
		UUID:  c.UUID(),
		Name:  c.Name(),
		Pod:   c.PodManifest(),
		State: apiclient.State(c.State().String()),
	}
}
