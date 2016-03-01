// Copyright 2016 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/apcera/kurma/stager/container"
	"github.com/apcera/logray"
	"github.com/opencontainers/runc/libcontainer"

	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			panic(err)
		}
		panic("--this line should have never been executed, congratulations--")
	}
}

func main() {
	logray.AddDefaultOutput("stdout://", logray.ALL)

	if err := container.Run(); err != nil {
		fmt.Printf("ERR: %v\n", err)
		os.Exit(1)
	}
}
