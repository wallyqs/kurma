// Copyright 2016 Apcera Inc. All rights reserved.

package commands

import (
	"fmt"
	"os"

	"github.com/apcera/kurma/client/cli"
	"github.com/apcera/termtables"
	"github.com/spf13/cobra"
)

var (
	ImageCmd = &cobra.Command{
		Use:   "image",
		Short: "Manage images within the system",
	}

	ImageListCmd = &cobra.Command{
		Use:   "list",
		Short: "List images within the system",
		Run:   cmdImageList,
	}

	ImageUploadCmd = &cobra.Command{
		Use:   "upload FILE",
		Short: "Upload an image to the system",
		Run:   cmdImageUpload,
	}
)

func init() {
	cli.RootCmd.AddCommand(ImageCmd)
	ImageCmd.AddCommand(ImageUploadCmd)
	ImageCmd.AddCommand(ImageListCmd)
}

func cmdImageList(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		fmt.Printf("Invalid command options specified.\n")
		os.Exit(1)
	}

	images, err := cli.GetClient().ListImages()
	if err != nil {
		fmt.Printf("Failed to get list of containers: %v", err)
		os.Exit(1)
	}

	// create the table
	table := termtables.CreateTable()

	table.AddHeaders("UUID", "Name")

	for _, image := range images {
		table.AddRow(getShortHash(image.Hash), image.Manifest.Name)
	}
	fmt.Printf("%s", table.Render())
}

func getShortHash(s string) string {
	if len(s) < 20 {
		return s
	}
	return s[:20]
}

func cmdImageUpload(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 1 {
		fmt.Printf("Invalid command options specified.\n")
		cmd.Help()
		return
	}

	// open the file
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Printf("Failed to open the image: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// create the image
	image, err := cli.GetClient().CreateImage(f)
	if err != nil {
		fmt.Printf("Failed to upload the image: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully uploaded image %s\n", image.Manifest.Name)
}
