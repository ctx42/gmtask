// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"context"
	"fmt"
	"testing"

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
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"C42_BUILD_DATE=2000-01-02T03:04:05.6Z\n" +
			"C42_CCID=unknown\n" +
			"C42_PROJ_DIST_DIR=%s/dist\n" +
			"C42_PROJ_NAME=project\n" +
			"C42_PROJ_ROOT_DIR=%s\n" +
			"C42_SCM_STATE=no-scm\n"
		want = fmt.Sprintf(want, prj.Root(), prj.Root())
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
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"export C42_BUILD_DATE=2000-01-02T03:04:05.6Z\n" +
			"export C42_CCID=unknown\n" +
			"export C42_PROJ_DIST_DIR=%s/dist\n" +
			"export C42_PROJ_NAME=project\n" +
			"export C42_PROJ_ROOT_DIR=%s\n" +
			"export C42_SCM_STATE=no-scm\n"
		want = fmt.Sprintf(want, prj.Root(), prj.Root())
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
		rng.EnvSet(xdef.EnvCCID, "cicd-000")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			ev(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z") + "\n" +
			ev(xdef.EnvCCID, "cicd-000") + "\n" +
			ev(xdef.EnvProjDistDir, prj.Path("dist")) + "\n" +
			ev(xdef.EnvProjGoImpSpec, prjkit.GoModName) + "\n" +
			ev(xdef.EnvProjName, "project") + "\n" +
			ev(xdef.EnvProjRootDir, prj.Root()) + "\n" +
			ev(xdef.EnvScmHash, cm.Hash) + "\n" +
			ev(xdef.EnvScmRepo, prjkit.GitOrigin) + "\n" +
			ev(xdef.EnvScmRev, "v0.1.0") + "\n" +
			ev(xdef.EnvScmState, ScmClean) + "\n" +
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

		rng := tst.Ring(xdef.EnvProjDistDir)

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Path("dist"), tst.Stdout())
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

		rng := tst.Ring("-e", xdef.EnvProjDistDir)

		// --- When ---
		err := Project{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Path("dist"), tst.Stdout())
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

		rng := tst.Ring(xdef.EnvProjDistDir, xdef.EnvProjGoImpSpec)

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
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			ev(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z"),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
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
		rng.EnvSet(xdef.EnvCCID, "cicd-000")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			ev(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z"),
			ev(xdef.EnvCCID, "cicd-000"),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjGoImpSpec, prjkit.GoModName),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmHash, cm.Hash),
			ev(xdef.EnvScmRepo, prjkit.GitOrigin),
			ev(xdef.EnvScmRev, "v0.1.0"),
			ev(xdef.EnvScmState, ScmClean),
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

		rng := tst.Ring(xdef.EnvProjDistDir)

		// --- When ---
		err := Project{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Path("dist"), tst.Stdout())
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

		rng := tst.Ring(xdef.EnvProjDistDir, xdef.EnvProjGoImpSpec)

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
		assert.ErrorEqual(t, "go module name does not match git origin", err)
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
