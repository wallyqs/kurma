// Copyright 2015-2016 Apcera Inc. All rights reserved.

package daemon

import (
	"net/http"

	"github.com/apcera/kurma/pkg/apiclient"
	"github.com/apcera/util/wsconn"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (s *Server) containerEnterRequest(w http.ResponseWriter, req *http.Request) {
	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		s.log.Errorf("Failed to upgrade tunnel connection: %v", err)
		http.Error(w, "Failed to setup request", 500)
		return
	}

	// parse the inbound request
	var enterRequest *apiclient.ContainerEnterRequest
	if err := ws.ReadJSON(&enterRequest); err != nil {
		s.log.Errorf("Failed to unmarshal enter request: %v", err)
		http.Error(w, "Failed to upgrade socket", 500)
		return
	}

	// get the container
	container := s.options.PodManager.Pod(enterRequest.UUID)
	if container == nil {
		http.Error(w, "Not Found", 404)
		return
	}

	// create the websocket connection
	wsc := wsconn.NewWebsocketConnection(ws)
	defer wsc.Close()

	// enter into the container
	process, err := container.Enter(enterRequest.AppName, &enterRequest.App, wsc, wsc, wsc, nil)
	if err != nil {
		s.log.Errorf("Failed to enter container: %v", err)
		http.Error(w, "Failed to enter container", 500)
		return
	}
	process.Wait()
	s.log.Debugf("Enter request finished")
}
