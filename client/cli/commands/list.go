// Copyright 2015 Apcera Inc. All rights reserved.

package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/apcera/kurma/client/cli"
	"github.com/apcera/kurma/stage1/client"
	"github.com/apcera/termtables"
	"github.com/spf13/cobra"
)

var (
	ListCmd = &cobra.Command{
		Use:   "list",
		Short: "List running containers",
		Run:   cmdList,
	}
)

func init() {
	cli.RootCmd.AddCommand(ListCmd)
}

func cmdList(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		fmt.Printf("Invalid command options specified.\n")
		os.Exit(1)
	}

	containers, err := cli.GetClient().ListContainers()
	if err != nil {
		fmt.Printf("Failed to get list of containers: %v", err)
		os.Exit(1)
	}

	// create the table
	table := termtables.CreateTable()

	table.AddHeaders("UUID", "Name", "State")
	sort.Sort(sortedContainers(containers))

	for _, container := range containers {
		var appName string
		for _, app := range container.Pod.Apps {
			appName = app.Name.String()
			break
		}
		table.AddRow(container.UUID, appName, container.State)
	}
	fmt.Printf("%s", table.Render())

}

type sortedContainers []*client.Container

func (a sortedContainers) Len() int      { return len(a) }
func (a sortedContainers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortedContainers) Less(i, j int) bool {
	return a[i].Pod.Apps[0].Name.String() < a[j].Pod.Apps[0].Name.String()
}
