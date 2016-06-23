// Copyright 2015-2016 Apcera Inc. All rights reserved.

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/apcera/kurma/pkg/apiclient"
	"github.com/apcera/kurma/pkg/cli"
	"github.com/apcera/kurma/pkg/image"
	"github.com/apcera/util/tempfile"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/spf13/cobra"

	kschema "github.com/apcera/kurma/schema"
)

var (
	CreateCmd = &cobra.Command{
		Use:   "create FILE",
		Short: "Create a new pod",
		Run:   cmdCreate,
	}

	createManifestFile string
	createName         string
	createNetworks     []string
)

func init() {
	cli.RootCmd.AddCommand(CreateCmd)
	CreateCmd.Flags().StringVarP(&createName, "name", "n", "", "pod's name")
	CreateCmd.Flags().StringVarP(&createManifestFile, "manifest", "", "", "specific manifest to use")
	CreateCmd.Flags().StringSliceVarP(&createNetworks, "net", "", []string{}, "network to attach to the pod")
}

func createPodFromFile(file string) (*apiclient.Image, error) {
	var f tempfile.ReadSeekCloser

	// open the file
	f, err := os.Open(file)
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

			f, err = image.Fetch(file, labels, true)
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
		image, err := createPodFromFile(args[0])
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

		// create the app
		app := image.Manifest.App
		// override the exec command if more than 1 args are given
		if len(args) > 1 {
			app.Exec = args[1:]
		}

		// create the RuntimeApp
		runtimeApp := schema.RuntimeApp{
			Name: *types.MustACName(createName),
			Image: schema.RuntimeImage{
				ID: *imageID,
			},
			App: app,
		}
		manifest.Apps = append(manifest.Apps, runtimeApp)
	}

	// check for host networking to add the isolator on
	hostNetworking := false
	for _, n := range createNetworks {
		if n == "host" {
			hostNetworking = true
			createNetworks = nil
			break
		}
	}
	if hostNetworking {
		if err := setHostNetworking(manifest); err != nil {
			fmt.Printf("Failed to update the pod for host networking: %v\n", err)
			os.Exit(1)
		}
	}

	req := &apiclient.PodCreateRequest{
		Name:     createName,
		Pod:      manifest,
		Networks: createNetworks,
	}

	// create the container
	pod, err := cli.GetClient().CreatePod(req)
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

func setHostNetworking(pod *schema.PodManifest) error {
	for _, i := range pod.Isolators {
		if i.Name.String() != kschema.LinuxNamespacesName {
			continue
		}

		if iso, ok := i.Value().(*kschema.LinuxNamespaces); ok {
			iso.SetNet(kschema.LinuxNamespaceHost)
			return nil
		}
	}

	iso := kschema.NewLinuxNamespace().(*kschema.LinuxNamespaces)
	iso.SetNet(kschema.LinuxNamespaceHost)

	var interim struct {
		Name  string              `json:"name"`
		Value types.IsolatorValue `json:"value"`
	}
	interim.Name = kschema.LinuxNamespacesName
	interim.Value = iso

	b, err := json.Marshal(interim)
	if err != nil {
		return err
	}

	var i types.Isolator
	if err := i.UnmarshalJSON(b); err != nil {
		return err
	}

	pod.Isolators = append(pod.Isolators, i)
	return nil
}
