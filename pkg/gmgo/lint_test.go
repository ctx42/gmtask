// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/jsonkit"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/gmtask/internal/gmtest"
)

func Test_Lint_Default(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)

		// --- When ---
		err := Lint{}.Default(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, prj.Path("tmp", ".golangci.yml"))
		assert.Contain(t, "0 issues.", tst.Stdout())
		want := "#gomake INFO# lint config: downloading"
		assert.Contain(t, want, tst.Stderr())
	})

	t.Run("force config download", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)

		assert.NoError(t, Lint{}.Default(ctx, rng))
		rng.EnvSet(GoLintConfigForceEnvKey, "1")

		// --- When ---
		err := Lint{}.Default(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, prj.Path("tmp", ".golangci.yml"))
		assert.Contain(t, "0 issues.", tst.Stdout())
		want := "#gomake INFO# lint config: downloading"
		assert.Contain(t, want, tst.Stderr())
	})

	t.Run("error - lint reports issue", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/failure/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)

		// --- When ---
		err := Lint{}.Default(ctx, rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		want := "source.go:10:21: SA5009: Printf format %d has arg #1"
		assert.Contain(t, want, tst.Stdout())
		want = "#gomake INFO# lint config: downloading"
		assert.Contain(t, want, tst.Stderr())
	})

	t.Run("uses config from custom dir", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--dir", "out")

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		bad := "version: \"2\"\nlinters:\n  enable: [no-such-linter]\n"
		prj.CreateFileWith(bad, "out", ".golangci.yml")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Lint{}.Default(t.Context(), rng)

		// --- Then ---
		assert.Error(t, err)
		want := "#gomake INFO# lint config: using out/.golangci.yml"
		assert.Contain(t, want, tst.Stderr())
		assert.Contain(t, "no-such-linter", tst.Stderr())
	})

	t.Run("help does not lint", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("-h")

		// --- When ---
		err := Lint{}.Default(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Usage of :go:lint:config", tst.Stderr())
	})

	t.Run("auto-installs when binary is missing", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		repo := setupConfigRepo(t)
		bin := t.TempDir()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		// exec.Command resolves the binary via the process PATH, not cmd.Env,
		// so strip golangci-lint from the process PATH. GOBIN/bin is first so
		// the post-install re-check finds the freshly installed binary.
		path := bin + string(os.PathListSeparator) +
			pathWithoutBinary("golangci-lint")
		t.Setenv("PATH", path)

		rng := tst.Ring()
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)
		rng.EnvSet("GOBIN", bin)
		rng.EnvSet("PATH", path)

		// --- When ---
		err := Lint{}.Default(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, filepath.Join(bin, "golangci-lint"))
		assert.FileExist(t, prj.Path("tmp", ".golangci.yml"))
		assert.Contain(t, "0 issues.", tst.Stdout())
		want := "#gomake INFO# lint config: downloading"
		assert.Contain(t, want, tst.Stderr())
	})
}

func Test_Lint_checkVersion(t *testing.T) {
	t.Run("gets the current version", func(t *testing.T) {
		// --- Given ---
		out := exekit.New(t).ExeStdout("golangci-lint", "version")
		want := must.Value(extractGolangCiVersion(out)).Original()

		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		ver, err := Lint{}.checkVersion(ctx, rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, want, ver.Original())
	})

	t.Run("binary in GOBIN off PATH", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		bin := t.TempDir()
		script := "" +
			"#!/bin/sh\n" +
			"echo 'golangci-lint has version 9.9.9 built with go1 from x'\n"
		oskit.Write(t, script, bin, "golangci-lint")
		must.Nil(os.Chmod(filepath.Join(bin, "golangci-lint"), 0o755))
		rng.EnvSet("PATH", pathWithoutBinary("golangci-lint"))
		rng.EnvSet("GOBIN", bin)

		// --- When ---
		have, err := Lint{}.checkVersion(t.Context(), rng, "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "9.9.9", have.Original())
	})

	t.Run("error - command fails", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		// --- When ---
		ver, err := Lint{}.checkVersion(ctx, rng, "/no/such/dir/gmgo-xyz")

		// --- Then ---
		assert.Error(t, err)
		assert.Nil(t, ver)
	})
}

func Test_requiredLintVer_tabular(t *testing.T) {
	tt := []struct {
		testN string

		cfgVer string
		want   string
	}{
		{"not configured", "", expLintVer.Original()},
		{"latest", "latest", expLintVer.Original()},
		{"not a version", "master", expLintVer.Original()},
		{"newer pinned", "v9.0.0", "v9.0.0"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, err := requiredLintVer(tc.cfgVer)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have.Original())
		})
	}
}

func Test_requiredLintVer(t *testing.T) {
	t.Run("error - older than required", func(t *testing.T) {
		// --- When ---
		have, err := requiredLintVer("v2.0.0")

		// --- Then ---
		want := "configured golangci-lint v2.0.0 is older than the required"
		assert.ErrorContain(t, want, err)
		assert.Nil(t, have)
	})
}

func Test_Lint_lint(t *testing.T) {
	t.Run("success", func(t *testing.T) {
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
		err := Lint{}.lint(ctx, rng, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "0 issues.", tst.Stdout())
	})

	t.Run("error - lint reports issue", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/failure/project")
		prj.CreateFileWith("version: \"2\"\n", "tmp", ".golangci.yml")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Lint{}.lint(ctx, rng, prj.Root(), "")

		// --- Then ---
		assert.ExitCode(t, 1, err)
		want := "source.go:10:22: printf: fmt.Sprintf format"
		assert.Contain(t, want, tst.Stdout())
	})

	t.Run("resolves config under dir without chdir", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/failure/project")
		// Config lives only under the project; process CWD is not the project.
		prj.CreateFileWith("version: \"2\"\n", "tmp", ".golangci.yml")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := Lint{}.lint(ctx, rng, prj.Root(), "")

		// --- Then ---
		assert.ExitCode(t, 1, err)
		want := "source.go:10:22: printf: fmt.Sprintf format"
		assert.Contain(t, want, tst.Stdout())
	})
}

func Test_Lint_Install(t *testing.T) {
	t.Run("installs the latest release by default", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		bin := t.TempDir()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet("GOBIN", bin)
		// "go install" reports each module it fetches on stderr, so what it
		// writes there depends on the module cache, not on the target.
		rng.SetStderr(&bytes.Buffer{})

		// --- When ---
		err := Lint{}.Install(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		exe := filepath.Join(bin, "golangci-lint")
		out := exekit.New(t).ExeStdout(exe, "version")

		have := must.Value(extractGolangCiVersion(out))
		assert.True(t, have.Equal(expLintVer) || have.GreaterThan(expLintVer))
	})

	t.Run("installs the version from the configuration", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		bin := t.TempDir()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// A release older than "@latest", so a target that ignored the
		// configuration would install a newer version and fail the assertion.
		want := semver.MustParse("v2.12.1")
		rng := tst.Ring()
		rng.EnvSet("GOBIN", bin)
		// "go install" reports each module it fetches on stderr, so what it
		// writes there depends on the module cache, not on the target.
		rng.SetStderr(&bytes.Buffer{})
		// The delivered block is the whole go.lint node ({version, file});
		// Install must use "version" and ignore the sibling "file" key.
		cfg := jsonkit.To(t, map[string]any{
			"version": want.Original(),
			"file":    ".golangci.yml",
		})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Lint{}.Install(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		exe := filepath.Join(bin, "golangci-lint")
		out := exekit.New(t).ExeStdout(exe, "version")

		have := must.Value(extractGolangCiVersion(out))
		assert.True(t, have.Equal(want))
	})

	t.Run("error - install fails", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvSet("GOPROXY", "off")
		rng.EnvSet("GOFLAGS", "-mod=mod")
		rng.EnvSet("GOBIN", t.TempDir())
		cfg := jsonkit.To(t, map[string]any{"version": "v0.0.0-no.such"})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Lint{}.Install(t.Context(), rng)

		// --- Then ---
		assert.ExitCode(t, 1, err)
		assert.Contain(t, "v0.0.0-no.such", tst.Stderr())
	})

	t.Run("error - config type mismatch", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		cfg := jsonkit.To(t, map[string]any{"version": 5})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Lint{}.Install(ctx, rng)

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

		rng := tst.Ring()
		rng.MetaSet(gomake.ConfigMetaKey, []byte("{bad"))

		// --- When ---
		err := Lint{}.Install(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "target config", err)
	})
}

func Test_Lint_Config(t *testing.T) {
	t.Run("download config file - tmp dir exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t)
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		pth := prj.Path("tmp", ".golangci.yml")
		assert.FileExist(t, pth)
		assert.True(t, oskit.FileSize(t, pth) > 0)

		want := "#gomake INFO# lint config: downloading from " + repo +
			" to tmp/.golangci.yml\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("download config file - tmp dir does not exist", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		pth := prj.Path(".golangci.yml")
		assert.FileExist(t, pth)
		assert.True(t, oskit.FileSize(t, pth) > 0)

		want := "#gomake INFO# lint config: downloading from " + repo +
			" to .golangci.yml\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("download config file - custom destination dir", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		tmp := t.TempDir()
		rng := tst.Ring("--dir", tmp)
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		pth := filepath.Join(tmp, ".golangci.yml")
		assert.FileExist(t, pth)
		assert.True(t, oskit.FileSize(t, pth) > 0)

		want := "#gomake INFO# lint config: downloading from " + repo +
			" to " + pth + "\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("config file name from configuration", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		repo := setupConfigRepo(t, "custom.yml")

		prj := gmtest.NewProject(t)
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)
		// The delivered block is the whole go.lint node ({version, file});
		// Config must use "file" and ignore the sibling "version" key.
		cfg := jsonkit.To(t, map[string]any{
			"file":    "custom.yml",
			"version": "v2.13.0",
		})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		pth := prj.Path("tmp", "custom.yml")
		assert.FileExist(t, pth)
		assert.True(t, oskit.FileSize(t, pth) > 0)

		want := "#gomake INFO# lint config: downloading from " + repo +
			" to tmp/custom.yml\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("source repo from configuration", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t)
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		cfg := jsonkit.To(t, map[string]any{"repo": repo})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		pth := prj.Path("tmp", ".golangci.yml")
		assert.FileExist(t, pth)
		assert.True(t, oskit.FileSize(t, pth) > 0)

		want := "#gomake INFO# lint config: downloading from " + repo +
			" to tmp/.golangci.yml\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("source repo on main branch", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		repo := setupConfigRepo(t)
		cmd := exec.Command("git", "-C", repo, "branch", "-m", "master", "main")
		must.Value(cmd.CombinedOutput())
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Lint{}.Config(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, prj.Path(".golangci.yml"))
		assert.Contain(t, "lint config: downloading", tst.Stderr())
	})

	t.Run("env source repo overrides configuration", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		repo := setupConfigRepo(t)

		prj := gmtest.NewProject(t)
		prj.CreateDir("tmp")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		// The env repo is a working clone; the config repo does not exist, so
		// a successful download proves the env value wins over the config.
		cfg := jsonkit.To(t, map[string]any{"repo": "git@example.com:no/op.git"})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)
		rng.EnvSet(GoLintConfigRepoEnvKey, repo)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		pth := prj.Path("tmp", ".golangci.yml")
		assert.FileExist(t, pth)
		assert.True(t, oskit.FileSize(t, pth) > 0)

		want := "#gomake INFO# lint config: downloading from " + repo +
			" to tmp/.golangci.yml\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - config type mismatch", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		cfg := jsonkit.To(t, map[string]any{"file": 5})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gomake.ErrType, err)
	})

	t.Run("error - repo config type mismatch", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		cfg := jsonkit.To(t, map[string]any{"repo": 5})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

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

		rng := tst.Ring()
		rng.MetaSet(gomake.ConfigMetaKey, []byte("{bad"))

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "target config", err)
	})

	t.Run("error - config checked before arguments", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// A bad config and a bad argument at once: the config is read first,
		// so its error wins and argument parsing never runs.
		rng := tst.Ring("-unknown")
		cfg := jsonkit.To(t, map[string]any{"file": 5})
		rng.MetaSet(gomake.ConfigMetaKey, cfg)

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		// ErrType (not the "-unknown" flag error) proves the config is read
		// before arguments are parsed.
		assert.ErrorIs(t, gomake.ErrType, err)
	})

	t.Run("error - custom dir does not exist", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--dir", "not_existing")

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "not_existing", err)

		want := "#gomake INFO# lint config: downloading from " +
			"git@github.com:ctx42/xdev.git to not_existing/.golangci.yml\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("config file already exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("abc", "tmp", ".golangci.yml")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "abc", prj.ReadFileStr("tmp", ".golangci.yml"))

		want := "#gomake INFO# lint config: using tmp/.golangci.yml\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("force set to false", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvSet(GoLintConfigForceEnvKey, "false")
		rng.EnvSet(GoLintConfigRepoEnvKey, filepath.Join(t.TempDir(), "none"))

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("version: \"2\"\n", ".golangci.yml")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Lint{}.Config(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "#gomake INFO# lint config: using .golangci.yml\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - invalid force value", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet(GoLintConfigForceEnvKey, "maybe")

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Lint{}.Config(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, strconv.ErrSyntax, err)
		want := "invalid GOMAKE_GOLINT_CONFIG_FORCE: \"maybe\""
		assert.ErrorContain(t, want, err)
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
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :go:lint:config\n" +
			"      --dir     directory to put lint config to (default: " +
			"\".\" or \"tmp\" if exists)\n" +
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
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :go:lint:config\n" +
			"      --dir     directory to put lint config to (default: " +
			"\".\" or \"tmp\" if exists)\n" +
			"  -h, --help    show help\n"
		assert.Equal(t, want, tst.Stderr())
	})
}
