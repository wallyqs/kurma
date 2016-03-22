// Copyright 2015 Apcera Inc. All rights reserved.

package commands

import (
	"fmt"

	"github.com/apcera/kurma/pkg/apiclient"
	"github.com/apcera/kurma/pkg/cli"
	"github.com/appc/spec/schema"
	"github.com/spf13/cobra"
)

var (
	VersionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run:   cmdVersion,
	}
)

func init() {
	cli.RootCmd.AddCommand(VersionCmd)
}

func cmdVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("Client Version: %v\n", apiclient.KurmaVersion)
	fmt.Printf("Client AppC Version: %v\n", schema.AppContainerVersion)

	client := cli.GetClient()
	hostInfo, err := client.Info()
	if err == nil {
		fmt.Println()
		fmt.Printf("Server Version: %v\n", hostInfo.KurmaVersion)
		fmt.Printf("Server AppC Version: %v\n", hostInfo.ACVersion)
	}
}
