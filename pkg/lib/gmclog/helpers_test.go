// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmclog

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_CreateFile(t *testing.T) {
	t.Run("not existing file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "CHANGELOG.md")

		// --- When ---
		err := CreateFile(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, pth)
	})

	t.Run("does not truncate existing", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, "abc", t.TempDir(), "CHANGELOG.md")

		// --- When ---
		err := CreateFile(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, pth)
		assert.Equal(t, "abc", oskit.ReadFileStr(t, pth))
	})

	t.Run("error - parent is a file", func(t *testing.T) {
		// --- Given ---
		fil := oskit.Create(t, "x", t.TempDir(), "file")

		// --- When ---
		err := CreateFile(filepath.Join(fil, "child"))

		// --- Then ---
		assert.ErrorIs(t, syscall.ENOTDIR, err)
	})

	t.Run("error - parent does not exist", func(t *testing.T) {
		// --- When ---
		err := CreateFile(filepath.Join(t.TempDir(), "nope", "CHANGELOG.md"))

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
	})

	t.Run("error - path is a directory", func(t *testing.T) {
		// --- When ---
		err := CreateFile(t.TempDir())

		// --- Then ---
		assert.ErrorContain(t, "is a directory", err)
	})
}
