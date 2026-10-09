// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"os"
	"testing"

	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmgo"
)

// TestMain gives git an author identity and isolates the tests from the host.
// [Setup] commits into a repository it initializes itself, so no fixture
// configures one, and a CI runner has no global identity to fall back on. The
// host's git configuration (commit signing, for one) and the build variables
// it may export would otherwise change what the tests see.
func TestMain(m *testing.M) {
	env := map[string]string{
		"GIT_AUTHOR_NAME":     "Test User",
		"GIT_AUTHOR_EMAIL":    "test@example.com",
		"GIT_COMMITTER_NAME":  "Test User",
		"GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
	}
	for key, val := range env {
		if err := os.Setenv(key, val); err != nil {
			panic(err)
		}
	}
	for _, key := range []string{gmgo.EnvBldBump, xdef.EnvBldDate} {
		if err := os.Unsetenv(key); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

func Test_Project_Env(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

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
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("--export")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"export C42_BLD_DATE='2000-01-02T03:04:05.600Z'\n" +
			"export C42_PRJ_NAME='project'\n" +
			"export C42_SCM_STATE='no-scm'\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("export quotes values", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("--export")
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.CfgAdd("DESC", "it's a b")
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "export DESC='it'\\''s a b'\n"
		assert.Contain(t, want, tst.Stdout())
	})

	t.Run("all environment variables set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.WithConfig()
		cm := prj.GitInitAddAll("v0.1.0")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()
		prj.Chdir()

		rng.EnvSet(EnvSSHAuthSock, "socket")
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			xdef.EnvBldDate + "=2000-01-02T03:04:05.600Z" + "\n" +
			xdef.EnvPrjName + "=project" + "\n" +
			xdef.EnvScmHash + "=" + cm.Hash + "\n" +
			xdef.EnvScmRepo + "=" + prjkit.GitOrigin + "\n" +
			xdef.EnvScmRev + "=v0.1.0" + "\n" +
			xdef.EnvScmState + "=" + gitaid.StateClean + "\n" +
			EnvSSHAuthSock + "=socket" + "\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("print specific variable", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring(xdef.EnvScmState)

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, ScmNo, tst.Stdout())
	})

	t.Run("single variable with export flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("-e", xdef.EnvScmState)

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, ScmNo, tst.Stdout())
	})

	t.Run("unknown variable", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("unknown")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tst.Stdout())
	})

	t.Run("error - too many args", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring(xdef.EnvScmState, xdef.EnvPrjName)

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrTooManyArgs, err)
	})

	t.Run("error - unknown flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--nope")

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -nope", err)
		assert.Contain(t, "Usage of :project:env:", tst.Stderr())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Env(t.Context(), rng)

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
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Info(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			xdef.EnvBldDate + "=2000-01-02T03:04:05.600Z",
			xdef.EnvPrjName + "=project",
			xdef.EnvScmState + "=" + ScmNo,
		}
		assert.Equal(t, want, toEnv(tst.Stdout()))
	})

	t.Run("all fields set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v0.1.0")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()
		prj.Chdir()

		rng.EnvSet(EnvSSHAuthSock, "socket")
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		err := Project{}.Info(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			xdef.EnvBldDate + "=2000-01-02T03:04:05.600Z",
			xdef.EnvPrjName + "=project",
			xdef.EnvScmHash + "=" + cm.Hash,
			xdef.EnvScmRepo + "=" + prjkit.GitOrigin,
			xdef.EnvScmRev + "=v0.1.0",
			xdef.EnvScmState + "=" + gitaid.StateClean,
			EnvSSHAuthSock + "=socket",
		}
		assert.Equal(t, want, toEnv(tst.Stdout()))
	})

	t.Run("print specific field value", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring(xdef.EnvScmState)

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Info(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, ScmNo, tst.Stdout())
	})

	t.Run("print unknown field value", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("unknown")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Info(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tst.Stdout())
	})

	t.Run("error - too many args", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring(xdef.EnvScmState, xdef.EnvPrjName)

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := Project{}.Info(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrTooManyArgs, err)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Project{}.Info(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :project:info:\n" +
			"  -h, --help    show help\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Info(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
	})
}

func Test_Project_Setup(t *testing.T) {
	t.Run("cwd with origin", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"
		rng := tst.Ring("--origin", origin)
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		mod := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", mod)
		assert.Contain(t, origin, prj.ReadFileStr(".git", "config"))
		runCfg := prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="acme"`, runCfg)
	})

	t.Run("cwd with module", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		module := "example.com/comp/my-repo"
		rng := tst.Ring("--module", module)
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		mod := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module "+module+"\n", mod)
		assert.NotContain(t, "[remote ", prj.ReadFileStr(".git", "config"))
		runCfg := prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="my-repo"`, runCfg)
	})

	t.Run("error - directory is not empty", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("--origin", "git@example.com:comp/acme.git")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Close()
		prj.Chdir()

		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrDirNotEmpty, err)
		assert.ErrorContain(t, "use --force to set up anyway", err)
		assert.NoFileExist(t, prj.Path("go.mod"))
		assert.NoDirExist(t, prj.Path(".git"))
	})

	t.Run("error - only dot entry", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("--origin", "git@example.com:comp/acme.git")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("ignored", ".hidden")
		prj.Close()
		prj.Chdir()

		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrDirNotEmpty, err)
	})

	t.Run("force non-empty directory", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"
		rng := tst.Ring("--force", "--origin", origin)
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		mod := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", mod)
		assert.Equal(t, "file0 1", prj.ReadFileStr("file0.txt"))
	})

	t.Run("create directory with git origin", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"
		rng := tst.Ring("--origin", origin, "--mkdir")
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		mod := prj.ReadFileStr("acme", "go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", mod)
		assert.Contain(t, origin, prj.ReadFileStr("acme", ".git", "config"))
		mod = prj.ReadFileStr("acme", "dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="acme"`, mod)
	})

	t.Run("create directory with module name", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		module := "example.com/comp/my-repo"
		rng := tst.Ring("--module", module, "--mkdir")
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		mod := prj.ReadFileStr("my-repo", "go.mod")
		assert.Contain(t, "module "+module+"\n", mod)
		gitCfg := prj.ReadFileStr("my-repo", ".git", "config")
		assert.NotContain(t, "[remote ", gitCfg)
		idea := prj.Path("my-repo", "dev", "idea")
		runCfg := oskit.ReadFileStr(t, idea, "go-test-all.run.xml")
		assert.Contain(t, `name="my-repo"`, runCfg)
	})

	t.Run("error - mkdir target exists", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("--module", "example.com/comp/my-repo", "--mkdir")

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()
		oskit.MkdirAll(t, prj.Root(), "my-repo")

		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorContain(t, "file exists", err)
	})

	t.Run("error - mkdir without origin or module", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("--mkdir")

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrMkdirNeedsName, err)
	})

	t.Run("error - positional argument", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring("myproj")

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrTooManyArgs, err)
		assert.ErrorContain(t, "[myproj]", err)
		assert.Equal(t, []string{}, oskit.Readdirnames(t, prj.Root()))
	})

	t.Run("default module name", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		assert.Contain(t, "module project\n", prj.ReadFileStr("go.mod"))
		assert.NotContain(t, "[remote ", prj.ReadFileStr(".git", "config"))
		runCfg := prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="project"`, runCfg)
	})

	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :project:setup:\n" +
			"  -f, --force     use the directory even if it has files\n" +
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

	t.Run("error - unknown flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :project:setup:\n" +
			"  -f, --force     use the directory even if it has files\n" +
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

	t.Run("error - mkdir existing directory", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("--origin", "git@example.com:comp/acme.git", "--mkdir")

		prj := gmtest.NewProject(t)
		prj.CreateDir("acme")
		prj.Close()
		prj.Chdir()

		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, os.ErrExist, err)
		assert.NoFileExist(t, prj.Path("acme", "go.mod"))
	})

	t.Run("force mkdir existing directory", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "acme", "file0.txt")
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"
		rng := tst.Ring("--origin", origin, "--mkdir", "--force")
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		mod := prj.ReadFileStr("acme", "go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", mod)
		assert.Equal(t, "file0 1", prj.ReadFileStr("acme", "file0.txt"))
	})

	t.Run("error - force mkdir onto a file", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("not a dir", "acme")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(
			"--origin", "git@example.com:comp/acme.git",
			"--mkdir",
			"--force",
		)
		setStructure(t, rng)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNotDir, err)
		assert.Equal(t, "not a dir", prj.ReadFileStr("acme"))
	})

	t.Run("error - mkdir check leaves nothing", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		origin := "git@example.org:proj/acme.git"
		module := "example.com/comp/acme"
		rng := tst.Ring("-o", origin, "-m", module, "--mkdir")

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrModuleOriginMismatch, err)
		assert.NoDirExist(t, prj.Path("acme"))
	})

	t.Run("error - origin and module mismatch", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		origin := "git@example.org:proj/acme.git"
		module := "example.com/comp/acme"
		rng := tst.Ring("-o", origin, "-m", module)

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrModuleOriginMismatch, err)
	})

	t.Run("error - no structure configured", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Project{}.Setup(t.Context(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoStructure, err)
		assert.Equal(t, "", tst.Stdout())
	})
}
