// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmmce

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFile replaces the file at pth with data. It writes a temporary file in
// the same directory and renames it into place, so a failed write leaves the
// original file intact. The new file keeps the mode of the one it replaces.
func writeFile(pth string, data []byte) error {
	inf, err := os.Stat(pth)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(pth), ".gmmce-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // No-op after the rename.

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err = tmp.Chmod(inf.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temp file mode: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err = os.Rename(tmpName, pth); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
