// Copyright 2015 Apcera Inc. All rights reserved.

package init

import (
	"io/ioutil"
	"strings"
)

func getConfigFromCmdline() *kurmaConfig {
	b, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return nil
	}
	str := strings.Trim(string(b), "\n")
	values := parseCmdline(str)
	return processCmdline(values)
}

func parseCmdline(cmdLine string) map[string]string {
	values := make(map[string]string)

	for _, part := range strings.Split(cmdLine, " ") {
		if !strings.HasPrefix(part, "kurma.") {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		values[kv[0]] = kv[1]
	}
	return values
}

func processCmdline(values map[string]string) *kurmaConfig {
	config := &kurmaConfig{}

	for k, v := range values {
		switch k {
		case "kurma.debug":
			config.Debug = v == "true"
		case "kurma.modules":
			config.Modules = strings.Split(v, ",")
		case "kurma.booted":
			config.SuccessfulBoot = &v
		}
	}

	return config
}
