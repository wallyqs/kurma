// Copyright 2015 Apcera Inc. All rights reserved.

package schema

import (
	"encoding/json"

	"github.com/appc/spec/schema/types"
)

const (
	HostApiAccessName = "host/api-access"
)

func init() {
	types.AddIsolatorValueConstructor(HostApiAccessName, newHostApiAccess)
}

func newHostApiAccess() types.IsolatorValue {
	n := HostApiAccess(false)
	return &n
}

type HostApiAccess bool

func (n *HostApiAccess) UnmarshalJSON(b []byte) error {
	priv := false
	if err := json.Unmarshal(b, &priv); err != nil {
		return err
	}
	*n = HostApiAccess(priv)
	return nil
}

func (n HostApiAccess) AssertValid() error {
	return nil
}
