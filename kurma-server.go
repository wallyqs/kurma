// Copyright 2015 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/apcera/kurma/stage1/container"
	"github.com/apcera/kurma/stage1/graphstorage/overlay"
	"github.com/apcera/kurma/stage1/image"
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

	storage, err := overlay.New()
	if err != nil {
		panic(err)
	}
	iopts := &image.Options{
		Directory: filepath.Join(directory, string("images")),
	}
	imageManager, err := image.New(storage, iopts)
	if err != nil {
		panic(fmt.Sprintf("failed to create the image manager: %v", err))
	}

	mopts := &container.Options{
		ContainerDirectory: filepath.Join(directory, "pods"),
		VolumeDirectory:    filepath.Join(directory, "volumes"),
		ParentCgroupName:   parentCgroupName,
	}
	containerManager, err := container.NewManager(imageManager, nil, mopts)
	if err != nil {
		panic(fmt.Sprintf("failed to create the container manager: %v", err))
	}

	opts := &server.Options{
		ImageManager:     imageManager,
		ContainerManager: containerManager,
		SocketFile:       socketfile,
	}

	s := server.New(opts)
	if err := s.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failure running process: %v", err)
	}
	runtime.Goexit()
}
