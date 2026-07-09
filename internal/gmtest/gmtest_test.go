package gmtest

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/prjkit"
)

func Test_NewProject(t *testing.T) {
	t.Run("creates project", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		// --- When ---
		prj := NewProject(tspy)

		// --- Then ---
		assert.Equal(t, "project", filepath.Base(prj.Root()))
		assert.Equal(t, prjkit.GoModName, prj.ImpSpec())

		prj.Close() // Must close to prevent error.
	})

	t.Run("forwards options", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		called := false
		opt := func(*prjkit.Project) { called = true }

		// --- When ---
		prj := NewProject(tspy, opt)

		// --- Then ---
		assert.True(t, called)

		prj.Close() // Must close to prevent error.
	})
}
