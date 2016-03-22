// Copyright 2016 Apcera Inc. All rights reserved.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/appc/spec/schema/types"
	"github.com/opencontainers/runc/libcontainer"

	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func init() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()

	if len(os.Args) > 1 && os.Args[1] == "init" {
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			panic(err)
		}
		panic("--this line should have never been executed, congratulations--")
	}
}

func main() {
	var app *types.App

	f := os.NewFile(3, "app.json")
	if err := json.NewDecoder(f).Decode(&app); err != nil {
		panic(err)
	}
	f.Close()

	factory, err := libcontainer.New("/containers")
	if err != nil {
		panic(err)
	}

	container, err := factory.Load(os.Args[1])
	if err != nil {
		panic(err)
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
		panic(err)
	}
	process.Wait()
}
