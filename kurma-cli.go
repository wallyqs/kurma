// Copyright 2014-2015 Apcera Inc. All rights reserved.

// +build ignore cli

package main

import (
	"fmt"
	"os"

	"github.com/apcera/kurma/client/cli"

	_ "github.com/apcera/kurma/client/cli/commands"
)

func main() {
	err := cli.RootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute: %v\n", err)
		os.Exit(1)
	}
}
