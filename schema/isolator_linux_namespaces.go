// Copyright 2015 Apcera Inc. All rights reserved.

package schema

import (
	"encoding/json"
	"fmt"

	"github.com/appc/spec/schema/types"
)

const (
	LinuxNamespacesName = "os/linux/namespaces"

	nsIPC  = "ipc"
	nsNet  = "net"
	nsPID  = "pid"
	nsUser = "user"
	nsUTS  = "uts"
)

func init() {
	types.AddIsolatorValueConstructor(LinuxNamespacesName, NewLinuxNamespace)
}

func NewLinuxNamespace() types.IsolatorValue {
	return &LinuxNamespaces{
		ns: make(map[string]LinuxNamespaceValue),
	}
}

type LinuxNamespaceValue string

const (
	LinuxNamespaceDefault = LinuxNamespaceValue("")
	LinuxNamespaceHost    = LinuxNamespaceValue("host")
)

type LinuxNamespaces struct {
	ns map[string]LinuxNamespaceValue
}

func (n *LinuxNamespaces) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &n.ns); err != nil {
		return err
	}
	return nil
}

func (n *LinuxNamespaces) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.ns)
}

func (n *LinuxNamespaces) AssertValid() error {
	for k, _ := range n.ns {
		switch k {
		case nsIPC, nsNet, nsPID, nsUser, nsUTS:
		default:
			return fmt.Errorf("unrecognized namespace %q", k)
		}
	}
	return nil
}

func (n *LinuxNamespaces) IPC() LinuxNamespaceValue {
	return n.ns[nsIPC]
}

func (n *LinuxNamespaces) Net() LinuxNamespaceValue {
	return n.ns[nsNet]
}

func (n *LinuxNamespaces) PID() LinuxNamespaceValue {
	return n.ns[nsPID]
}

func (n *LinuxNamespaces) User() LinuxNamespaceValue {
	return n.ns[nsUser]
}

func (n *LinuxNamespaces) UTS() LinuxNamespaceValue {
	return n.ns[nsUTS]
}

func (n *LinuxNamespaces) SetIPC(val LinuxNamespaceValue) {
	n.ns[nsIPC] = val
}

func (n *LinuxNamespaces) SetNet(val LinuxNamespaceValue) {
	n.ns[nsNet] = val
}

func (n *LinuxNamespaces) SetPID(val LinuxNamespaceValue) {
	n.ns[nsPID] = val
}

func (n *LinuxNamespaces) SetUser(val LinuxNamespaceValue) {
	n.ns[nsUser] = val
}

func (n *LinuxNamespaces) SetUTS(val LinuxNamespaceValue) {
	n.ns[nsUTS] = val
}

// The appc/spec doesn't have a method to generate a new isolator live in
// code. You can instantiate a new one, but it its parsed interface version of
// the object is a private field. To get one programmatically and have it be
// usable, then we need to loop it through json.
func GenerateHostNamespaceIsolator() (*types.Isolator, error) {
	n := &LinuxNamespaces{
		ns: map[string]LinuxNamespaceValue{
			nsIPC:  LinuxNamespaceHost,
			nsNet:  LinuxNamespaceHost,
			nsUser: LinuxNamespaceHost,
			nsUTS:  LinuxNamespaceHost,
		},
	}

	var interim struct {
		Name  string              `json:"name"`
		Value types.IsolatorValue `json:"value"`
	}
	interim.Name = LinuxNamespacesName
	interim.Value = n

	b, err := json.Marshal(interim)
	if err != nil {
		return nil, err
	}

	var i types.Isolator
	if err := i.UnmarshalJSON(b); err != nil {
		return nil, err
	}

	return &i, nil
}
