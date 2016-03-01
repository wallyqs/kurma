// Copyright 2015 Apcera Inc. All rights reserved.

package api

import (
	"net"
	"net/http"

	"github.com/apcera/kurma/stage1/client"
	"github.com/apcera/logray"
	"github.com/gorilla/mux"
	rpc "github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
)

// Options devices the configuration fields that can be passed to New() when
// instantiating a new api.Server.
type Options struct {
	BindAddress string
}

// Server represents the process that acts as a daemon to receive container
// management requests.
type Server struct {
	log     *logray.Logger
	options *Options
	client  client.Client
}

// New creates and returns a new Server object with the provided Options as
// configuration.
func New(options *Options) *Server {
	if options.BindAddress == "" {
		options.BindAddress = ":12312"
	}

	s := &Server{
		log:     logray.New(),
		options: options,
	}
	return s
}

// Start begins the server. It will return an error if starting the Server
// fails, or return nil on success.
func (s *Server) Start() error {
	client, err := client.New("unix:///var/lib/kurma.sock")
	if err != nil {
		return err
	}
	s.client = client

	l, err := net.Listen("tcp", s.options.BindAddress)
	if err != nil {
		return err
	}

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
