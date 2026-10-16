// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package prjcfg

import (
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// dkfModPath is the module path of the Dockerfile module newDkfModule creates.
const dkfModPath = "example.com/dkf"

// newDkfModule creates a Dockerfile module named dkfModPath in a temporary
// directory and writes a go.mod for the project in root requiring it,
// replaced by that directory, so resolving it needs no network. A non-empty
// cfg becomes the module's configs/project.conf. It returns the module
// directory.
func newDkfModule(t tester.T, root, cfg string) string {
	t.Helper()
	dir := t.TempDir()
	oskit.Write(t, "module "+dkfModPath+"\n\ngo 1.26\n", dir, "go.mod")
	if cfg != "" {
		oskit.Write(t, cfg, oskit.MkdirAll(t, dir, "configs"), File)
	}
	mod := "" +
		"module example.com/comp/project\n\n" +
		"go 1.26\n\n" +
		"require " + dkfModPath + " v0.0.0\n\n" +
		"replace " + dkfModPath + " => " + dir + "\n"
	oskit.Write(t, mod, root, "go.mod")
	return dir
}
