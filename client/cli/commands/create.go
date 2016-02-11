// Copyright 2015 Apcera Inc. All rights reserved.

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/apcera/kurma/client/cli"
	"github.com/apcera/kurma/util/aciremote"
	"github.com/apcera/util/tempfile"
	"github.com/appc/spec/schema"
	"github.com/spf13/cobra"
)

var (
	CreateCmd = &cobra.Command{
		Use:   "create FILE",
		Short: "Create a new container",
		Run:   cmdCreate,
	}

	createManifestFile string
	createName         string
)

func init() {
	cli.RootCmd.AddCommand(CreateCmd)
	CreateCmd.Flags().StringVarP(&createName, "name", "n", "", "container's name")
	CreateCmd.Flags().StringVarP(&createManifestFile, "manifest", "", "", "alternative manifest to use")
}

func cmdCreate(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 1 {
		fmt.Printf("Invalid command options specified.\n")
		cmd.Help()
		return
	}

	var f tempfile.ReadSeekCloser

	// if a manifest file is given, then read it and use it as the manifest
	var manifest *schema.ImageManifest
	if createManifestFile != "" {
		manifestFile, err := os.Open(createManifestFile)
		if err != nil {
			fmt.Printf("Failed to open the manifest file: %v\n", err)
			os.Exit(1)
		}
		defer manifestFile.Close()

		if err := json.NewDecoder(manifestFile).Decode(&manifest); err != nil {
			fmt.Printf("Failed to parse the provided manifest: %v\n", err)
			os.Exit(1)
		}
	}

	// open the file
	f, err := os.Open(args[0])
	if err != nil {
		if os.IsNotExist(err) {
			f, err = aciremote.RetrieveImage(args[0], true)
			if err != nil {
				fmt.Printf("Failed to retrieve the container image: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("Failed to open the container image: %v\n", err)
			os.Exit(1)
		}
	}
	defer f.Close()

	// create the image
	image, err := cli.GetClient().CreateImage(f)
	if err != nil {
		fmt.Printf("Failed to create the image: %v\n", err)
		os.Exit(1)
	}

	// use the manifest from the image if none was already loaded
	if manifest == nil {
		manifest = image.Manifest
	}

	// create the container
	container, err := cli.GetClient().CreateContainer(createName, image.Hash, manifest)
	if err != nil {
		fmt.Printf("Failed to launch the new container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Launched container %s\n", container.UUID)
}
