// Copyright 2016 Apcera Inc. All rights reserved.
// Copyright 2009 The Go Authors. All rights reserved.

package podmanager

import (
	"bufio"
	"net"
	"os"
	"strings"

	"github.com/appc/cni/pkg/types"
)

// See resolv.conf(5) on a Linux machine.
// Based on the same function from Go src/net/dnsconfig_unix.go
func dnsReadConfig(filename string) (*types.DNS, error) {
	conf := &types.DNS{}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && (line[0] == ';' || line[0] == '#') {
			// comment.
			continue
		}
		f := strings.Fields(line)
		if len(f) < 1 {
			continue
		}
		switch f[0] {
		case "nameserver": // add one name server
			if len(f) > 1 && len(conf.Nameservers) < 3 { // small, but the standard limit
				// One more check: make sure server name is
				// just an IP address.  Otherwise we need DNS
				// to look it up.
				if net.ParseIP(f[1]) != nil {
					conf.Nameservers = append(conf.Nameservers, f[1])
				}
			}

		case "domain": // set search path to just this domain
			if len(f) > 1 {
				conf.Domain = f[1]
			}

		case "search": // set search path to given servers
			conf.Search = make([]string, len(f)-1)
			for i := 0; i < len(conf.Search); i++ {
				conf.Search[i] = f[i+1]
			}

		case "options": // magic options
			conf.Options = make([]string, len(f)-1)
			for i := 0; i < len(conf.Options); i++ {
				conf.Options[i] = f[i+1]
			}

		}
	}

	return conf, nil
}
