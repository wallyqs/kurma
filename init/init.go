// Copyright 2015 Apcera Inc. All rights reserved.

package init

import (
	"fmt"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/logray"
)

// runner is an object that is used to handle the startup of the full KurmaOS
// system. It will take of the running of the process once init.Run() is
// invoked.
type runner struct {
	config         *kurmaConfig
	log            *logray.Logger
	podManager     backend.PodManager
	imageManager   backend.ImageManager
	networkManager backend.NetworkManager
}

// Run takes over the process and launches KurmaOS.
func Run() error {
	r := &runner{
		config: defaultConfiguration(),
		log:    logray.New(),
	}
	return r.Run()
}

// Run handles executing the bootstrap setup. This prepares the current host
// environment to run and manage pods. It will return an error if any part of
// the setup fails.
func (r *runner) Run() error {
	r.log.Info("Launching KurmaOS\n\n")

	for _, f := range setupFunctions {
		if err := f(r); err != nil {
			r.log.Errorf("ERROR: %v", err)
			return fmt.Errorf("%v: %v", f, err)
		}
	}
	return nil
}
