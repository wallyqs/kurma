// Copyright 2016 Apcera Inc. All rights reserved.

package networkmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/schema"

	ntypes "github.com/apcera/kurma/pkg/networkmanager/types"
)

const (
	callSetup = "/opt/network/setup"
	callAdd   = "/opt/network/add"
	callDel   = "/opt/network/del"
)

var (
	callTimeout = errors.New("call to driver timed out")
)

// networkDriver captures some of the state information about the configured
// network plugins.
type networkDriver struct {
	config  *ntypes.NetConf
	manager *Manager

	// Store the interfaces this driver has provisioned on pods. This is used so
	// on Provision we can store the generate interface name and look it back up
	// on Deprovision.
	podInterfaces      map[string]string
	podInterfacesMutex sync.RWMutex
}

// generateArgs creates the relevant command line arguments that need to be
// passed to the driver.
func (d *networkDriver) generateArgs(targetPod backend.Pod) []string {
	uuid := targetPod.UUID()

	netNsPath := filepath.Join(netNsContainerPath, uuid)

	d.podInterfacesMutex.RLock()
	iface := d.podInterfaces[uuid]
	d.podInterfacesMutex.RUnlock()

	return []string{netNsPath, uuid, iface}
}

// call handles calling into a network plugin with the specific command and
// arguments. It will process any success/response message and return once done
// or timed out.
func (d *networkDriver) call(exec string, args []string, val interface{}) error {
	app := &schema.RunApp{
		User:  "0",
		Group: "0",
		Exec:  append([]string{exec}, args...),
	}

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

	process, err := d.manager.networkPod.Enter(d.config.Name, app, stdinr, stdoutw, stdoutw, func() {
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
