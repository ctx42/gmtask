// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

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
		have := NewProject(tspy)

		// --- Then ---
		assert.DirExist(t, have.Root())
		assert.Equal(t, prjkit.ProjDir, filepath.Base(have.Root()))
		assert.Equal(t, prjkit.GoModName, have.ImpSpec())

		have.Close() // Must close to prevent error.
	})

	t.Run("forwards options", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		var seen *prjkit.Project
		opt := func(prj *prjkit.Project) { seen = prj }

		// --- When ---
		have := NewProject(tspy, opt)

		// --- Then ---
		assert.Same(t, have, seen)

		have.Close() // Must close to prevent error.
	})
}

func Test_NewNamedProject(t *testing.T) {
	t.Run("uses name as directory basename", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		// --- When ---
		have := NewNamedProject(tspy, "custom")

		// --- Then ---
		assert.DirExist(t, have.Root())
		assert.Equal(t, "custom", filepath.Base(have.Root()))
		assert.Equal(t, prjkit.GoModNameStem+"custom", have.ImpSpec())

		have.Close() // Must close to prevent error.
	})

	t.Run("forwards options", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()

		var seen *prjkit.Project
		opt := func(prj *prjkit.Project) { seen = prj }

		// --- When ---
		have := NewNamedProject(tspy, "custom", opt)

		// --- Then ---
		assert.Same(t, have, seen)

		have.Close() // Must close to prevent error.
	})

	t.Run("error - name with separator", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectFatal()
		tspy.ExpectLogEqual("gmtest: invalid project name \"a/b\"")
		tspy.Close()

		// --- When ---
		assert.Panic(t, func() { NewNamedProject(tspy, "a/b") })
	})

	t.Run("error - empty name", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectFatal()
		tspy.ExpectLogEqual("gmtest: invalid project name \"\"")
		tspy.Close()

		// --- When ---
		assert.Panic(t, func() { NewNamedProject(tspy, "") })
	})
}
