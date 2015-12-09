// Copyright 2015 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/apcera/kurma/client/api"
	"github.com/apcera/logray"
)

func main() {
	logray.AddDefaultOutput("stdout://", logray.ALL)

	opts := &api.Options{}

	s := api.New(opts)
	if err := s.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failure running process: %v", err)
	}
	runtime.Goexit()
}
