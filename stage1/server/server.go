// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"net"
	"os"

	pb "github.com/apcera/kurma/stage1/client"
	"github.com/apcera/kurma/stage1/container"
	"github.com/apcera/logray"
	"google.golang.org/grpc"
)

// Options devices the configuration fields that can be passed to New() when
// instantiating a new Server.
type Options struct {
	ParentCgroupName   string
	ContainerDirectory string
	RequiredNamespaces []string
	ContainerManager   *container.Manager
	SocketFile         string
	SocketGroup        *int
	SocketPermissions  *os.FileMode
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
	defer l.Close()

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

	// create the RPC handler
	rpc := &rpcServer{
		log:            s.log.Clone(),
		pendingUploads: make(map[string]*pendingContainer),
	}

	// check if we were given an existing manager
	if s.options.ContainerManager != nil {
		rpc.manager = s.options.ContainerManager
	} else {
		// initialize the container manager
		rpc.manager, err = s.initializeManager()
		if err != nil {
			return err
		}
	}
	rpc.manager.HostSocketFile = s.options.SocketFile

	// create the gRPC server and run
	gs := grpc.NewServer()
	pb.RegisterKurmaServer(gs, rpc)
	s.log.Debug("Server is ready")
	gs.Serve(l)
	return nil
}

// initializeManager creates the stage0 manager object which will handle
// container launching.
func (s *Server) initializeManager() (*container.Manager, error) {
	mopts := &container.Options{
		ParentCgroupName:   s.options.ParentCgroupName,
		ContainerDirectory: s.options.ContainerDirectory,
		RequiredNamespaces: s.options.RequiredNamespaces,
	}

	m, err := container.NewManager(mopts)
	if err != nil {
		return nil, err
	}
	m.Log = s.log.Clone()
	return m, nil
}
