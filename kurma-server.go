// Copyright 2015 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"os"

	"github.com/apcera/kurma/stage1/server"
	"github.com/apcera/logray"
)

func main() {
	logray.AddDefaultOutput("stdout://", logray.ALL)

	directory, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	opts := &server.Options{
		ParentCgroupName:   "kurma",
		ContainerDirectory: directory,
		RequiredNamespaces: []string{"ipc", "mount", "pid", "uts"},
	}

	s := server.New(opts)
	if err := s.Start(); err != nil {
		panic(err)
	}
}
