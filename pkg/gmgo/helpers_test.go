// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"

	"github.com/ctx42/gmtask/internal/gmtest"
)

func Test_ImpPath(t *testing.T) {
	t.Run("error - not go module - dir arg", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		have, err := ImpPath(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
		assert.ErrorContain(t, prj.Root(), err)
		assert.Empty(t, have)
	})

	t.Run("error - not go module - cwd", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := ImpPath(ctx, rng, "")

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
		assert.ErrorContain(t, prj.Root(), err)
		assert.Empty(t, have)
	})

	t.Run("error - invalid go.mod file", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFile("go.mod")
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := ImpPath(ctx, rng, "")

		// --- Then ---
		assert.ErrorIs(t, ErrImpPath, err)
		assert.ErrorContain(t, prj.Root(), err)
		assert.Empty(t, have)
	})

	t.Run("error - go list fails", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet("GOWORK", "dir")

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := ImpPath(ctx, rng, "")

		// --- Then ---
		assert.ErrorIs(t, ErrImpPath, err)
		assert.ErrorContain(t, prj.Root(), err)
		assert.ErrorContain(t, "cannot determine Go module import path", err)
		assert.Empty(t, have)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := ImpPath(ctx, rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prjkit.GoModName, have)
	})
}

func Test_InitModule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := InitModule(ctx, rng, prj.Root(), "package")

		// --- Then ---
		assert.NoError(t, err)
		have := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module package", have)

		// The "go" directive is pinned to "major.minor", not the toolchain's
		// patch version.
		ver := semver.MustParse(strings.TrimPrefix(runtime.Version(), "go"))
		want := fmt.Sprintf("go %d.%d\n", ver.Major(), ver.Minor())
		assert.Contain(t, want, have)
	})

	t.Run("error - directory does not exist", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		dir := prj.Path("not_existing")

		// --- When ---
		err := InitModule(ctx, rng, dir, "package")

		// --- Then ---
		assert.ErrorIs(t, ErrModInit, err)
		assert.ErrorContain(t, dir, err)
	})

	t.Run("error - invalid module name", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		module := "example.com:project/abc-proj"

		// --- When ---
		err := InitModule(ctx, rng, prj.Root(), module)

		// --- Then ---
		assert.ErrorIs(t, ErrModInit, err)
		msg := err.Error()
		assert.Contain(t, prj.Root(), msg)
		assert.Contain(t, fmt.Sprintf("malformed module path %q", module), msg)
	})
}

func Test_LDFlags(t *testing.T) {
	t.Run("multiple variables", func(t *testing.T) {
		// --- Given ---
		vars := []LDVar{
			{Name: "scmRev", Value: "v1.1"},
			{Name: "scmHash", Value: "abc"},
			{Name: "scmState", Value: "clean"},
		}

		// --- When ---
		have := LDFlags("example.com/pkg", vars)

		// --- Then ---
		want := "" +
			"-X 'example.com/pkg.scmRev=v1.1' " +
			"-X 'example.com/pkg.scmHash=abc' " +
			"-X 'example.com/pkg.scmState=clean'"
		assert.Equal(t, want, have)
	})

	t.Run("no variables", func(t *testing.T) {
		// --- When ---
		have := LDFlags("example.com/pkg", nil)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_CovLogFilename(t *testing.T) {
	t.Run("BUILD_ID not empty", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet(BuildIDEnvKey, "123")

		// --- When ---
		have := CovLogFilename(rng)

		// --- Then ---
		assert.Equal(t, "go_test_coverage_123.log", have)
	})

	t.Run("empty BUILD_ID", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		// --- When ---
		have := CovLogFilename(rng)

		// --- Then ---
		assert.Equal(t, "go_test_coverage.log", have)
	})
}

func Test_TestLogFilename(t *testing.T) {
	t.Run("BUILD_ID not empty", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet(BuildIDEnvKey, "123")

		// --- When ---
		have := TestLogFilename(rng)

		// --- Then ---
		assert.Equal(t, "go_test_run_123.log", have)
	})

	t.Run("empty BUILD_ID", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		// --- When ---
		have := TestLogFilename(rng)

		// --- Then ---
		assert.Equal(t, "go_test_run.log", have)
	})
}

func Test_gitGetFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		repo := setupConfigRepo(t)
		dst := filepath.Join(t.TempDir(), ".golangci.yml")

		// --- When ---
		err := gitGetFile(ctx, repo, "master", ".golangci.yml", dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, dst)
		assert.True(t, oskit.FileSize(t, dst) > 0)
	})

	t.Run("error - destination directory missing", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		repo := setupConfigRepo(t)
		dst := filepath.Join(t.TempDir(), "missing", ".golangci.yml")

		// --- When ---
		err := gitGetFile(ctx, repo, "master", ".golangci.yml", dst)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})

	t.Run("error - clone fails", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		repo := filepath.Join(t.TempDir(), "not-a-repo")
		dst := filepath.Join(t.TempDir(), ".golangci.yml")

		// --- When ---
		err := gitGetFile(ctx, repo, "master", ".golangci.yml", dst)

		// --- Then ---
		assert.ErrorContain(t, "git clone", err)
	})

	t.Run("error - source not in repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		repo := setupConfigRepo(t)
		dst := filepath.Join(t.TempDir(), "out.yml")

		// --- When ---
		err := gitGetFile(ctx, repo, "master", "missing.yml", dst)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})
}

func Test_freeAddr(t *testing.T) {
	t.Run("returns a bindable loopback address", func(t *testing.T) {
		// --- When ---
		have, err := freeAddr()

		// --- Then ---
		assert.NoError(t, err)

		_, port, err := net.SplitHostPort(have)
		assert.NoError(t, err)
		assert.NotEqual(t, "0", port)

		lis, err := net.Listen("tcp", have)
		assert.NoError(t, err)
		_ = lis.Close()
	})
}

func Test_rfc3339Milli(t *testing.T) {
	t.Run("truncates to millisecond and normalizes to UTC", func(t *testing.T) {
		// --- Given ---
		loc := time.FixedZone("CET", 2*60*60)
		tim := time.Date(2026, 7, 9, 12, 0, 0, 123_456_789, loc)

		// --- When ---
		have := rfc3339Milli(tim)

		// --- Then ---
		assert.Equal(t, "2026-07-09T10:00:00.123Z", have)
	})
}

func Test_extractGolangCiVersion(t *testing.T) {
	t.Run("success - short", func(t *testing.T) {
		// --- Given ---
		line := "version v1.53.3 built with go1.20.5"

		// --- When ---
		ver, err := extractGolangCiVersion(line)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.53.3", ver.Original())
	})

	t.Run("success - long", func(t *testing.T) {
		// --- Given ---
		line := "golangci-lint has version v1.55.2 built with go1.21.3 from " +
			"(unknown, " +
			"mod sum: \"h1:yllEIsSJ7MtlDBwDJ9IMBkyEUz2fYE0b5B8IUgO1oP8=\") " +
			"on (unknown)"

		// --- When ---
		ver, err := extractGolangCiVersion(line)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.55.2", ver.Original())
	})

	t.Run("error - invalid version line", func(t *testing.T) {
		// --- Given ---
		line := "v1.53.3"

		// --- When ---
		ver, err := extractGolangCiVersion(line)

		// --- Then ---
		assert.ErrorIs(t, semver.ErrInvalidSemVer, err)
		assert.Nil(t, ver)
	})
}
