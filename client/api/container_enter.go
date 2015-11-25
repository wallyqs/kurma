// Copyright 2015 Apcera Inc. All rights reserved.

package api

import (
	"io"
	"net/http"

	"github.com/apcera/kurma/stage1/client"
	"github.com/apcera/util/wsconn"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (s *Server) containerEnterRequest(w http.ResponseWriter, req *http.Request) {
	iws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		s.log.Errorf("Failed to upgrade tunnel connection: %v", err)
		http.Error(w, "Failed to setup request", 500)
		return
	}

	// parse the inbound request
	var enterRequest *client.ContainerEnterRequest
	if err := iws.ReadJSON(&enterRequest); err != nil {
		s.log.Errorf("Failed to unmarshal enter request: %v", err)
		http.Error(w, "Failed to upgrade socket", 500)
		return
	}

	// call out
	owsc, err := s.client.EnterContainer(enterRequest.UUID, enterRequest.Command...)
	if err != nil {
		s.log.Errorf("Failed to call to kurma daemon: %v", err)
		http.Error(w, "Failed to upgrade socket", 500)
		return
	}

	// create the websocket connection
	iwsc := wsconn.NewWebsocketConnection(iws)
	defer iwsc.Close()

	go io.Copy(owsc, iwsc)
	io.Copy(iwsc, owsc)
	owsc.Close()
	iwsc.Close()
}
