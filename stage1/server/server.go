// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"net"
	"net/http"
	"os"

	"github.com/apcera/kurma/stage1/container"
	"github.com/apcera/kurma/stage1/image"
	"github.com/apcera/logray"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
)

// Options devices the configuration fields that can be passed to New() when
// instantiating a new Server.
type Options struct {
	ImageManager      *image.Manager
	ContainerManager  *container.Manager
	SocketFile        string
	SocketGroup       *int
	SocketPermissions *os.FileMode
}

// Server represents the process that acts as a daemon to receive container
// management requests.
type Server struct {
	log     *logray.Logger
	options *Options
}

// New creates and returns a new Server object with the provided Options as
// configuration.
func New(options *Options) *Server {
	s := &Server{
		log:     logray.New(),
		options: options,
	}
	return s
}

// Start begins the server. It will return an error if starting the Server
// fails, or return nil on success.
func (s *Server) Start() error {
	l, err := net.Listen("unix", s.options.SocketFile)
	if err != nil {
		return err
	}

	// chmod/chown the socket, if specified
	if s.options.SocketPermissions != nil {
		if err := os.Chmod(s.options.SocketFile, *s.options.SocketPermissions); err != nil {
			return err
		}
	}
	if s.options.SocketGroup != nil {
		if err := os.Chown(s.options.SocketFile, os.Getuid(), *s.options.SocketGroup); err != nil {
			return err
		}
	}
	s.options.ContainerManager.HostSocketFile = s.options.SocketFile

	svr := rpc.NewServer()
	svr.RegisterCodec(json.NewCodec(), "application/json")
	svr.RegisterService(&ContainerService{server: s}, "Containers")
	svr.RegisterService(&ImageService{server: s}, "Images")

	router := mux.NewRouter()
	router.Handle("/rpc", svr)
	router.HandleFunc("/info", s.infoRequest).Methods("GET")
	router.HandleFunc("/containers/enter", s.containerEnterRequest).Methods("GET")
	router.HandleFunc("/images/create", s.imageCreateRequest).Methods("POST")

	s.log.Debug("Server is ready")
	go func() {
		if err := http.Serve(l, router); err != nil {
			s.log.Errorf("Failed ot start HTTP server: %v", err)
		}
	}()
	return nil
}
