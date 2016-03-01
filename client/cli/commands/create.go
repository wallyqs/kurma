// Copyright 2015 Apcera Inc. All rights reserved.

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/apcera/kurma/client/cli"
	"github.com/apcera/kurma/stage1/client"
	"github.com/apcera/kurma/util/aciremote"
	"github.com/apcera/util/tempfile"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/spf13/cobra"
)

var (
	CreateCmd = &cobra.Command{
		Use:   "create FILE",
		Short: "Create a new pod",
		Run:   cmdCreate,
	}

	createManifestFile string
	createName         string
)

func init() {
	cli.RootCmd.AddCommand(CreateCmd)
	CreateCmd.Flags().StringVarP(&createName, "name", "n", "", "pod's name")
	CreateCmd.Flags().StringVarP(&createManifestFile, "manifest", "", "", "specific manifest to use")
}

func createPodFromFile(args []string) (*client.Image, error) {
	var f tempfile.ReadSeekCloser

	// open the file
	f, err := os.Open(args[0])
	if err != nil {
		if os.IsNotExist(err) {
			info, err := cli.GetClient().Info()
			if err != nil {
				fmt.Printf("Failed to retrieve host information: %v\n", err)
				os.Exit(1)
			}
			labels := make(map[types.ACIdentifier]string)
			labels[types.ACIdentifier("os")] = "linux"
			labels[types.ACIdentifier("arch")] = info.Arch

			f, err = aciremote.RetrieveImage(args[0], labels, true)
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

	return image, nil
}

func cmdCreate(cmd *cobra.Command, args []string) {
	// if a manifest file is given, then read it and use it as the manifest
	manifest := schema.BlankPodManifest()
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
	} else {
		image, err := createPodFromFile(args)
		if err != nil {
			fmt.Printf("Failed to handle image: %v\n", err)
			os.Exit(1)
		}

		// handle a blank name
		if createName == "" {
			n, err := convertACIdentifierToACName(image.Manifest.Name)
			if err != nil {
				fmt.Printf("Failed to convert the pod name: %v\n", err)
				os.Exit(1)
			}
			createName = n.String()
		}

		imageID, err := types.NewHash(image.Hash)
		if err != nil {
			fmt.Printf("Failed to parse the image hash: %v\n", err)
			os.Exit(1)
		}

		// create the RuntimeApp
		runtimeApp := schema.RuntimeApp{
			Name: *types.MustACName(createName),
			Image: schema.RuntimeImage{
				ID: *imageID,
			},
		}
		manifest.Apps = append(manifest.Apps, runtimeApp)
	}

	// create the container
	pod, err := cli.GetClient().CreatePod(createName, manifest)
	if err != nil {
		fmt.Printf("Failed to launch the new container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Launched pod %s\n", pod.UUID)
}

func convertACIdentifierToACName(name types.ACIdentifier) (*types.ACName, error) {
	parts := strings.Split(name.String(), "/")
	n, err := types.SanitizeACName(parts[len(parts)-1])
	if err != nil {
		return nil, err
	}
	return types.NewACName(n)
}
