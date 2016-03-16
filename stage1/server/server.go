// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"net"
	"net/http"
	"os"

	"github.com/apcera/kurma/stage1"
	"github.com/apcera/logray"
	"github.com/gorilla/mux"
	rpc "github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
)

// Options devices the configuration fields that can be passed to New() when
// instantiating a new Server.
type Options struct {
	ImageManager         stage1.ImageManager
	PodManager           stage1.PodManager
	SocketRemoveIfExists bool
	SocketFile           string
	SocketGroup          *int
	SocketPermissions    *os.FileMode
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
	if s.options.SocketRemoveIfExists {
		if _, err := os.Stat(s.options.SocketFile); err == nil {
			os.Remove(s.options.SocketFile)
		}
	}

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
	s.options.PodManager.SetHostSocketFile(s.options.SocketFile)

	svr := rpc.NewServer()
	svr.RegisterCodec(json2.NewCodec(), "application/json")
	svr.RegisterService(&PodService{server: s}, "Pods")
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
