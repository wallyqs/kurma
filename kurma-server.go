// Copyright 2015 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"flag"
	"os"
	"runtime"

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

	opts := &server.Options{
		ParentCgroupName:   parentCgroupName,
		ContainerDirectory: directory,
		RequiredNamespaces: []string{"ipc", "mount", "pid", "uts"},
		SocketFile:         socketfile,
	}

	s := server.New(opts)
	if err := s.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failure running process: %v", err)
	}
	runtime.Goexit()
}
