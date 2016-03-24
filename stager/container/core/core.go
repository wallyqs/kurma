// Copyright 2016 Apcera Inc. All rights reserved.

package core

import (
	"runtime"

	"github.com/apcera/logray"
	"github.com/opencontainers/runc/libcontainer"
)

func Run() error {
	logray.AddDefaultOutput("stdout://", logray.ALL)
	cs := &containerSetup{
		log:           logray.New(),
		stagerConfig:  defaultStagerConfig,
		appContainers: make(map[string]libcontainer.Container),
		appProcesses:  make(map[string]*libcontainer.Process),
		appWaitch:     make(map[string]chan struct{}),
	}
	if err := cs.run(); err != nil {
		return err
	}
	runtime.Goexit()
	return nil
}
