// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/apcera/kurma/stage1/client"
	"github.com/appc/spec/schema"
)

func (s *Server) infoRequest(w http.ResponseWriter, req *http.Request) {
	hostInfo := &client.HostInfo{
		Cpus:          runtime.NumCPU(),
		Platform:      runtime.GOOS,
		Arch:          runtime.GOARCH,
		ACVersion:     schema.AppContainerVersion,
		KurmaVersion:  client.KurmaVersion,
		KernelVersion: getKernelVersion(),
	}

	hostname, err := os.Hostname()
	if err != nil {
		s.log.Errorf("Failed to get hostname: %v", err)
		http.Error(w, "Failed to process request", 500)
		return
	}
	hostInfo.Hostname = hostname

	mem, err := totalMemory()
	if err != nil {
		s.log.Errorf("Failed to get calculate memory: %v", err)
		http.Error(w, "Failed to process request", 500)
		return
	}
	hostInfo.Memory = mem

	json.NewEncoder(w).Encode(hostInfo)
}

func totalMemory() (int64, error) {
	meminfo, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	memPattern := regexp.MustCompile("MemTotal.*?([0-9]+)")

	memKb := memPattern.FindStringSubmatch(string(meminfo))[1]
	memBytes, err := strconv.ParseInt(memKb, 10, 64)
	if err != nil {
		return 0, err
	}

	memBytes *= 1024

	return memBytes, nil
}
