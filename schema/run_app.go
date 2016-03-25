// Copyright 2016 Apcera Inc. All rights reserved.

package schema

import (
	"github.com/appc/spec/schema/types"
)

type RunApp struct {
	Exec              types.Exec        `json:"exec"`
	User              string            `json:"user"`
	Group             string            `json:"group"`
	SupplementaryGIDs []int             `json:"supplementaryGIDs,omitempty"`
	WorkingDirectory  string            `json:"workingDirectory,omitempty"`
	Environment       types.Environment `json:"environment,omitempty"`
	Tty               bool              `json:"tty,omitempty"`
}
