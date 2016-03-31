// Copyright 2015-2016 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/apcera/kurma/kurmad"
	"github.com/apcera/logray"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "configFile", "kurmad.yaml", "Path to the kurma configuration file")
	flag.Parse()

	logray.AddDefaultOutput("stdout://", logray.ALL)

	if err := kurmad.Run(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failure running process: %v\n", err)
		os.Exit(1)
	}
	runtime.Goexit()
}
