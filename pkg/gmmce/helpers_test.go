// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmmce

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_writeFile(t *testing.T) {
	t.Run("replaces contents and keeps mode", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := oskit.Write(t, "old", dir, "README.md")
		must.Nil(os.Chmod(pth, 0o640))

		// --- When ---
		err := writeFile(pth, []byte("new"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "new", oskit.ReadFileStr(t, pth))
		assert.Equal(t, os.FileMode(0o640), oskit.Stat(t, pth).Mode().Perm())
		assert.Equal(t, []string{"README.md"}, oskit.Readdirnames(t, dir))
	})

	t.Run("error - file does not exist", func(t *testing.T) {
		// --- When ---
		err := writeFile(filepath.Join(t.TempDir(), "nope.md"), nil)

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
	})

	t.Run("error - rename fails", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := oskit.MkdirAll(t, dir, "README.md")
		oskit.Write(t, "x", pth, "file")

		// --- When ---
		err := writeFile(pth, []byte("new"))

		// --- Then ---
		assert.ErrorContain(t, "replace file", err)
		assert.Equal(t, []string{"README.md"}, oskit.Readdirnames(t, dir))
	})
}
