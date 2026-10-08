// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmclog

import (
	"fmt"
	"os"
)

// CreateFile creates an empty file at pth when none exists; an existing file
// is left untouched. Creating and checking happen in one open call, so a file
// created concurrently by another process is never truncated. It returns an
// error when pth is a directory.
func CreateFile(pth string) error {
	fil, err := os.OpenFile(pth, os.O_RDONLY|os.O_CREATE, 0o666) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	inf, err := fil.Stat()
	_ = fil.Close()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if inf.IsDir() {
		return fmt.Errorf("create file: %s: is a directory", pth)
	}
	return nil
}
