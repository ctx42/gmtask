// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmgo"
)

func Test_WithSetupDockerRepo(t *testing.T) {
	// --- Given ---
	sup := &Setup{}

	// --- When ---
	WithSetupDockerRepo("repo")(sup)

	// --- Then ---
	assert.Equal(t, "repo", sup.repo)
}

func Test_WithSetupGitOrigin(t *testing.T) {
	// --- Given ---
	sup := &Setup{}

	// --- When ---
	WithSetupGitOrigin("remote")(sup)

	// --- Then ---
	assert.Equal(t, "remote", sup.origin)
}

func Test_WithSetupGoModule(t *testing.T) {
	// --- Given ---
	sup := &Setup{}

	// --- When ---
	WithSetupGoModule("module")(sup)

	// --- Then ---
	assert.Equal(t, "module", sup.module)
}

func Test_WithSetupMkdir(t *testing.T) {
	// --- Given ---
	sup := &Setup{}

	// --- When ---
	WithSetupMkdir(true)(sup)

	// --- Then ---
	assert.True(t, sup.mkdir)
}

func Test_WithSetupForce(t *testing.T) {
	// --- Given ---
	sup := &Setup{}

	// --- When ---
	WithSetupForce(true)(sup)

	// --- Then ---
	assert.True(t, sup.force)
}

func Test_NewSetup(t *testing.T) {
	t.Run("current working directory", func(t *testing.T) {
		// --- When ---
		sup, err := NewSetup("")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, oskit.Getwd(t), sup.root)
	})

	t.Run("only project root dir set", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewNamedProject(t, "acme")
		prj.Close()
		prj.Chdir()

		// --- When ---
		sup, err := NewSetup(prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), sup.root)
		assert.Empty(t, sup.origin)
		assert.Equal(t, "acme", sup.name)
		assert.Equal(t, "acme", sup.module)
	})

	t.Run("relative root", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Chdir(t, dir)

		// --- When ---
		sup, err := NewSetup(filepath.Join("sub", "proj"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(oskit.Getwd(t), "sub", "proj"), sup.root)
		assert.Equal(t, "proj", sup.module)
		assert.Equal(t, "proj", sup.name)
	})

	t.Run("module with major version suffix", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewNamedProject(t, "acme")
		prj.Close()

		// --- When ---
		sup, err := NewSetup(
			prj.Root(),
			WithSetupGoModule("github.com/prj/repo/v2"),
			WithSetupGitOrigin("git@github.com:prj/repo.git"),
		)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "github.com/prj/repo/v2", sup.module)
		assert.Equal(t, "repo", sup.name)
	})

	t.Run("only module set", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewNamedProject(t, "acme")
		prj.Close()
		prj.Chdir()

		module := "example.com/comp/acme"

		// --- When ---
		sup, err := NewSetup(prj.Root(), WithSetupGoModule(module))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), sup.root)
		assert.Empty(t, sup.origin)
		assert.Equal(t, "example.com/comp/acme", sup.module)
		assert.Equal(t, "acme", sup.name)
	})

	t.Run("only remote set", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewNamedProject(t, "acme")
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"

		// --- When ---
		sup, err := NewSetup(prj.Root(), WithSetupGitOrigin(origin))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), sup.root)
		assert.Equal(t, origin, sup.origin)
		assert.Equal(t, "example.com/comp/acme", sup.module)
		assert.Equal(t, "acme", sup.name)
	})

	t.Run("mkdir names the root after the project", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewNamedProject(t, "work")
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"
		opts := []func(*Setup){
			WithSetupGitOrigin(origin),
			WithSetupMkdir(true),
		}

		// --- When ---
		sup, err := NewSetup("", opts...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(prj.Root(), "acme"), sup.root)
		assert.Equal(t, "acme", sup.name)
	})

	t.Run("mkdir with an explicit root uses it", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewNamedProject(t, "work")
		prj.Close()
		prj.Chdir()

		opts := []func(*Setup){
			WithSetupGoModule("example.com/comp/acme"),
			WithSetupMkdir(true),
		}

		// --- When ---
		sup, err := NewSetup(prj.Root(), opts...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), sup.root)
	})

	t.Run("error - mkdir without origin or module", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewNamedProject(t, "work")
		prj.Close()
		prj.Chdir()

		// --- When ---
		sup, err := NewSetup("", WithSetupMkdir(true))

		// --- Then ---
		assert.ErrorIs(t, ErrMkdirNeedsName, err)
		assert.Nil(t, sup)
	})

	t.Run("module does not match origin error", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewNamedProject(t, "acme")
		prj.Close()
		prj.Chdir()

		origin := "git@example.org:proj/acme.git"
		module := "example.com/comp/acme"
		opts := []func(*Setup){
			WithSetupGitOrigin(origin),
			WithSetupGoModule(module),
		}

		// --- When ---
		sup, err := NewSetup(prj.Root(), opts...)

		// --- Then ---
		assert.ErrorIs(t, ErrModuleOriginMismatch, err)
		assert.Nil(t, sup)
	})
}

func Test_checkSetupRoot(t *testing.T) {
	t.Run("missing path is fine", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "not_existing")

		// --- When ---
		err := checkSetupRoot(pth, false)

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("empty directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		err := checkSetupRoot(dir, false)

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("force takes a non-empty directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, []byte("content"), dir, "file0.txt")

		// --- When ---
		err := checkSetupRoot(dir, true)

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - non-empty directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, []byte("content"), dir, "file0.txt")

		// --- When ---
		err := checkSetupRoot(dir, false)

		// --- Then ---
		assert.ErrorIs(t, ErrDirNotEmpty, err)
		assert.ErrorContain(t, "use --force to set up anyway", err)
	})

	t.Run("error - a file is never a root even with force", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Write(t, []byte("content"), t.TempDir(), "file0.txt")

		// --- When ---
		err := checkSetupRoot(pth, true)

		// --- Then ---
		assert.ErrorIs(t, ErrNotDir, err)
	})
}

func Test_Setup_vars(t *testing.T) {
	// --- Given ---
	sup := &Setup{
		name:   "acme-app",
		module: "example.com/acme/acme-app",
		origin: "git@example.com:acme/acme-app.git",
		repo:   "registry.example.com",
	}

	// --- When ---
	have := sup.vars()

	// --- Then ---
	assert.Equal(t, "acme-app", have.ProjectName)
	assert.Equal(t, "example.com/acme/acme-app", have.Module)
	assert.Equal(t, "app", have.Package)
	assert.Equal(t, "git@example.com:acme/acme-app.git", have.Origin)
	assert.Equal(t, "registry.example.com", have.Repo)
}

func Test_Setup_Setup(t *testing.T) {
	t.Run("in current working directory", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup(""))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		assert.Contain(t, "module project\n", prj.ReadFileStr("go.mod"))
		have := prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="project"`, have)
	})

	t.Run("in given relative directory", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateDir("xyz")
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup("xyz"))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		assert.Contain(t, "module xyz\n", prj.ReadFileStr("xyz", "go.mod"))
		have := prj.ReadFileStr("xyz", "dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="xyz"`, have)
	})

	t.Run("initializes module in subdir when cwd has go.mod", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateDir("xyz")
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup("xyz"))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		assert.Contain(t, "module xyz\n", prj.ReadFileStr("xyz", "go.mod"))
	})

	t.Run("based on git origin", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		origin := "git@example.com:comp/acme.git"
		sup := must.Value(NewSetup("", WithSetupGitOrigin(origin)))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		have := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", have)
		assert.Contain(t, origin, prj.ReadFileStr(".git", "config"))
		have = prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="acme"`, have)
	})

	t.Run("based on module", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		module := "example.com/comp/acme"
		sup := must.Value(NewSetup("", WithSetupGoModule(module)))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())
		have := prj.ReadFileStr("go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", have)
		assert.NotContain(t, "[remote ", prj.ReadFileStr(".git", "config"))
		have = prj.ReadFileStr("dev", "idea", "go-test-all.run.xml")
		assert.Contain(t, `name="acme"`, have)
	})

	t.Run("error - no structure configured", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup(""))
		rng := tst.Ring()

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoStructure, err)
		assert.Equal(t, "", tst.Stdout())
		assert.False(t, oskit.PathExists(t, prj.Path("dev")))
		assert.False(t, oskit.PathExists(t, prj.Path("go.mod")))
	})

	t.Run("error creating go module", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		module := "example.com:proj/my-repo" // Invalid.
		sup := must.Value(NewSetup("", WithSetupGoModule(module)))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmgo.ErrModInit, err)
		assert.ErrorContain(t, "example.com:proj/my-repo", err)
		assert.NotEmpty(t, tst.Stdout())
	})

	t.Run("error - directory is not empty", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup("", WithSetupGoModule("project")))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrDirNotEmpty, err)
		assert.NoFileExist(t, prj.Path("go.mod"))
	})

	t.Run("force commits existing files on develop", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Close()
		prj.Chdir()

		opts := []func(*Setup){
			WithSetupGoModule("project"),
			WithSetupForce(true),
		}
		sup := must.Value(NewSetup("", opts...))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())

		head := prj.ExeStdout("git", "symbolic-ref", "--short", "HEAD")
		assert.Equal(t, "develop\n", head)
		assert.Equal(t, "", prj.ExeStdout("git", "ls-tree", "-r", "master"))
		files := prj.ExeStdout("git", "ls-tree", "-r", "--name-only", "develop")
		assert.Contain(t, "file0.txt\n", files)
		assert.Contain(t, "go.mod\n", files)
		assert.Equal(t, "", prj.ExeStdout("git", "status", "--porcelain"))
	})

	t.Run("mkdir creates the project root", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		opts := []func(*Setup){
			WithSetupGoModule("example.com/comp/acme"),
			WithSetupMkdir(true),
		}
		sup := must.Value(NewSetup("", opts...))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "done\n", tst.Stdout())

		have := prj.ReadFileStr("acme", "go.mod")
		assert.Contain(t, "module example.com/comp/acme\n", have)
	})

	t.Run("error - no structure leaves the root uncreated", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		opts := []func(*Setup){
			WithSetupGoModule("example.com/comp/acme"),
			WithSetupMkdir(true),
		}
		sup := must.Value(NewSetup("", opts...))
		rng := tst.Ring()

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoStructure, err)
		assert.NoDirExist(t, prj.Path("acme"))
	})

	t.Run("go.mod file already exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup(
			"",
			WithSetupGoModule("project"),
			WithSetupForce(true),
		))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotContain(t, "module project initialized", tst.Stdout())
		assert.Contain(t, "git: repository initialized", tst.Stdout())
		assert.Contain(t, "done\n", tst.Stdout())
	})

	t.Run("already git repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Exe("git", "init")
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup(
			"",
			WithSetupGoModule("project"),
			WithSetupForce(true),
		))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "module \"project\" initialized", tst.Stdout())
		assert.NotContain(t, "git: repository initialized", tst.Stdout())
		assert.Contain(t, "done\n", tst.Stdout())
	})

	t.Run("error - IsRepo cancelled", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		// go.mod present so InitModule is skipped and IsRepo is the first
		// ctx-aware call after structure materialization.
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup(
			"",
			WithSetupGoModule("project"),
			WithSetupForce(true),
		))
		rng := tst.Ring()
		setStructure(t, rng)

		// --- When ---
		err := sup.Setup(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, context.Canceled, err)
		assert.NotContain(t, "done\n", tst.Stdout())
	})
}

func Test_Setup_addRegRepo(t *testing.T) {
	t.Run("appends registry repo", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFile("configs", CfgFile)
		prj.Close()

		opts := []func(*Setup){WithSetupDockerRepo("my.nexus.dev:5000/repo")}
		sup := must.Value(NewSetup(prj.Root(), opts...))

		// --- When ---
		err := sup.addRegRepo(rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "C42_REG_REPO=my.nexus.dev:5000/repo\n"
		assert.Equal(t, want, prj.ReadFileStr(CfgPath))
		want = "added C42_REG_REPO to: " + prj.Path(CfgPath) + "\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("last line without newline", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("A=1", "configs", CfgFile)
		prj.Close()

		opts := []func(*Setup){WithSetupDockerRepo("repo")}
		sup := must.Value(NewSetup(prj.Root(), opts...))

		// --- When ---
		err := sup.addRegRepo(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "A=1\nC42_REG_REPO=repo\n", prj.ReadFileStr(CfgPath))
		assert.Contain(t, "added C42_REG_REPO", tst.Stdout())
	})

	t.Run("already set", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("C42_REG_REPO=old\n", "configs", CfgFile)
		prj.Close()

		opts := []func(*Setup){WithSetupDockerRepo("repo")}
		sup := must.Value(NewSetup(prj.Root(), opts...))

		// --- When ---
		err := sup.addRegRepo(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "C42_REG_REPO=old\n", prj.ReadFileStr(CfgPath))
	})

	t.Run("config directory missing", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		opts := []func(*Setup){WithSetupDockerRepo("repo")}
		sup := must.Value(NewSetup(prj.Root(), opts...))

		// --- When ---
		err := sup.addRegRepo(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "C42_REG_REPO=repo\n", prj.ReadFileStr(CfgPath))
		assert.Contain(t, "added C42_REG_REPO", tst.Stdout())
	})

	t.Run("no-op when no repo", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		sup := must.Value(NewSetup(prj.Root()))

		// --- When ---
		err := sup.addRegRepo(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.NoFileExist(t, prj.Path(CfgPath))
	})

	t.Run("error - config directory is a file", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFile("configs")
		prj.Close()

		opts := []func(*Setup){WithSetupDockerRepo("repo")}
		sup := must.Value(NewSetup(prj.Root(), opts...))

		// --- When ---
		err := sup.addRegRepo(rng)

		// --- Then ---
		assert.ErrorIs(t, syscall.ENOTDIR, err)
		assert.ErrorContain(t, "create config directory", err)
	})
}

func Test_Setup_initScmRepo(t *testing.T) {
	t.Run("without remote", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("", "README.md")
		prj.Close()

		sup := must.Value(NewSetup(prj.Root()))
		rng := tst.Ring()

		// --- When ---
		err := sup.initScmRepo(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"git: repository initialized\n" +
			"git: empty initial commit made\n" +
			"git: master tagged with v0.0.0\n" +
			"git: branch develop created\n" +
			"git: all files added\n" +
			"git: project files committed\n"
		assert.Equal(t, want, tst.Stdout())

		gitLog := prj.ExeStdout("git", "log", "--format=%s%d", "--name-only")
		want = "" +
			"chore: scaffold project (HEAD -> develop)\n" +
			"\n" +
			"README.md\n" +
			"Initial commit. (tag: v0.0.0, master)\n"
		assert.Equal(t, want, gitLog)
		assert.Equal(t, "", prj.ExeStdout("git", "remote"))
	})

	t.Run("with remote", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("", "README.md")
		prj.Close()

		opt := WithSetupGitOrigin(prjkit.GitSSHOrigin)
		sup := must.Value(NewSetup(prj.Root(), opt))
		rng := tst.Ring()

		// --- When ---
		err := sup.initScmRepo(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"git: repository initialized\n" +
			"git: remote origin added\n" +
			"git: empty initial commit made\n" +
			"git: master tagged with v0.0.0\n" +
			"git: branch develop created\n" +
			"git: all files added\n" +
			"git: project files committed\n"
		assert.Equal(t, want, tst.Stdout())

		gitLog := prj.ExeStdout("git", "log", "--format=%s%d", "--name-only")
		want = "" +
			"chore: scaffold project (HEAD -> develop)\n" +
			"\n" +
			"README.md\n" +
			"Initial commit. (tag: v0.0.0, master)\n"
		assert.Equal(t, want, gitLog)
		origin := prj.ExeStdout("git", "remote", "get-url", "origin")
		assert.Equal(t, prjkit.GitSSHOrigin+"\n", origin)
	})

	t.Run("path not existing error", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		prjRoot := prj.Path("a", "b", "c")
		sup := must.Value(NewSetup(prjRoot))
		rng := tst.Ring()

		// --- When ---
		err := sup.initScmRepo(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
		assert.ErrorContain(t, "git init: ", err)
	})
}
