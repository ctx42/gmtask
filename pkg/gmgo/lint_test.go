package gmgo

import (
	"context"
	"io/fs"
	"path/filepath"
	"regexp"
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
		tst := ringtest.New(t).WetStdout()
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
	})

	t.Run("force config download", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()
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
	})

	t.Run("error - lint reports issue", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()
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
	})
}

func Test_Lint_checkVersion(t *testing.T) {
	t.Run("gets the current version", func(t *testing.T) {
		// --- Given ---
		want := exekit.New(t).ExeStdout("golangci-lint", "version")
		want = regexp.MustCompile("version (.*) built").
			FindAllStringSubmatch(want, 1)[0][1]

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
		err := Lint{}.lint(ctx, rng, prj.Root())

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
		err := Lint{}.lint(ctx, rng, prj.Root())

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
		tst := ringtest.New(t)
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
	})

	t.Run("download config file - tmp dir does not exist", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
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
	})

	t.Run("download config file - custom destination dir", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
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
	})

	t.Run("config file name from configuration", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
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
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--dir", "not_existing")

		// --- When ---
		err := Lint{}.Config(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "not_existing", err)
	})

	t.Run("config file already exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

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
