// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmmce

import (
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// egFixture is the testdata example file copied into scanned directories.
const egFixture = "testdata/pkg1/examples_test.go"

// copyExamples creates a "pkg1" package directory under dir and copies
// [egFixture] into it, so the scanned tree holds a known example function.
func copyExamples(t tester.T, dir string) {
	t.Helper()
	sub := oskit.MkdirAll(t, dir, "pkg1")
	oskit.CopyFile(t, sub, egFixture)
}
