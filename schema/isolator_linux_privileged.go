// Copyright 2016 Apcera Inc. All rights reserved.

package schema

import (
	"encoding/json"

	"github.com/appc/spec/schema/types"
)

const (
	LinuxPrivilegedName = "os/linux/privileged"
)

func init() {
	types.AddIsolatorValueConstructor(LinuxPrivilegedName, newLinuxPrivileged)
}

func newLinuxPrivileged() types.IsolatorValue {
	n := LinuxPrivileged(false)
	return &n
}

type LinuxPrivileged bool

func (n *LinuxPrivileged) UnmarshalJSON(b []byte) error {
	priv := false
	if err := json.Unmarshal(b, &priv); err != nil {
		return err
	}
	*n = LinuxPrivileged(priv)
	return nil
}

func (n LinuxPrivileged) AssertValid() error {
	return nil
}
