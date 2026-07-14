// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"context"
	"os"
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
		assert.ErrorEqual(t, "go module name does not match git origin", err)
		assert.Nil(t, sup)
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
		prj.CreateDir("xyz")
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
		prj.CreateDir("xyz")
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

	t.Run("go.mod file already exists", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()
		prj.Chdir()

		sup := must.Value(NewSetup("", WithSetupGoModule("project")))
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

		sup := must.Value(NewSetup("", WithSetupGoModule("project")))
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
}

func Test_Setup_addImgSrc(t *testing.T) {
	t.Run("appends image source when repo set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateDir("configs")
		prj.CreateFile("configs", CfgFile)
		prj.Close()

		opts := []func(*Setup){WithSetupDockerRepo("my.nexus.dev:5000/repo")}
		sup := must.Value(NewSetup(prj.Root(), opts...))
		rng := tst.Ring()

		// --- When ---
		err := sup.addImgSrc(rng)

		// --- Then ---
		assert.NoError(t, err)
		have := prj.ReadFileStr(CfgPath)
		assert.Equal(t, "OCI_IMAGE_SOURCE=my.nexus.dev:5000/repo\n", have)
		assert.Contain(t, "added OCI_IMAGE_SOURCE to:", tst.Stdout())
	})

	t.Run("no-op when no repo", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		sup := must.Value(NewSetup(prj.Root()))
		rng := tst.Ring()

		// --- When ---
		err := sup.addImgSrc(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tst.Stdout())
	})

	t.Run("error - config file missing", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		opts := []func(*Setup){WithSetupDockerRepo("my.nexus.dev:5000/repo")}
		sup := must.Value(NewSetup(prj.Root(), opts...))
		rng := tst.Ring()

		// --- When ---
		err := sup.addImgSrc(rng)

		// --- Then ---
		var e *os.PathError
		assert.ErrorAs(t, &e, err)
	})
}

func Test_Setup_initScmRepo(t *testing.T) {
	t.Run("without remote", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("", "README.md")
		prj.Close()

		sup := must.Value(NewSetup(prj.Root()))
		rng := tst.Ring()

		// --- When ---
		err := sup.initScmRepo(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"git: repository initialized\n" +
			"git: all files added\n" +
			"git: initial commit made\n" +
			"git: repository tagged with v0.0.0\n"
		assert.Equal(t, want, tst.Stdout())

		gitLog := prj.ExeStdout("git", "--no-pager", "log", "--decorate=short", "--pretty=oneline", "-n1")
		assert.Contain(t, "tag: v0.0.0", gitLog)
		assert.Contain(t, "Initial commit.\n", gitLog)
	})

	t.Run("with remote", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("", "README.md")
		prj.Close()

		sup := must.Value(NewSetup(prj.Root(), WithSetupGitOrigin(prjkit.GitSSHOrigin)))
		rng := tst.Ring()

		// --- When ---
		err := sup.initScmRepo(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"git: repository initialized\n" +
			"git: all files added\n" +
			"git: remote origin added\n" +
			"git: initial commit made\n" +
			"git: repository tagged with v0.0.0\n"
		have := tst.Stdout()
		assert.Equal(t, want, have)

		gitLog := prj.ExeStdout("git", "--no-pager", "log", "--decorate=short", "--pretty=oneline", "-n1")
		assert.Contain(t, "tag: v0.0.0", gitLog)
		assert.Contain(t, "Initial commit.\n", gitLog)
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
		assert.True(t, os.IsNotExist(err))
	})
}
