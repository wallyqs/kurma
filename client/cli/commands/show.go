// Copyright 2015 Apcera Inc. All rights reserved.

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/apcera/kurma/client/cli"
	"github.com/spf13/cobra"
)

var (
	ShowCmd = &cobra.Command{
		Use:   "show UUID",
		Short: "Show a running container",
		Run:   cmdShow,
	}
)

func init() {
	cli.RootCmd.AddCommand(ShowCmd)
}

func cmdShow(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Printf("Must specify the UUID of the container to show.\n")
		cmd.Help()
		return
	}

	container, err := cli.GetClient().GetContainer(args[0])
	if err != nil {
		fmt.Printf("Failed to retrieve container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Container %s:\n\n", container.UUID)

	// convert back with pretty mode
	b, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal container: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", string(b))
}
