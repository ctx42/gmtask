// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmtest helps gmtask tests set up temporary Go projects.
package gmtest

import (
	"path/filepath"

	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/prjkit"
)

// NewProject creates a temporary directory for a test project. The directory
// basename is [prjkit.ProjDir] and the module path is [prjkit.GoModName]. See
// [NewNamedProject] for the contract of the returned project.
func NewProject(t tester.T, opts ...func(*prjkit.Project)) *prjkit.Project {
	t.Helper()
	return NewNamedProject(t, prjkit.ProjDir, opts...)
}

// NewNamedProject is like [NewProject] but uses name as the directory
// basename. The module path is [prjkit.GoModNameStem]+name; options cannot
// change it. The root directory is created with [prjkit.WithProjectCreate],
// which resolves symbolic links in its path, so opts must not include that
// option again.
//
// The test fails immediately when name is not a single path element or when
// the project cannot be created. The returned project is open and must be
// closed with [prjkit.Project.Close], or a registered cleanup fails the test.
func NewNamedProject(
	t tester.T,
	name string,
	opts ...func(*prjkit.Project),
) *prjkit.Project {

	t.Helper()
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) {
		t.Fatalf("gmtest: invalid project name %q", name)
	}
	root := filepath.Join(t.TempDir(), name)
	opts = append([]func(*prjkit.Project){prjkit.WithProjectCreate}, opts...)
	prj := prjkit.New(t, root, opts...)
	if prj == nil {
		t.Fatal("gmtest: creating test project")
	}
	return prj
}
