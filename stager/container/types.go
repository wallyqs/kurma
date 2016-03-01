// Copyright 2016 Apcera Inc. All rights reserved.

package container

type stagerRuntimeState string

const (
	stagerStateSetup    = stagerRuntimeState("setup")
	stagerStateRunning  = stagerRuntimeState("running")
	stagerStateTeardown = stagerRuntimeState("teardown")
)

type stagerConfig struct {
	RequiredNamespaces []string `json:"requiredNamespaces"`
	DefaultNamespaces  []string `json:"defaultNamespaces"`
	GraphStorage       string   `json:"graphStorage"`
}

type stagerState struct {
	Apps  map[string]*stagerAppState `json:"apps"`
	State stagerRuntimeState         `json:"state"`
}

type stagerAppState struct {
	Pid      int  `json:"pid"`
	Exited   bool `json:"exited"`
	ExitCode int  `json:"exitCode"`
}
