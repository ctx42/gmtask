// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmclog_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ctx42/gmtask/pkg/lib/gmclog"
)

func ExampleRelease_String() {
	date := time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC)
	rel, err := gmclog.NewRelease("v0.1.2", date)
	if err != nil {
		fmt.Println(err)
		return
	}
	rel.AddChange("Add feature", "Fix bug")

	fmt.Print(rel.String())
	// Output:
	// ## v0.1.2 (Sun, 02 Jan 2000 03:04:06 UTC)
	// - Add feature.
	// - Fix bug.
}

func ExampleReadReleases() {
	clg, err := gmclog.ReadReleases("testdata", "changelog_0.md")
	if err != nil {
		fmt.Println(err)
		return
	}

	rel := clg.Releases[0]
	fmt.Println(rel.Version.Original())
	fmt.Println(rel.Changes)
	// Output:
	// v0.1.5
	// [- Change 1. - Change 2.]
}

func ExampleChangelog_Save() {
	dir, err := os.MkdirTemp("", "gmclog-example-")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	pth := filepath.Join(dir, "CHANGELOG.md")
	if err = gmclog.CreateFile(pth); err != nil {
		fmt.Println(err)
		return
	}
	clg, err := gmclog.ReadChangelog(pth)
	if err != nil {
		fmt.Println(err)
		return
	}

	date := time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC)
	rel, err := gmclog.NewRelease("v0.1.0", date)
	if err != nil {
		fmt.Println(err)
		return
	}
	rel.AddChange("Initial release")
	clg.AddRelease(rel)
	if err = clg.Save(); err != nil {
		fmt.Println(err)
		return
	}

	data, err := os.ReadFile(pth) //nolint:gosec
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(string(data))
	// Output:
	// ## v0.1.0 (Sun, 02 Jan 2000 03:04:06 UTC)
	// - Initial release.
}
