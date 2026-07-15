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
		assert.Equal(t, "project", filepath.Base(have.Root()))
		assert.Equal(t, prjkit.GoModName, have.ImpSpec())

		have.Close() // Must close to prevent error.
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
		have := NewProject(tspy, opt)

		// --- Then ---
		assert.True(t, called)

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
		name := "custom"

		// --- When ---
		have := NewNamedProject(tspy, name)

		// --- Then ---
		assert.Equal(t, name, filepath.Base(have.Root()))
		assert.Equal(t, prjkit.GoModNameStem+name, have.ImpSpec())

		have.Close() // Must close to prevent error.
	})

	t.Run("forwards options", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectCleanups(1)
		tspy.ExpectTempDir(1)
		tspy.Close()
		name := "custom"
		called := false
		opt := func(*prjkit.Project) { called = true }

		// --- When ---
		have := NewNamedProject(tspy, name, opt)

		// --- Then ---
		assert.True(t, called)

		have.Close() // Must close to prevent error.
	})
}
