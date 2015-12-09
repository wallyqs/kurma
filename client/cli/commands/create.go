// Copyright 2015 Apcera Inc. All rights reserved.

package commands

import (
	"fmt"
	"os"

	"github.com/apcera/kurma/client/cli"
	"github.com/spf13/cobra"
)

var (
	CreateCmd = &cobra.Command{
		Use:   "create FILE",
		Short: "Create a new container",
		Run:   cmdCreate,
	}
)

func init() {
	cli.RootCmd.AddCommand(CreateCmd)
}

func cmdCreate(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 1 {
		fmt.Printf("Invalid command options specified.\n")
		cmd.Help()
		return
	}

	// open the file
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Printf("Failed to open the container image: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// create the image
	image, err := cli.GetClient().CreateImage(f)
	if err != nil {
		fmt.Printf("Failed to create the image: %v\n", err)
		os.Exit(1)
	}

	// create the container
	container, err := cli.GetClient().CreateContainer("", image.Hash, image.Manifest)
	if err != nil {
		fmt.Printf("Failed to launch the new container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Launched container %s\n", container.UUID)
}
