// Copyright 2016 Apcera Inc. All rights reserved.

package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/apcera/kurma/stage1"
)

const (
	callSetup = "/opt/network/setup"
	callAdd   = "/opt/network/add"
	callDel   = "/opt/network/del"
)

var (
	callTimeout = errors.New("call to driver timed out")
)

// call handles calling into a network plugin with the specific command and
// arguments. It will process any success/response message and return once done
// or timed out.
func (d *networkDriver) call(container stage1.Container, exec string, args []string, val interface{}) error {
	cmdline := append([]string{exec}, args...)

	stdinr, stdinw, err := os.Pipe()
	if err != nil {
		return err
	}
	stdoutr, stdoutw, err := os.Pipe()
	if err != nil {
		return err
	}

	// queue the writing of the config
	var outBytes []byte
	wg := sync.WaitGroup{}
	wg.Add(1)

	process, err := container.Enter(cmdline, stdinr, stdoutw, stdoutw, func() {
		// mark done... note, this is ran in a separate goroutine
		defer wg.Done()

		// close our side of the pipes
		stdinr.Close()
		stdoutw.Close()

		// writie the json configuration
		stdinw.Write(d.config.RawConfig)
		stdinw.Close()

		// read the response
		outBytes, _ = ioutil.ReadAll(stdoutr)
	})
	if err != nil {
		return err
	}

	// queue up the timeout handling and retrival of the exit code
	var exitCode int
	var werr error
	wch := make(chan struct{})

	// begin a goroutine to wait for the process to finish
	go func() {
		defer close(wch)

		ps, err := process.Wait()
		if err != nil {
			werr = err
			return
		}

		// get the exit code
		if status, ok := ps.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}()
	runtime.Gosched()

	// see which returns
	select {
	case <-time.After(60 * time.Second):
		process.Kill()
		return callTimeout
	case <-wch:
	}

	// wait for the goroutines to return
	wg.Wait()

	// check for a wait error
	if werr != nil {
		return werr
	}

	// if exit code wasn't 0, return stderr value as the error
	if exitCode == 0 {
		if val != nil {
			return json.Unmarshal(outBytes, &val)
		}
	} else {
		return fmt.Errorf("exited %d: %s", exitCode, string(outBytes))
	}
	return nil
}
