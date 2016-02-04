// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"io"
	"net/http"

	"github.com/apcera/kurma/stage1/client"
	"github.com/apcera/util/wsconn"
	"github.com/gorilla/websocket"
	"github.com/kr/pty"
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
	var enterRequest *client.ContainerEnterRequest
	if err := ws.ReadJSON(&enterRequest); err != nil {
		s.log.Errorf("Failed to unmarshal enter request: %v", err)
		http.Error(w, "Failed to upgrade socket", 500)
		return
	}

	// get the container
	container := s.options.ContainerManager.Container(enterRequest.UUID)
	if container == nil {
		http.Error(w, "Not Found", 404)
		return
	}

	// create the websocket connection
	wsc := wsconn.NewWebsocketConnection(ws)
	defer wsc.Close()

	// create a pty, which we'll use for the process entering the container and
	// copy the data back up the transport.
	master, slave, err := pty.Open()
	if err != nil {
		s.log.Errorf("Failed to allocate pty: %v", err)
		http.Error(w, "Failed to allocate pty", 500)
		return
	}
	defer func() {
		slave.Close()
		master.Close()
	}()
	go io.Copy(wsc, master)
	go io.Copy(master, wsc)

	// enter into the container
	process, err := container.Enter(enterRequest.Command, slave, slave, slave, nil)
	if err != nil {
		s.log.Errorf("Failed to enter container: %v", err)
		http.Error(w, "Failed to enter container", 500)
		return
	}
	process.Wait()
	s.log.Debugf("Enter request finished")
}
