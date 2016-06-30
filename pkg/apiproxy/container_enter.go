// Copyright 2015-2016 Apcera Inc. All rights reserved.

package apiproxy

import (
	"io"
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
	iws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		s.log.Errorf("Failed to upgrade tunnel connection: %v", err)
		http.Error(w, "Failed to setup request", 500)
		return
	}

	// parse the inbound request
	var enterRequest *apiclient.ContainerEnterRequest
	if err := iws.ReadJSON(&enterRequest); err != nil {
		s.log.Errorf("Failed to unmarshal enter request: %v", err)
		http.Error(w, "Failed to upgrade socket", 500)
		return
	}

	// call out
	owsc, err := s.client.EnterContainer(enterRequest.UUID, enterRequest.AppName, &enterRequest.App)
	if err != nil {
		s.log.Errorf("Failed to call to kurma daemon: %v", err)
		http.Error(w, "Failed to upgrade socket", 500)
		return
	}

	// create the websocket connection
	iwsc := wsconn.NewWebsocketConnection(iws)
	defer iwsc.Close()

	// Proxy any text control messages
	if oowsc, ok := owsc.(*wsconn.WebsocketConnection); ok {
		go func() {
			for b := range oowsc.GetTextChannel() {
				if len(b) == 0 {
					return
				}
				iws.WriteMessage(websocket.TextMessage, b)
			}
		}()
	}

	go io.Copy(owsc, iwsc)
	io.Copy(iwsc, owsc)
	owsc.Close()
	iwsc.Close()
}
