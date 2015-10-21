// Copyright 2015 Apcera Inc. All rights reserved.

package enter

import (
	"fmt"
	"io"
	"os"

	"github.com/apcera/kurma/client/cli"
	"github.com/creack/termios/raw"

	pb "github.com/apcera/kurma/stage1/client"
	"golang.org/x/net/context"
)

func init() {
	cli.DefineCommand("enter", parseFlags, enter, cliEnter, "FIXME")
}

func parseFlags(cmd *cli.Cmd) {
}

func cliEnter(cmd *cli.Cmd) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("Invalid command options specified.")
	}
	return cmd.Run()
}

func enter(cmd *cli.Cmd) error {
	// Set the local terminal in raw mode to turn off buffering and local
	// echo. Also defers setting it back to normal for when the call is done.
	termios, err := raw.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return err
	}
	defer raw.TcSetAttr(os.Stdin.Fd(), termios)

	// Initialize the reader/writer
	stream, err := cmd.Client.Enter(context.Background())
	if err != nil {
		return err
	}

	// Send the first packet with the stream ID (container ID) and the command we
	// want to execute.
	enterRequest := &pb.EnterRequest{
		StreamId: cmd.Args[0],
		Command:  cmd.Args[1:],
		Bytes:    nil,
	}
	if err := stream.Send(enterRequest); err != nil {
		return err
	}

	w := pb.NewByteStreamWriter(pb.NewEnterRequestBrokerWriter(stream), cmd.Args[0])
	r := pb.NewByteStreamReader(stream, nil)

	go io.Copy(w, os.Stdin)
	io.Copy(os.Stdout, r)
	stream.CloseSend()
	return nil
}
