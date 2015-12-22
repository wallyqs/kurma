// Copyright 2015 Apcera Inc. All rights reserved.

package schema

import (
	"encoding/json"
	"fmt"

	"github.com/appc/spec/schema/types"
)

const (
	LinuxNamespacesName = "os/linux/namespaces"

	nsIPC   = "ipc"
	nsMount = "mount"
	nsNet   = "net"
	nsPID   = "pid"
	nsUser  = "user"
	nsUTS   = "uts"
)

func init() {
	types.AddIsolatorValueConstructor(LinuxNamespacesName, newLinuxNamespace)
}

func newLinuxNamespace() types.IsolatorValue {
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

func (n *LinuxNamespaces) AssertValid() error {
	for k, _ := range n.ns {
		switch k {
		case nsIPC, nsMount, nsNet, nsPID, nsUser, nsUTS:
		default:
			return fmt.Errorf("unrecognized namespace %q", k)
		}
	}
	return nil
}

func (n *LinuxNamespaces) IPC() LinuxNamespaceValue {
	return n.ns[nsIPC]
}

func (n *LinuxNamespaces) Mount() LinuxNamespaceValue {
	return n.ns[nsMount]
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
