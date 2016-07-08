// Copyright 2016 Apcera Inc. All rights reserved.

package core

import (
	"net/url"
	"runtime"

	"github.com/apcera/logray"
	"github.com/opencontainers/runc/libcontainer"
)

const (
	formatString = `{"time":%epoch%,"nsec":%nanosecond%,"level":"%class%","pid":"%pid%","message":%json:message%}`
)

func Run() error {
	u := url.URL{
		Scheme: "stdout",
		RawQuery: url.Values(map[string][]string{
			"format": []string{formatString},
		}).Encode(),
	}

	logray.AddDefaultOutput(u.String(), logray.ALL)
	cs := &containerSetup{
		log:           logray.New(),
		stagerConfig:  defaultStagerConfig,
		appContainers: make(map[string]libcontainer.Container),
		appProcesses:  make(map[string]*libcontainer.Process),
		appWaitch:     make(map[string]chan struct{}),
	}
	if err := cs.run(); err != nil {
		cs.log.Flush()
		return err
	}
	runtime.Goexit()
	return nil
}
