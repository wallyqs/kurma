// Copyright 2016 Apcera Inc. All rights reserved.

package run

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/apcera/kurma/schema"
	"github.com/opencontainers/runc/libcontainer"
)

func Run() error {
	// Read in the app configuration
	var app *schema.RunApp
	f := os.NewFile(3, "app.json")
	if err := json.NewDecoder(f).Decode(&app); err != nil {
		return err
	}
	f.Close()

	// Load the container with libcontainer
	factory, err := libcontainer.New("/containers")
	if err != nil {
		return err
	}
	container, err := factory.Load(os.Args[1])
	if err != nil {
		return err
	}

	// Setup the process
	workingDirectory := app.WorkingDirectory
	if workingDirectory == "" {
		workingDirectory = "/"
	}
	process := &libcontainer.Process{
		Cwd:    workingDirectory,
		User:   app.User,
		Args:   app.Exec,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	for _, env := range app.Environment {
		process.Env = append(process.Env, fmt.Sprintf("%s=%s", env.Name, env.Value))
	}

	// Create a tty for the process if the caller wants it
	if app.Tty {
		console, err := process.NewConsole(os.Getuid())
		if err != nil {
			return err
		}
		go io.Copy(os.Stdout, console)
		go io.Copy(console, os.Stdin)
	}

	// Run it!
	if err := container.Start(process); err != nil {
		return err
	}
	process.Wait()
	return nil
}
