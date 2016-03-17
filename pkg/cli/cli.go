// Copyright 2013-2015 Apcera Inc. All rights reserved.

package cli

import (
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/apcera/kurma/pkg/apiclient"
	"github.com/spf13/cobra"
)

const (
	defaultKurmaRemotePort = "12312"
	envKurmaHost           = "KURMA_HOST"
)

var (
	Verbose   bool
	Debug     bool
	KurmaHost string
)

var RootCmd = &cobra.Command{
	Use:   "kurma-cli",
	Short: "kurma-cli",
	Long:  "kurma-cli is the command line client for kurma",
	Run: func(cmd *cobra.Command, args []string) {
		f := cmd.HelpFunc()
		f(cmd, args)
	},
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose output")
	RootCmd.PersistentFlags().BoolVarP(&Debug, "debug", "d", false, "debug output")
	RootCmd.PersistentFlags().StringVarP(&KurmaHost, "host", "H", os.Getenv(envKurmaHost), "kurma host to talk to")
}

func GetClient() apiclient.Client {
	c, err := apiclient.New(determineKurmaHostPort())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v", err)
		os.Exit(1)
	}
	return c
}

func determineKurmaHostPort() string {
	// See if KurmaHost is a socket file
	if fi, _ := os.Stat(KurmaHost); fi != nil {
		u := url.URL{Scheme: "unix", Path: KurmaHost}
		return u.String()
	}

	// quick check if it is referring to the local host
	ip := net.ParseIP(KurmaHost)
	if ip != nil {
		u := url.URL{Scheme: "tcp", Host: net.JoinHostPort(KurmaHost, defaultKurmaRemotePort)}
		return u.String()
	}

	u := url.URL{Scheme: "unix", Path: "/var/lib/kurma/kurma.sock"}
	return u.String()
}
