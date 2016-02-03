// Copyright 2015 Apcera Inc. All rights reserved.

package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/apcera/kurma/client/cli"
	"github.com/creack/termios/raw"
	"github.com/spf13/cobra"
)

var (
	EnterCmd = &cobra.Command{
		Use:   "enter UUID",
		Short: "Enter a running container",
		Run:   cmdEnter,
	}
)

func init() {
	cli.RootCmd.AddCommand(EnterCmd)
}

func cmdEnter(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Printf("Invalid command options specified.\n")
		cmd.Help()
		return
	}

	// Set the local terminal in raw mode to turn off buffering and local
	// echo. Also defers setting it back to normal for when the call is done.
	termios, err := raw.MakeRaw(os.Stdin.Fd())
	if err == nil {
		defer raw.TcSetAttr(os.Stdin.Fd(), termios)
	}

	// Initialize the reader/writer
	conn, err := cli.GetClient().EnterContainer(args[0], args[1:]...)
	if err != nil {
		fmt.Printf("Failed to enter the container: %v\n", err)
		os.Exit(1)
	}

	go func() {
		io.Copy(conn, os.Stdin)
		conn.Write([]byte{4}) // write EOT
	}()
	io.Copy(os.Stdout, conn)
	conn.Close()
}
