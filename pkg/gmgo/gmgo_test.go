// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/jsonkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
)

func Test_Go_Vet(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Vet(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - vet detects issue", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/failure/project")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Vet(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "source.go:10:22: fmt.Sprintf format", tst.Stderr())
	})

	t.Run("tmp exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Vet(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("ignores delivered config block", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		// Vet never decodes; a stray key in the delivered block must not fail it.
		cfg := jsonkit.To(t, map[string]any{"nonsense": "x"})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Go{}.Vet(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
	})
}

func Test_Go_Check(t *testing.T) {
	t.Run("success - lint ignores the sibling timeout", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t, prjkit.WithProjectEnv(os.Environ()))
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)
		// gomake delivers :go:check the go-node block ({timeout}). The lint step
		// reads only its own "version"/"file" keys and ignores the sibling
		// "timeout", so the whole block passes through to Test unchanged.
		cfg := jsonkit.To(t, map[string]any{"timeout": "10m"})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Go{}.Check(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "0 issues.", tst.Stdout())
		assert.Contain(t, "ok  \texample.com/comp/project", tst.Stdout())
	})

	t.Run("error - vet fails first", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/failure/project")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Check(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "source.go:10:22: fmt.Sprintf format", tst.Stderr())
	})
}

func Test_Go_TestV(t *testing.T) {
	t.Run("success - tmp dir does not exist", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		rng := tst.Ring("--", "-timeout", "10m")

		prj := gmtest.NewProject(t, prjkit.WithProjectEnv(os.Environ()))
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		wantLog := prj.ReadFileStr(TestLogFilename(rng))
		have := tst.Stdout()
		assert.Equal(t, wantLog, have)
		assert.Contain(t, "--- PASS: Test_Hello (", have)
		assert.Contain(t, "coverage: 100.0% of statements", have)

		want := "" +
			"mode: atomic\n" +
			"example.com/comp/project/source.go:4.21,4.45 1 1\n"
		have = prj.ReadFileStr(CovLogFilename(rng))
		assert.Equal(t, want, have)
	})

	t.Run("success - tmp dir exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		wantLog := prj.ReadFileStr("tmp", TestLogFilename(rng))
		have := tst.Stdout()
		assert.Equal(t, wantLog, have)
		assert.Contain(t, "--- PASS: Test_Hello (", have)
		assert.Contain(t, "coverage: 100.0% of statements", have)

		want := "" +
			"mode: atomic\n" +
			"example.com/comp/project/source.go:4.21,4.45 1 1\n"
		have = prj.ReadFileStr("tmp", CovLogFilename(rng))
		assert.Equal(t, want, have)
	})

	t.Run("success - existing custom tmp dir", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		tmp := t.TempDir()
		rng := tst.Ring("--dir", tmp)

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		wantLog := oskit.ReadFileStr(t, tmp, TestLogFilename(rng))
		have := tst.Stdout()
		assert.Equal(t, wantLog, have)
		assert.Contain(t, "--- PASS: Test_Hello (", have)
		assert.Contain(t, "coverage: 100.0% of statements", have)

		want := "" +
			"mode: atomic\n" +
			"example.com/comp/project/source.go:4.21,4.45 1 1\n"
		have = oskit.ReadFileStr(t, tmp, CovLogFilename(rng))
		assert.Equal(t, want, have)
	})

	t.Run("set timeout using environment variable", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		rng := tst.Ring()
		rng.EnvSet(GoTestTimeoutEnvKey, "1ns")

		prj := gmtest.NewProject(t, prjkit.WithProjectEnv(os.Environ()))
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "test timed out after 1ns", tst.Stdout())
	})

	t.Run("set timeout using configuration", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		rng := tst.Ring()
		cfg := jsonkit.To(t, map[string]any{"timeout": "1ns"})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		prj := gmtest.NewProject(t, prjkit.WithProjectEnv(os.Environ()))
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "test timed out after 1ns", tst.Stdout())
	})

	t.Run("set timeout using numeric configuration", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		rng := tst.Ring()
		// A JSON number is read as the nanosecond count, so 1 is 1ns.
		rng.MetaSet(gomake.ConfigMetaKey, jsonkit.To(t, map[string]any{"timeout": 1}))

		prj := gmtest.NewProject(t, prjkit.WithProjectEnv(os.Environ()))
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "test timed out after 1ns", tst.Stdout())
	})

	t.Run("environment timeout overrides configuration", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		rng := tst.Ring()
		cfg := jsonkit.To(t, map[string]any{"timeout": "10m"})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)
		rng.EnvSet(GoTestTimeoutEnvKey, "1ns")

		prj := gmtest.NewProject(t, prjkit.WithProjectEnv(os.Environ()))
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "test timed out after 1ns", tst.Stdout())
	})

	t.Run("error - config type mismatch", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		rng := tst.Ring()
		cfg := jsonkit.To(t, map[string]any{"timeout": "nope"})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		prj := gmtest.NewProject(t, prjkit.WithProjectEnv(os.Environ()))
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrType, err)
		assert.NoFileExist(t, prj.Path(TestLogFilename(rng)))
	})

	t.Run("additional args passed to test", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		rng := tst.Ring("--", "-timeout", "1ns")

		prj := gmtest.NewProject(t, prjkit.WithProjectEnv(os.Environ()))
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "test timed out after 1ns", tst.Stdout())
	})

	t.Run("error - custom tmp dir does not exist", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--dir", "not_existing")

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "not_existing", err)
	})

	t.Run("error - failing tests", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/failing_tests/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		wantLog := prj.ReadFileStr("tmp", TestLogFilename(rng))
		have := tst.Stdout()
		assert.Equal(t, wantLog, have)
		assert.Contain(t, "source_test.go:9: expected different result", have)
		assert.Contain(t, "--- FAIL: Test_Hello (", have)
		assert.Contain(t, "coverage: 100.0% of statements", have)

		want := "" +
			"mode: atomic\n" +
			"example.com/comp/project/source.go:7.21,7.70 1 1\n"
		have = prj.ReadFileStr("tmp", CovLogFilename(rng))
		assert.Equal(t, want, have)
	})

	t.Run("error - not go module", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "FAIL\t./... [setup failed]", tst.Stdout())
		assert.Contain(t, "does not contain main module", tst.Stderr())
	})

	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--help")

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :go:test-v\n" +
			"      --dir     directory to put reports to (default: \".\" or \"tmp\" if exists)\n" +
			"  -h, --help    show help\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("-unknown")

		// --- When ---
		err := Go{}.TestV(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :go:test-v\n" +
			"      --dir     directory to put reports to (default: \".\" or \"tmp\" if exists)\n" +
			"  -h, --help    show help\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Go_Test(t *testing.T) {
	t.Run("success - tmp dir does not exist", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		wantLog := prj.ReadFileStr(TestLogFilename(rng))
		have := tst.Stdout()
		assert.Equal(t, wantLog, have)
		assert.Contain(t, "ok  \texample.com/comp/project\t", have)
		assert.Contain(t, "coverage: 100.0% of statements", have)

		want := "" +
			"mode: atomic\n" +
			"example.com/comp/project/source.go:4.21,4.45 1 1\n"
		have = prj.ReadFileStr(CovLogFilename(rng))
		assert.Equal(t, want, have)
	})

	t.Run("success - tmp dir exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		wantLog := prj.ReadFileStr("tmp", TestLogFilename(rng))
		have := tst.Stdout()
		assert.Equal(t, wantLog, have)
		assert.Contain(t, "ok  \texample.com/comp/project\t", have)
		assert.Contain(t, "coverage: 100.0% of statements", have)

		want := "" +
			"mode: atomic\n" +
			"example.com/comp/project/source.go:4.21,4.45 1 1\n"
		have = prj.ReadFileStr("tmp", CovLogFilename(rng))
		assert.Equal(t, want, have)
	})

	t.Run("success - existing custom tmp dir", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		tmp := t.TempDir()
		rng := tst.Ring("--dir", tmp)

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		wantLog := oskit.ReadFileStr(t, tmp, TestLogFilename(rng))
		have := tst.Stdout()
		assert.Equal(t, wantLog, have)
		assert.Contain(t, "ok  \texample.com/comp/project\t", have)
		assert.Contain(t, "coverage: 100.0% of statements", have)

		want := "" +
			"mode: atomic\n" +
			"example.com/comp/project/source.go:4.21,4.45 1 1\n"
		have = oskit.ReadFileStr(t, tmp, CovLogFilename(rng))
		assert.Equal(t, want, have)
	})

	t.Run("error - custom tmp dir does not exist", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--dir", "not_existing")

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "not_existing", err)
	})

	t.Run("error - failing tests", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/failing_tests/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		wantLog := prj.ReadFileStr("tmp", TestLogFilename(rng))
		have := tst.Stdout()
		assert.Equal(t, wantLog, have)
		assert.Contain(t, "source_test.go:9: expected different result", have)
		assert.Contain(t, "--- FAIL: Test_Hello (", have)
		assert.Contain(t, "coverage: 100.0% of statements", have)

		want := "" +
			"mode: atomic\n" +
			"example.com/comp/project/source.go:7.21,7.70 1 1\n"
		have = prj.ReadFileStr("tmp", CovLogFilename(rng))
		assert.Equal(t, want, have)
	})

	t.Run("error - not go module", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()

		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "FAIL\t./... [setup failed]", tst.Stdout())
		assert.Contain(t, "does not contain main module", tst.Stderr())
	})

	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("-h")

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :go:test\n" +
			"      --dir     directory to put reports to (default: \".\" or \"tmp\" if exists)\n" +
			"  -h, --help    show help\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("-unknown")

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :go:test\n" +
			"      --dir     directory to put reports to (default: \".\" or \"tmp\" if exists)\n" +
			"  -h, --help    show help\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Go_test(t *testing.T) {
	t.Run("tmp directory created", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Test(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, prj.Path("tmp", TestLogFilename(rng)))
		assert.Contain(t, "ok  \texample.com/comp/project", tst.Stdout())
	})
}

func Test_parseDirTarget(t *testing.T) {
	t.Run("resolves dir flag and remaining args", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring("--dir", "reports", "./pkg/...")

		// --- When ---
		out, next, help, err := parseDirTarget(rng, ":go:test", "dir help")

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, help)
		assert.Equal(t, "reports", out)
		assert.Equal(t, []string{"./pkg/..."}, next.Args())
	})

	t.Run("help requested writes usage", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		out, _, help, err := parseDirTarget(rng, ":go:test", "dir help")

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, help)
		assert.Equal(t, "", out)
		assert.Contain(t, "Usage of :go:test", tst.Stderr())
	})

	t.Run("error - unknown flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("-unknown")

		// --- When ---
		_, _, _, err := parseDirTarget(rng, ":go:test", "dir help")

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		assert.Contain(t, "Usage of :go:test", tst.Stderr())
	})
}

func Test_Go_Doc(t *testing.T) {
	t.Run("error - not go project", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Doc(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
		assert.ErrorContain(t, prj.Root(), err)
	})
}

func Test_Go_Pkgsite(t *testing.T) {
	t.Run("error - not go project", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Go{}.Pkgsite(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
		assert.ErrorContain(t, prj.Root(), err)
	})
}

func Test_serveDocServer(t *testing.T) {
	t.Run("error - not a go module", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		serve := func(context.Context, *ring.Ring, string) error {
			return errors.New("serve must not be called")
		}

		// --- When ---
		err := serveDocServer(ctx, rng, serve, "/pkg/")

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
	})

	t.Run("returns the serve error", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		wantErr := errors.New("serve failed")
		serve := func(context.Context, *ring.Ring, string) error {
			return wantErr
		}

		// --- When ---
		err := serveDocServer(ctx, rng, serve, "/pkg/")

		// --- Then ---
		assert.ErrorIs(t, wantErr, err)
	})
}

func Test_serveDoc(t *testing.T) {
	t.Run("error - godoc fails before context is done", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()
		addr := "localhost:0"

		// Point PATH at an empty directory, so the "godoc" binary cannot be
		// resolved, and its launch fails immediately, whether godoc is
		// installed on the host.
		t.Setenv("PATH", t.TempDir())

		// --- When ---
		err := serveDoc(ctx, rng, addr)

		// --- Then ---
		assert.Error(t, err)
	})

	t.Run("no error when context cancelled", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rng := ringtest.New(t).Ring()
		addr := "localhost:0"

		// --- When ---
		err := serveDoc(ctx, rng, addr)

		// --- Then ---
		assert.NoError(t, err)
	})
}

func Test_servePkgsite(t *testing.T) {
	t.Run("error - pkgsite fails before context is done", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()
		addr := "localhost:0"

		// Point PATH at an empty directory so the "pkgsite" binary cannot be
		// resolved and its launch fails immediately, whether or not pkgsite is
		// installed on the host.
		t.Setenv("PATH", t.TempDir())

		// --- When ---
		err := servePkgsite(ctx, rng, addr)

		// --- Then ---
		assert.Error(t, err)
	})

	t.Run("no error when context cancelled", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rng := ringtest.New(t).Ring()
		addr := "localhost:0"

		// --- When ---
		err := servePkgsite(ctx, rng, addr)

		// --- Then ---
		assert.NoError(t, err)
	})
}

func Test_browserCmd_tabular(t *testing.T) {
	tt := []struct {
		testN string

		goos     string
		wantName string
		wantArgs []string
	}{
		{"linux", "linux", "xdg-open", []string{"http://x/"}},
		{"macos", "darwin", "open", []string{"http://x/"}},
		{
			"windows",
			"windows",
			"rundll32",
			[]string{"url.dll,FileProtocolHandler", "http://x/"},
		},
		{"other unix", "freebsd", "xdg-open", []string{"http://x/"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			name, args := browserCmd(tc.goos, "http://x/")

			// --- Then ---
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantArgs, args)
		})
	}
}

func Test_waitForServer(t *testing.T) {
	t.Run("true once the server responds", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		srv := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {},
		))
		defer srv.Close()

		// --- When ---
		have := waitForServer(ctx, srv.URL+"/")

		// --- Then ---
		assert.True(t, have)
	})

	t.Run("false when context cancelled before server responds", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Port 0 is never listening, so the poll depends on ctx to stop.
		url := "http://localhost:0/"

		// --- When ---
		have := waitForServer(ctx, url)

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("error - false on malformed url", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		// --- When ---
		have := waitForServer(ctx, "://bad")

		// --- Then ---
		assert.False(t, have)
	})
}

// setBuildConfig stores a go:build target configuration block on rng the way
// gomake delivers it, so [Go.Build] reads it from the target's "modules" block.
// Each value in modules maps a module import path to its "package" and optional
// "names" overrides.
func setBuildConfig(t tester.T, rng *ring.Ring, modules map[string]any) {
	t.Helper()
	cfg := jsonkit.To(t, map[string]any{"modules": modules})
	rng.MetaSet(gomake.ConfigMetaKey, cfg)
}

func Test_Go_Build(t *testing.T) {
	t.Run("inject with name overrides", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		tim := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("cmd/project.go")
		rng.EnvSet(xdef.EnvImgCreated, tim.Format(time.RFC3339Nano))
		rng.EnvSet(gomake.CCIDEnvKey, "job-42")
		setBuildConfig(t, rng, map[string]any{
			"example.com/comp/project": map[string]any{
				"package": "example.com/comp/project",
				"names": map[string]any{
					xdef.VarBuildDate: "BuildDate",
					xdef.VarScmRev:    "ScmRev",
					xdef.VarScmHash:   "ScmHash",
					xdef.VarScmState:  "ScmWDState",
					xdef.VarCCID:      "CCTag",
				},
			},
		})

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "" +
			"#gomake INFO# go build -ldflags=" +
			"-X 'example.com/comp/project.BuildDate=2000-01-02T03:04:05Z' " +
			"-X 'example.com/comp/project.ScmRev=v1.2.3' " +
			"-X 'example.com/comp/project.ScmHash=%s' " +
			"-X 'example.com/comp/project.ScmWDState=clean' " +
			"-X 'example.com/comp/project.CCTag=job-42' " +
			"cmd/project.go\n"
		want = fmt.Sprintf(want, cm.Hash)
		assert.Equal(t, want, tst.Stderr())

		want = "" +
			"BuildDate: 2000-01-02T03:04:05Z\n" +
			"ScmRev: v1.2.3\n" +
			"ScmHash: %s\n" +
			"ScmWDState: clean\n" +
			"CCTag: job-42\n"
		want = fmt.Sprintf(want, cm.Hash)
		assert.Equal(t, want, prj.ExeStdout("./project"))
	})

	t.Run("inject with default names", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		cm := prj.GitInitAddAll("v2.0.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("cmd/project.go")
		rng.EnvSet(gomake.CCIDEnvKey, "job-7")
		setBuildConfig(t, rng, map[string]any{
			"example.com/comp/project": map[string]any{
				"package": "example.com/comp/project",
			},
		})

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		out := tst.Stderr()
		assert.Contain(t, "-X 'example.com/comp/project.buildDate=", out)
		assert.Contain(t, "-X 'example.com/comp/project.scmRev=v2.0.0'", out)
		assert.Contain(t, "-X 'example.com/comp/project.scmHash="+cm.Hash+"'", out)
		assert.Contain(t, "-X 'example.com/comp/project.scmState=clean'", out)
		assert.Contain(t, "-X 'example.com/comp/project.ccid=job-7'", out)
	})

	t.Run("no matching config injects nothing", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		prj.GitInitAddAll("v1.0.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("cmd/project.go")
		setBuildConfig(t, rng, map[string]any{
			"other.com/unrelated": map[string]any{"package": "other.com/x"},
		})

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "#gomake INFO# go build cmd/project.go\n", tst.Stderr())
	})

	t.Run("configured outside git repo uses placeholders", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("cmd/project.go")
		setBuildConfig(t, rng, map[string]any{
			"example.com/comp/project": map[string]any{
				"package": "example.com/comp/project",
				"names": map[string]any{
					xdef.VarScmRev:   "ScmRev",
					xdef.VarScmHash:  "ScmHash",
					xdef.VarScmState: "ScmWDState",
					xdef.VarCCID:     "CCTag",
				},
			},
		})

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		serr := tst.Stderr()
		assert.Contain(t, "ScmRev="+xdef.PhRev, serr)
		assert.Contain(t, "ScmHash="+xdef.PhHash, serr)

		out := prj.ExeStdout("./project")
		assert.Contain(t, "ScmRev: "+xdef.PhRev+"\n", out)
		assert.Contain(t, "ScmHash: "+xdef.PhHash+"\n", out)
		assert.Contain(t, "ScmWDState: "+xdef.PhUnknown+"\n", out)
		assert.Contain(t, "CCTag: "+xdef.PhUnknown+"\n", out)
	})

	t.Run("error - invalid config missing package", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		prj.GitInitAddAll("v1.0.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("cmd/project.go")
		setBuildConfig(t, rng, map[string]any{
			"example.com/comp/project": map[string]any{
				"names": map[string]any{xdef.VarScmRev: "ScmRev"},
			},
		})

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrConfig, err)
	})

	t.Run("error - invalid names config type", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		prj.GitInitAddAll("v1.0.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("cmd/project.go")
		setBuildConfig(t, rng, map[string]any{
			"example.com/comp/project": map[string]any{
				"package": "example.com/comp/project",
				"names":   map[string]any{xdef.VarBuildDate: 5},
			},
		})

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrType, err)
	})

	t.Run("error - invalid target config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("cmd/project.go")
		rng.MetaSet(gomake.ConfigMetaKey, []byte("{bad"))

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "target config", err)
	})

	t.Run("error - not a go module", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("cmd/project.go")

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrNoGoMod, err)
	})

	t.Run("error - invalid program", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstInvalidProgram, "project.go")
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("project.go")

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "main redeclared in this block", tst.Stderr())
		// The dispatcher renders the returned error; Build must not also
		// print it, or the failure is reported to stderr twice.
		assert.NotContain(t, "exit status", tst.Stderr())
	})

	t.Run("cross-compile with environment variables", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		prj.GitInitAddAll("v1.0.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("-o", "project-linux", "cmd/project.go")
		rng.EnvSet("GOOS", "linux")
		rng.EnvSet("GOARCH", "amd64")
		rng.EnvSet("CGO_ENABLED", "0")
		setBuildConfig(t, rng, map[string]any{
			"example.com/comp/project": map[string]any{
				"package": "example.com/comp/project",
			},
		})

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "#gomake INFO# go build -ldflags=", tst.Stderr())
		assert.Contain(t, "-o project-linux cmd/project.go", tst.Stderr())

		out := prj.ExeStdout("file", "project-linux")
		assert.Contain(t, "ELF 64-bit LSB", out)
		assert.Contain(t, "x86-64", out)
	})

	t.Run("cross-compile for windows", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith(tstBuildMain, "cmd", "project.go")
		prj.CreateFileWith(tstBuildVersion, "project.go")
		prj.GitInitAddAll("v1.0.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("-o", "project.exe", "cmd/project.go")
		rng.EnvSet("GOOS", "windows")
		rng.EnvSet("GOARCH", "amd64")
		rng.EnvSet("CGO_ENABLED", "0")
		setBuildConfig(t, rng, map[string]any{
			"example.com/comp/project": map[string]any{
				"package": "example.com/comp/project",
			},
		})

		// --- When ---
		err := Go{}.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "#gomake INFO# go build -ldflags=", tst.Stderr())
		assert.Contain(t, "-o project.exe cmd/project.go", tst.Stderr())

		out := prj.ExeStdout("file", "project.exe")
		assert.Contain(t, "PE32+ executable", out)
		assert.Contain(t, "x86-64", out)
		assert.Contain(t, "Windows", out)
	})
}

// tstBuildMain represents file used to test :go:build target.
var tstBuildMain = `package main

import (
	"fmt"

	"example.com/comp/project"
)

func main() {
	fmt.Printf("BuildDate: %s\n", project.BuildDate)
	fmt.Printf("ScmRev: %s\n", project.ScmRev)
	fmt.Printf("ScmHash: %s\n", project.ScmHash)
	fmt.Printf("ScmWDState: %s\n", project.ScmWDState)
	fmt.Printf("CCTag: %s\n", project.CCTag)
}
`

// tstBuildVersion represents file used to test :go:build target.
var tstBuildVersion = `package project

// Variables set by ldflags.
var (
	BuildDate  = "<not set>"
	ScmRev     = "<not set>"
	ScmHash    = "<not set>"
	ScmWDState = "<not set>"
	CCTag      = "<not set>"
)
`

// tstInvalidProgram represents an invalid program to test :go:build target.
var tstInvalidProgram = `package main

func main() {}
func main() {}
`

func Test_buildValues(t *testing.T) {
	t.Run("outside git repo uses placeholders", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		have := buildValues(ctx, rng)

		// --- Then ---
		assert.Equal(t, xdef.PhRev, have[xdef.VarScmRev])
		assert.Equal(t, xdef.PhHash, have[xdef.VarScmHash])
		assert.Equal(t, xdef.PhUnknown, have[xdef.VarScmState])
		assert.Equal(t, xdef.PhUnknown, have[xdef.VarCCID])
		assert.NotEmpty(t, have[xdef.VarBuildDate])
	})

	t.Run("git repo with environment overrides", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()
		tim := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
		rng.EnvSet(xdef.EnvImgCreated, tim.Format(time.RFC3339Nano))
		rng.EnvSet(gomake.CCIDEnvKey, "job-42")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()
		prj.Chdir()

		// --- When ---
		have := buildValues(ctx, rng)

		// --- Then ---
		assert.Equal(t, "2000-01-02T03:04:05Z", have[xdef.VarBuildDate])
		assert.Equal(t, "job-42", have[xdef.VarCCID])
		assert.Equal(t, "v1.2.3", have[xdef.VarScmRev])
		assert.Equal(t, cm.Hash, have[xdef.VarScmHash])
		assert.Equal(t, "clean", have[xdef.VarScmState])
	})
}

func Test_gitOr_tabular(t *testing.T) {
	tt := []struct {
		testN string

		s    string
		err  error
		ph   string
		want string
	}{
		{"error returns placeholder", "value", errors.New("x"), "ph", "ph"},
		{"empty string returns placeholder", "", nil, "ph", "ph"},
		{"value returned when present", "value", nil, "ph", "value"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := gitOr(tc.s, tc.err, tc.ph)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
