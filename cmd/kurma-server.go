// Copyright 2015 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/apcera/kurma/imagestore"
	"github.com/apcera/kurma/stage1/pod"
	"github.com/apcera/kurma/stage1/server"
	"github.com/apcera/logray"
)

func main() {
	var socketfile, parentCgroupName string
	flag.StringVar(&parentCgroupName, "cgroup", "kurma", "Name of the cgroup to create")
	flag.StringVar(&socketfile, "socket", "kurma.sock", "Socket file to create")
	flag.Parse()

	logray.AddDefaultOutput("stdout://", logray.ALL)

	directory, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	iopts := &imagestore.Options{
		Directory: filepath.Join(directory, string("images")),
	}
	imageManager, err := imagestore.New(iopts)
	if err != nil {
		panic(fmt.Sprintf("failed to create the image manager: %v", err))
	}

	mopts := &pod.Options{
		PodDirectory:          filepath.Join(directory, "pods"),
		LibcontainerDirectory: filepath.Join(directory, "libcontainer"),
		VolumeDirectory:       filepath.Join(directory, "volumes"),
		ParentCgroupName:      parentCgroupName,
	}
	podManager, err := pod.NewManager(imageManager, nil, mopts)
	if err != nil {
		panic(fmt.Sprintf("failed to create the pod manager: %v", err))
	}

	socketPermission := os.FileMode(0666)

	opts := &server.Options{
		ImageManager:         imageManager,
		PodManager:           podManager,
		SocketRemoveIfExists: true,
		SocketFile:           socketfile,
		SocketPermissions:    &socketPermission,
	}

	s := server.New(opts)
	if err := s.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failure running process: %v", err)
	}
	runtime.Goexit()
}
