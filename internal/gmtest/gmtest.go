// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmtest helps gmtask tests set up temporary Go projects.
package gmtest

import (
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
)

// NewProject creates a temporary directory for a test project. By default the
// directory basename is "project" and the module path is [prjkit.GoModName].
func NewProject(t tester.T, opts ...func(*prjkit.Project)) *prjkit.Project {
	t.Helper()
	return NewNamedProject(t, "project", opts...)
}

// NewNamedProject creates a temporary directory for a test project using name
// as the directory basename.
func NewNamedProject(
	t tester.T,
	name string,
	opts ...func(*prjkit.Project),
) *prjkit.Project {

	t.Helper()
	dir := oskit.MkdirAll(t, t.TempDir(), name)
	return prjkit.New(t, dir, opts...)
}
