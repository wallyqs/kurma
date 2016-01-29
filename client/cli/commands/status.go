// Copyright 2015 Apcera Inc. All rights reserved.

package commands

import (
	"fmt"
	"os"

	"github.com/apcera/kurma/client/cli"
	"github.com/apcera/termtables"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var (
	StatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Displays status information about the host.",
		Run:   cmdStatus,
	}
)

func init() {
	cli.RootCmd.AddCommand(StatusCmd)
}

func cmdStatus(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		fmt.Printf("Invalid command options specified.\n")
		os.Exit(1)
	}

	info, err := cli.GetClient().Info()
	if err != nil {
		fmt.Printf("Failed to get host status: %v", err)
		os.Exit(1)
	}

	// create the table
	table := termtables.CreateTable()

	table.AddRow(
		termtables.CreateCell("Hostname", &termtables.CellStyle{Alignment: termtables.AlignRight}),
		info.Hostname)
	table.AddRow(
		termtables.CreateCell("Platform", &termtables.CellStyle{Alignment: termtables.AlignRight}),
		info.Platform)
	table.AddRow(
		termtables.CreateCell("Architecture", &termtables.CellStyle{Alignment: termtables.AlignRight}),
		info.Arch)
	table.AddRow(
		termtables.CreateCell("CPUs", &termtables.CellStyle{Alignment: termtables.AlignRight}),
		info.Cpus)
	table.AddRow(
		termtables.CreateCell("Memory", &termtables.CellStyle{Alignment: termtables.AlignRight}),
		humanize.Bytes(uint64(info.Memory)))

	if info.KernelVersion != "" {
		table.AddRow(
			termtables.CreateCell("Kernel", &termtables.CellStyle{Alignment: termtables.AlignRight}),
			info.KernelVersion)
	}

	fmt.Printf("Host Information\n\n%s", table.Render())
}
