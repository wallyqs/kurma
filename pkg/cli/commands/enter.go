// Copyright 2015-2016 Apcera Inc. All rights reserved.

package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/apcera/kurma/pkg/apiclient"
	"github.com/apcera/kurma/pkg/cli"
	"github.com/apcera/kurma/schema"
	"github.com/apcera/util/wsconn"
	"github.com/creack/termios/raw"
	"github.com/spf13/cobra"
)

var (
	EnterCmd = &cobra.Command{
		Use:   "enter UUID APP",
		Short: "Enter a running container",
		Run:   cmdEnter,
	}
)

func init() {
	cli.RootCmd.AddCommand(EnterCmd)
}

func cmdEnter(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Printf("Invalid command options specified.\n")
		cmd.Help()
		return
	}

	// Define the response object and defer the printing of it. This way it is
	// after all other defer logic, so it can safely os.Exit, and it is printing
	// the message after the terminal is reset to normal.
	var response *apiclient.ContainerEnterResponse
	defer func() {
		if response != nil && response.Code != 0 {
			if response.Message != "" {
				fmt.Fprintf(os.Stderr, "%s\n", response.Message)
			}
			os.Exit(response.Code)
		}
	}()

	// Set the local terminal in raw mode to turn off buffering and local
	// echo. Also defers setting it back to normal for when the call is done.
	termios, err := raw.MakeRaw(os.Stdin.Fd())
	if err == nil {
		defer raw.TcSetAttr(os.Stdin.Fd(), termios)
	}

	var app *schema.RunApp
	if len(args) > 2 {
		app = &schema.RunApp{
			WorkingDirectory: "/",
			User:             "0",
			Group:            "0",
			Exec:             args[2:],
			Tty:              true,
		}
	} else {
		app = &schema.RunApp{
			WorkingDirectory: "/",
			User:             "0",
			Group:            "0",
			Tty:              true,
		}
	}

	// Initialize the reader/writer
	conn, err := cli.GetClient().EnterContainer(args[0], args[1], app)
	if err != nil {
		fmt.Printf("Failed to enter the container: %v\n", err)
		os.Exit(1)
	}

	wg := sync.WaitGroup{}

	if wsc, ok := conn.(*wsconn.WebsocketConnection); ok {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range wsc.GetTextChannel() {
				if len(b) == 0 {
					return
				}
				json.Unmarshal(b, &response)
			}
		}()
	}

	go func() {
		io.Copy(conn, os.Stdin)
		conn.Write([]byte{4}) // write EOT
	}()
	io.Copy(os.Stdout, conn)

	conn.Close()
	wg.Wait()
}
