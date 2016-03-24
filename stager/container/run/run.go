// Copyright 2016 Apcera Inc. All rights reserved.

package run

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer"
)

func Run() error {
	var app *types.App

	f := os.NewFile(3, "app.json")
	if err := json.NewDecoder(f).Decode(&app); err != nil {
		return err
	}
	f.Close()

	factory, err := libcontainer.New("/containers")
	if err != nil {
		return err
	}

	container, err := factory.Load(os.Args[1])
	if err != nil {
		return err
	}

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
	if err := container.Start(process); err != nil {
		return err
	}
	process.Wait()
	return nil
}
