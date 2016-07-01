// Copyright 2015-2016 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"runtime"

	"github.com/apcera/kurma/kurmad"
	"github.com/apcera/logray"
)

const (
	formatString = "%color:class%%year%-%month%-%day% %hour%:%minute%:%second%.%nanosecond% %tzoffset% %tz% [%class% pid=%pid% pod='%field:pod%' source='%sourcefile%:%sourceline%']%color:default% %message%"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "configFile", "kurmad.yml", "Path to the kurma configuration file")
	flag.Parse()

	u := url.URL{
		Scheme: "stdout",
		RawQuery: url.Values(map[string][]string{
			"format": []string{formatString},
		}).Encode(),
	}

	logray.AddDefaultOutput(u.String(), logray.ALL)

	if err := kurmad.Run(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failure running process: %v\n", err)
		os.Exit(1)
	}
	runtime.Goexit()
}
