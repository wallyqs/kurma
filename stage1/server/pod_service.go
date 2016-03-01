// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"fmt"
	"net/http"

	"github.com/apcera/kurma/stage1"
	"github.com/apcera/kurma/stage1/client"
)

type PodService struct {
	server *Server
}

func (s *PodService) Create(r *http.Request, req *client.PodCreateRequest, resp *client.PodResponse) error {
	c, err := s.server.options.PodManager.Create(req.Name, req.Pod)
	if err != nil {
		return err
	}
	resp.Pod = exportPod(c)
	return nil
}

func (s *PodService) List(r *http.Request, args *client.None, resp *client.PodListResponse) error {
	cs := s.server.options.PodManager.Pods()
	resp.Pods = make([]*client.Pod, len(cs))
	for i, c := range cs {
		resp.Pods[i] = exportPod(c)
	}
	return nil
}

func (s *PodService) Get(r *http.Request, uuid *string, resp *client.PodResponse) error {
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

func (s *PodService) Destroy(r *http.Request, uuid *string, ret *client.None) error {
	if uuid == nil {
		return fmt.Errorf("no container UUID was specified")
	}
	pod := s.server.options.PodManager.Pod(*uuid)
	if pod == nil {
		return fmt.Errorf("specified pod was not found")
	}
	return pod.Stop()
}

func exportPod(c stage1.Pod) *client.Pod {
	return &client.Pod{
		UUID:  c.UUID(),
		Name:  c.Name(),
		Pod:   c.PodManifest(),
		State: client.State(c.State().String()),
	}
}
