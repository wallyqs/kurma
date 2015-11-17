// Copyright 2015 Apcera Inc. All rights reserved.

package init

import (
	"testing"

	tt "github.com/apcera/util/testtool"
)

func TestParseCmdline(t *testing.T) {
	m := parseCmdline("BOOT_IMAGE=/vmlinuz kurma.booted=LABEL=KURMA-A kurma.modules=foo,bar vga=123 loglevel=3")

	tt.TestEqual(t, len(m), 2)
	tt.TestEqual(t, m["kurma.booted"], "LABEL=KURMA-A")
	tt.TestEqual(t, m["kurma.modules"], "foo,bar")
}

func TestProcessCmdline(t *testing.T) {
	m := map[string]string{"kurma.debug": "true", "kurma.modules": "foo,bar", "kurma.booted": "LABEL=KURMA-A"}

	c := processCmdline(m)

	tt.TestNotEqual(t, c, nil)
	tt.TestNotEqual(t, c.SuccessfulBoot, nil)
	tt.TestEqual(t, *c.SuccessfulBoot, "LABEL=KURMA-A")
	tt.TestEqual(t, c.Debug, true)
	tt.TestEqual(t, c.Modules, []string{"foo", "bar"})
}
