// Copyright 2015-2016 Apcera Inc. All rights reserved.

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/apcera/kurma/pkg/cli"
	"github.com/spf13/cobra"
)

var (
	ShowCmd = &cobra.Command{
		Use:   "show UUID",
		Short: "Show a running pod",
		Run:   cmdShow,
	}
)

func init() {
	cli.RootCmd.AddCommand(ShowCmd)
}

func cmdShow(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Printf("Must specify the UUID of the pod to show.\n")
		cmd.Help()
		return
	}

	pod, err := cli.GetClient().GetPod(args[0])
	if err != nil {
		fmt.Printf("Failed to retrieve pod: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Pod %s:\n\n", pod.UUID)

	// convert back with pretty mode
	b, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal pod: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", string(b))
}
