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
	dir := oskit.MkdirAll(t, t.TempDir(), "project")
	return prjkit.New(t, dir, opts...)
}
