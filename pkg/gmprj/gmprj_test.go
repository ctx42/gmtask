// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"context"
	"testing"

	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
)

func Test_Project_Env(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"C42_BLD_DATE=2000-01-02T03:04:05.600Z\n" +
			"C42_PRJ_NAME=project\n" +
			"C42_SCM_STATE=no-scm\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("minimal with export", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--export")
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"export C42_BLD_DATE=2000-01-02T03:04:05.600Z\n" +
			"export C42_PRJ_NAME=project\n" +
			"export C42_SCM_STATE=no-scm\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("all environment variables set", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.WithConfig()
		cm := prj.GitInitAddAll("v0.1.0")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(EnvSSHAuthSock, "socket")
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			ev(xdef.EnvBldDate, "2000-01-02T03:04:05.600Z") + "\n" +
			ev(xdef.EnvPrjName, "project") + "\n" +
			ev(xdef.EnvScmHash, cm.Hash) + "\n" +
			ev(xdef.EnvScmRepo, prjkit.GitOrigin) + "\n" +
			ev(xdef.EnvScmRev, "v0.1.0") + "\n" +
			ev(xdef.EnvScmState, gitaid.StateClean) + "\n" +
			ev(EnvSSHAuthSock, "socket") + "\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("print specific variable", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(xdef.EnvScmState)

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, ScmNo, tst.Stdout())
	})

	t.Run("single variable with export flag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("-e", xdef.EnvScmState)

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, ScmNo, tst.Stdout())
	})

	t.Run("print not existing environment variable", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("unknown")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tst.Stdout())
	})

	t.Run("too many args error", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring(xdef.EnvScmState, xdef.EnvPrjName)

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrTooManyArgs, err)
	})

	t.Run("unknown flag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring("--nope")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -nope", err)
		assert.Contain(t, "Usage of :project:env:", tst.Stderr())
	})

	t.Run("needs project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--help")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :project:env:\n" +
			"  -e, --export    export variables\n" +
			"  -h, --help      show help\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Project_Info(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			ev(xdef.EnvBldDate, "2000-01-02T03:04:05.600Z"),
			ev(xdef.EnvPrjName, "project"),
			ev(xdef.EnvScmState, ScmNo),
		}
		assert.Equal(t, want, toEnv(tst.Stdout()))
	})

	t.Run("all fields set", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v0.1.0")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		rng.EnvSet(EnvSSHAuthSock, "socket")
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			ev(xdef.EnvBldDate, "2000-01-02T03:04:05.600Z"),
			ev(xdef.EnvPrjName, "project"),
			ev(xdef.EnvScmHash, cm.Hash),
			ev(xdef.EnvScmRepo, prjkit.GitOrigin),
			ev(xdef.EnvScmRev, "v0.1.0"),
			ev(xdef.EnvScmState, gitaid.StateClean),
			ev(EnvSSHAuthSock, "socket"),
		}
		assert.Equal(t, want, toEnv(tst.Stdout()))
	})

	t.Run("print specific field value", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(xdef.EnvScmState)

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, ScmNo, tst.Stdout())
	})

	t.Run("print unknown field value", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("unknown")

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tst.Stdout())
	})

	t.Run("too many args error", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring(xdef.EnvScmState, xdef.EnvPrjName)

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrTooManyArgs, err)
	})

	t.Run("needs project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
	})
}

func Test_Project_Setup(t *testing.T) {
	t.Run("in current working directory with git origin", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"
		rng := tst.Ring("--origin", origin)
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		have := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", have)
		assert.Contain(t, origin, prj.ReadFileStr(".git", "config"))
		have = prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="acme"`, have)
	})

	t.Run("in current working directory with module name", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		module := "example.com/comp/my-repo"
		rng := tst.Ring("--module", module)
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		have := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module "+module+"\n", have)
		assert.NotContain(t, "[remote ", prj.ReadFileStr(".git", "config"))
		have = prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="my-repo"`, have)
	})

	t.Run("create directory with git origin", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"
		rng := tst.Ring("--origin", origin, "--mkdir")
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		have := prj.ReadFileStr("acme", "go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", have)
		assert.Contain(t, origin, prj.ReadFileStr("acme", ".git", "config"))
		have = prj.ReadFileStr("acme", "dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="acme"`, have)
	})

	t.Run("create directory with module name", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		module := "example.com/comp/my-repo"
		rng := tst.Ring("--module", module, "--mkdir")
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		have := prj.ReadFileStr("my-repo", "go.mod")
		assert.Contain(t, "module "+module+"\n", have)
		assert.NotContain(t, "[remote ", prj.ReadFileStr("my-repo", ".git", "config"))
		have = prj.ReadFileStr("my-repo", "dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="my-repo"`, have)
	})

	t.Run("mkdir target already exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()
		oskit.MkdirAll(t, prj.Root(), "my-repo")

		rng := tst.Ring("--module", "example.com/comp/my-repo", "--mkdir")

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "file exists", err)
	})

	t.Run("error - mkdir without origin or module", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--mkdir")

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrMkdirNeedsName, err)
	})

	t.Run("default module name", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		assert.Contain(t, "module project\n", prj.ReadFileStr("go.mod"))
		assert.NotContain(t, "[remote ", prj.ReadFileStr(".git", "config"))
		have := prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="project"`, have)
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
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :project:setup:\n" +
			"  -h, --help      show help\n" +
			"  -d, --mkdir     create project directory\n" +
			"  -m, --module    go module name\n" +
			"  -o, --origin    remote repository path\n" +
			"\nExamples:\n" +
			"  # Derive the module name from the git remote.\n" +
			"  gomake :project:setup --origin git@github.com:prj/repo.git\n" +
			"\n" +
			"  # Set the Go module path explicitly.\n" +
			"  gomake :project:setup --module github.com/prj/repo\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("unknown flag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--unknown")

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :project:setup:\n" +
			"  -h, --help      show help\n" +
			"  -d, --mkdir     create project directory\n" +
			"  -m, --module    go module name\n" +
			"  -o, --origin    remote repository path\n" +
			"\nExamples:\n" +
			"  # Derive the module name from the git remote.\n" +
			"  gomake :project:setup --origin git@github.com:prj/repo.git\n" +
			"\n" +
			"  # Set the Go module path explicitly.\n" +
			"  gomake :project:setup --module github.com/prj/repo\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("both origin and module set and incompatible", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		origin := "git@example.org:proj/acme.git"
		module := "example.com/comp/acme"
		rng := tst.Ring("-o", origin, "-m", module)

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrModuleOriginMismatch, err)
	})

	t.Run("setup error - no structure configured", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Project{}.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoStructure, err)
		assert.Equal(t, "", tst.Stdout())
	})
}
