// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
)

func Test_ImpPath(t *testing.T) {
	t.Run("error - not go module - dir arg", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		have, err := ImpPath(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
		assert.ErrorEqual(t, gomake.ErrNoGoMod.Error()+": "+prj.Root(), err)
		assert.Empty(t, have)
	})

	t.Run("error - not go module - cwd", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := ImpPath(t.Context(), rng, "")

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
		assert.ErrorContain(t, prj.Root(), err)
		assert.Empty(t, have)
	})

	t.Run("error - invalid go.mod file", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFile("go.mod")
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := ImpPath(t.Context(), rng, "")

		// --- Then ---
		assert.ErrorIs(t, ErrImpPath, err)
		assert.ErrorContain(t, prj.Root(), err)
		assert.Empty(t, have)
	})

	t.Run("error - go list fails", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet("GOFLAGS", "-mod=bogus")

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		// --- When ---
		have, err := ImpPath(t.Context(), rng, "")

		// --- Then ---
		assert.ErrorIs(t, ErrImpPath, err)
		assert.ErrorContain(t, prj.Root(), err)
		assert.ErrorContain(t, "-mod=bogus", err)
		assert.Empty(t, have)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := ImpPath(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prjkit.GoModName, have)
	})

	t.Run("workspace", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		mod := "module example.com/sub\n\ngo 1.26\n"
		prj.CreateFileWith(mod, "sub", "go.mod")
		prj.CreateFileWith("go 1.26\n\nuse (\n\t.\n\t./sub\n)\n", "go.work")
		prj.Close()

		// --- When ---
		have, err := ImpPath(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prjkit.GoModName, have)
	})
}

func Test_pinGoMajorMinor(t *testing.T) {
	t.Run("rewrites patch version", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		dir := t.TempDir()
		ver := semver.MustParse(strings.TrimPrefix(runtime.Version(), "go"))
		oskit.Write(t, "module x\n\ngo "+ver.String()+"\n", dir, "go.mod")

		// --- When ---
		err := pinGoMajorMinor(t.Context(), rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		want := fmt.Sprintf("\ngo %d.%d\n", ver.Major(), ver.Minor())
		assert.Contain(t, want, oskit.ReadFileStr(t, dir, "go.mod"))
	})

	t.Run("rewrites previous minor", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		dir := t.TempDir()
		ver := semver.MustParse(strings.TrimPrefix(runtime.Version(), "go"))
		format := "module x\n\ngo %d.%d.0\n"
		mod := fmt.Sprintf(format, ver.Major(), ver.Minor()-1)
		oskit.Write(t, mod, dir, "go.mod")

		// --- When ---
		err := pinGoMajorMinor(t.Context(), rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		want := fmt.Sprintf("\ngo %d.%d\n", ver.Major(), ver.Minor())
		assert.Contain(t, want, oskit.ReadFileStr(t, dir, "go.mod"))
	})

	t.Run("already major minor", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		dir := t.TempDir()
		ver := semver.MustParse(strings.TrimPrefix(runtime.Version(), "go"))
		mod := fmt.Sprintf("module x\n\ngo %d.%d\n", ver.Major(), ver.Minor())
		oskit.Write(t, mod, dir, "go.mod")

		// --- When ---
		err := pinGoMajorMinor(t.Context(), rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, mod, oskit.ReadFileStr(t, dir, "go.mod"))
	})

	t.Run("no go directive", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		dir := t.TempDir()
		oskit.Write(t, "module x\n", dir, "go.mod")

		// --- When ---
		err := pinGoMajorMinor(t.Context(), rng, dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "module x\n", oskit.ReadFileStr(t, dir, "go.mod"))
	})

	t.Run("error - go.mod missing", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		// --- When ---
		err := pinGoMajorMinor(t.Context(), rng, t.TempDir())

		// --- Then ---
		assert.ErrorIs(t, ErrModInit, err)
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})

	t.Run("error - go env fails", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOTOOLCHAIN", "bogus")
		dir := t.TempDir()
		oskit.Write(t, "module x\n\ngo 1.26.3\n", dir, "go.mod")

		// --- When ---
		err := pinGoMajorMinor(t.Context(), rng, dir)

		// --- Then ---
		assert.ErrorIs(t, ErrModInit, err)
		assert.ErrorContain(t, `invalid GOTOOLCHAIN "bogus"`, err)
	})

	t.Run("error - go mod edit fails", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		dir := t.TempDir()
		oskit.Write(t, "module x\n\ngo 01.2\n", dir, "go.mod")

		// --- When ---
		err := pinGoMajorMinor(t.Context(), rng, dir)

		// --- Then ---
		assert.ErrorIs(t, ErrModInit, err)
		assert.ErrorContain(t, "invalid go version '01.2'", err)
	})
}

func Test_InitModule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := InitModule(t.Context(), rng, prj.Root(), "package")

		// --- Then ---
		assert.NoError(t, err)
		mod := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module package", mod)
		// The "go" directive is pinned to "major.minor", not the toolchain's
		// patch version.
		ver := semver.MustParse(strings.TrimPrefix(runtime.Version(), "go"))
		want := fmt.Sprintf("go %d.%d\n", ver.Major(), ver.Minor())
		assert.Contain(t, want, mod)
	})

	t.Run("error - directory does not exist", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		dir := prj.Path("not_existing")

		// --- When ---
		err := InitModule(t.Context(), rng, dir, "package")

		// --- Then ---
		assert.ErrorIs(t, ErrModInit, err)
		assert.ErrorContain(t, dir, err)
	})

	t.Run("error - invalid module name", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		module := "example.com:project/abc-proj"

		// --- When ---
		err := InitModule(t.Context(), rng, prj.Root(), module)

		// --- Then ---
		assert.ErrorIs(t, ErrModInit, err)
		want := regexp.QuoteMeta(prj.Root()) + ".*" +
			regexp.QuoteMeta(fmt.Sprintf("malformed module path %q", module))
		assert.ErrorRegexp(t, want, err)
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
		ctx := t.Context()
		rng := ringtest.New(t).Ring()
		repo := setupConfigRepo(t)
		dst := filepath.Join(t.TempDir(), ".golangci.yml")

		// --- When ---
		err := gitGetFile(ctx, rng, repo, "master", ".golangci.yml", dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, dst)
		assert.True(t, oskit.FileSize(t, dst) > 0)
	})

	t.Run("error - destination directory missing", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		rng := ringtest.New(t).Ring()
		repo := setupConfigRepo(t)
		dst := filepath.Join(t.TempDir(), "missing", ".golangci.yml")

		// --- When ---
		err := gitGetFile(ctx, rng, repo, "master", ".golangci.yml", dst)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})

	t.Run("error - clone fails", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		rng := ringtest.New(t).Ring()
		repo := filepath.Join(t.TempDir(), "not-a-repo")
		dst := filepath.Join(t.TempDir(), ".golangci.yml")

		// --- When ---
		err := gitGetFile(ctx, rng, repo, "master", ".golangci.yml", dst)

		// --- Then ---
		assert.ErrorContain(t, "git clone "+repo+": ", err)
		assert.ErrorContain(t, "exit status 128", err)
	})

	t.Run("uses ring environment", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		repo := setupConfigRepo(t)
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GIT_CONFIG_COUNT", "1")
		rng.EnvSet("GIT_CONFIG_KEY_0", "url."+repo+".insteadOf")
		rng.EnvSet("GIT_CONFIG_VALUE_0", "alias:cfg")
		dst := filepath.Join(t.TempDir(), ".golangci.yml")

		// --- When ---
		err := gitGetFile(ctx, rng, "alias:cfg", "", ".golangci.yml", dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, dst)
	})

	t.Run("error - context cancelled", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		repo := setupConfigRepo(t)
		dst := filepath.Join(t.TempDir(), ".golangci.yml")

		// --- When ---
		err := gitGetFile(ctx, ringtest.New(t).Ring(), repo, "", "x", dst)

		// --- Then ---
		assert.ErrorIs(t, context.Canceled, err)
		assert.ErrorContain(t, "git clone "+repo, err)
	})

	t.Run("error - source not in repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		rng := ringtest.New(t).Ring()
		repo := setupConfigRepo(t)
		dst := filepath.Join(t.TempDir(), "out.yml")

		// --- When ---
		err := gitGetFile(ctx, rng, repo, "master", "missing.yml", dst)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
	})
}

func Test_freeAddr(t *testing.T) {
	t.Run("bindable loopback address", func(t *testing.T) {
		// --- When ---
		have, err := freeAddr()

		// --- Then ---
		assert.NoError(t, err)
		_, port := must.Values(net.SplitHostPort(have))
		assert.NotEqual(t, "0", port)
		lis := must.Value(net.Listen("tcp", have))
		_ = lis.Close()
	})
}

func Test_BldDateFmt(t *testing.T) {
	t.Run("millisecond precision in UTC", func(t *testing.T) {
		// --- Given ---
		loc := time.FixedZone("CET", 2*60*60)
		tim := time.Date(2026, 7, 9, 12, 0, 0, 123_456_789, loc)

		// --- When ---
		have := BldDateFmt(tim)

		// --- Then ---
		assert.Equal(t, "2026-07-09T10:00:00.123Z", have)
	})

	t.Run("trailing zero kept", func(t *testing.T) {
		// --- Given ---
		tim := time.Date(2000, 1, 2, 3, 4, 5, 600_000_000, time.UTC)

		// --- When ---
		have := BldDateFmt(tim)

		// --- Then ---
		assert.Equal(t, "2000-01-02T03:04:05.600Z", have)
	})

	t.Run("matches xdef rendering", func(t *testing.T) {
		// --- Given ---
		want := xdef.BldDateStr()
		tim := must.Value(time.Parse(time.RFC3339Nano, want))

		// --- When ---
		have := BldDateFmt(tim)

		// --- Then ---
		assert.Equal(t, want, have)
		assert.Regexp(t, `\.\d{3}Z$`, want)
	})
}

func Test_extractGolangCiVersion(t *testing.T) {
	t.Run("success - short", func(t *testing.T) {
		// --- Given ---
		line := "version v1.53.3 built with go1.20.5"

		// --- When ---
		have, err := extractGolangCiVersion(line)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.53.3", have.Original())
	})

	t.Run("success - long", func(t *testing.T) {
		// --- Given ---
		line := "" +
			"golangci-lint has version v1.55.2 built with go1.21.3 from " +
			"(unknown, " +
			"mod sum: \"h1:yllEIsSJ7MtlDBwDJ9IMBkyEUz2fYE0b5B8IUgO1oP8=\") " +
			"on (unknown)"

		// --- When ---
		have, err := extractGolangCiVersion(line)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.55.2", have.Original())
	})

	t.Run("error - invalid version line", func(t *testing.T) {
		// --- When ---
		have, err := extractGolangCiVersion("v1.53.3")

		// --- Then ---
		assert.ErrorIs(t, semver.ErrInvalidSemVer, err)
		assert.Nil(t, have)
	})
}
