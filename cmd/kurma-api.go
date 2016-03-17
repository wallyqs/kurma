// Copyright 2015-2016 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/apcera/kurma/pkg/apiproxy"
	"github.com/apcera/logray"
)

func main() {
	logray.AddDefaultOutput("stdout://", logray.ALL)

	opts := &apiproxy.Options{}

	s := apiproxy.New(opts)
	if err := s.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failure running process: %v", err)
	}
	runtime.Goexit()
}
