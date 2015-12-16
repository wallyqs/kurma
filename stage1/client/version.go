// Copyright 2015 Apcera Inc. All rights reserved.

package client

import (
	"github.com/appc/spec/schema/types"
)

var (
	// version is the plain text version string. It will often be set at build
	// time though substitution.
	version string = "0.2.0+git"

	// KurmaVersion is the SemVer representation of version
	KurmaVersion types.SemVer
)

func init() {
	v, err := types.NewSemVer(version)
	if err != nil {
		panic(err)
	}
	KurmaVersion = *v
}
