// Copyright 2016 Apcera Inc. All rights reserved.

package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/appc/cni/pkg/ns"
	"github.com/appc/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

func main() {
	runtime.LockOSThread()
	var err error

	parts := strings.Split(os.Args[0], string(os.PathSeparator))
	switch parts[len(parts)-1] {
	case "setup", "del":
		os.Exit(0)
	case "add":
		err = add()
	default:
		fmt.Fprintf(os.Stderr, "Unrecognized command %q\n", parts[len(parts)-1])
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func add() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("invalid command line arguments: %v nspath container-uuid iface", os.Args[0])
	}

	addr, err := netlink.ParseAddr("127.0.0.1/8")
	if err != nil {
		return fmt.Errorf("failed to parse loopback address: %v\n", err)
	}

	err = ns.WithNetNSPath(os.Args[1], false, func(*os.File) error {
		link, err := netlink.LinkByName(os.Args[3])
		if err != nil {
			return fmt.Errorf("failed to locate loopback device: %v", err)
		}

		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("failed to add address: %v", err)
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to set link up: %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	result := types.Result{
		IP4: &types.IPConfig{
			IP: *addr.IPNet,
		},
	}
	result.Print()
	return nil
}
