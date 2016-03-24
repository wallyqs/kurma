// Copyright 2016 Apcera Inc. All rights reserved.

package common

type StagerRuntimeState string

const (
	StagerStateSetup    = StagerRuntimeState("setup")
	StagerStateRunning  = StagerRuntimeState("running")
	StagerStateTeardown = StagerRuntimeState("teardown")
)

type StagerConfig struct {
	RequiredNamespaces []string `json:"requiredNamespaces"`
	DefaultNamespaces  []string `json:"defaultNamespaces"`
	GraphStorage       string   `json:"graphStorage"`
}

type StagerState struct {
	Apps  map[string]*StagerAppState `json:"apps"`
	State StagerRuntimeState         `json:"state"`
}

type StagerAppState struct {
	Pid        int    `json:"pid,omitempty"`
	Exited     bool   `json:"exited"`
	ExitCode   int    `json:"exitCode,omitempty"`
	ExitReason string `json:"exitReason,omitempty"`
}
