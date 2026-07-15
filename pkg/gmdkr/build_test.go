// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmprj"
)

func Test_NewBuild(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.GitInitAddAll("v1.2.3")
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		cfg := ConfigFrom(inf, nil)

		// --- When ---
		bld, err := NewBuild(*cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotNil(t, bld)
	})

	t.Run("error - empty name", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "",
			tag:  "scm-rev",
		}

		// --- When ---
		bld, err := NewBuild(cfg)

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyName, err)
		assert.Nil(t, bld)
	})

	t.Run("error - empty tag", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "",
		}

		// --- When ---
		bld, err := NewBuild(cfg)

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyTag, err)
		assert.Nil(t, bld)
	})

	t.Run("nil config args", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "v1.2.3",
		}

		// --- When ---
		bld, err := NewBuild(cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotNil(t, bld.args)
	})
}

func Test_Build_ImgStem(t *testing.T) {
	t.Run("with private repo", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
			repo: "example.com/repo",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgStem()

		// --- Then ---
		assert.Equal(t, "example.com/repo/project", have)
	})

	t.Run("without private repo", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgStem()

		// --- Then ---
		assert.Equal(t, "project", have)
	})

	t.Run("with target", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name:   "project",
			tag:    "scm-rev",
			target: "target",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgStem()

		// --- Then ---
		assert.Equal(t, "project", have)
	})

	t.Run("with private repo and target", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name:   "project",
			tag:    "scm-rev",
			target: "target",
			repo:   "example.com/repo",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgStem()

		// --- Then ---
		assert.Equal(t, "example.com/repo/project", have)
	})
}

func Test_Build_ImgName(t *testing.T) {
	t.Run("without private repo", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgName()

		// --- Then ---
		assert.Equal(t, "project", have)
	})

	t.Run("with private repo", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
			repo: "example.com/repo",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgName()

		// --- Then ---
		assert.Equal(t, "example.com/repo/project", have)
	})

	t.Run("with target", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name:   "project",
			tag:    "scm-rev",
			target: "target",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgName()

		// --- Then ---
		assert.Equal(t, "project-target", have)
	})

	t.Run("with private repo and target", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name:   "project",
			tag:    "scm-rev",
			target: "target",
			repo:   "example.com/repo",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgName()

		// --- Then ---
		assert.Equal(t, "example.com/repo/project-target", have)
	})
}

func Test_Build_ImgRef(t *testing.T) {
	t.Run("with private repo", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
			repo: "example.com/repo",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgRef()

		// --- Then ---
		assert.Equal(t, "example.com/repo/project:scm-rev", have)
	})

	t.Run("without private repo", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgRef()

		// --- Then ---
		assert.Equal(t, "project:scm-rev", have)
	})

	t.Run("with target", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name:   "project",
			tag:    "scm-rev",
			target: "target",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgRef()

		// --- Then ---
		assert.Equal(t, "project-target:scm-rev", have)
	})
}

func Test_Build_ImgRefLatest(t *testing.T) {
	t.Run("with private repo", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
			repo: "example.com/repo",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgRefLatest()

		// --- Then ---
		assert.Equal(t, "example.com/repo/project:latest", have)
	})

	t.Run("without private repo", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgRefLatest()

		// --- Then ---
		assert.Equal(t, "project:latest", have)
	})

	t.Run("with target", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name:   "project",
			tag:    "scm-rev",
			target: "target",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgRefLatest()

		// --- Then ---
		assert.Equal(t, "project-target:latest", have)
	})
}

func Test_Build_ImgTag(t *testing.T) {
	t.Run("tag", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgTag()

		// --- Then ---
		assert.Equal(t, "scm-rev", have)
	})

	t.Run("with target", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name:   "project",
			tag:    "scm-rev",
			target: "target",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.ImgTag()

		// --- Then ---
		assert.Equal(t, "scm-rev", have)
	})
}

func Test_Build_Env(t *testing.T) {
	t.Run("with build kit", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
			kit:  true,
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.Env()

		// --- Then ---
		want := []string{"DOCKER_BUILDKIT=1"}
		assert.Equal(t, want, have)
	})

	t.Run("without build kit", func(t *testing.T) {
		// --- Given ---
		cfg := Config{
			name: "project",
			tag:  "scm-rev",
		}
		bld := must.Value(NewBuild(cfg))

		// --- When ---
		have := bld.Env()

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_Build_Cmd(t *testing.T) {
	t.Run("full example", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-socket")
		rng.EnvSet(xdef.EnvCCID, "cicd-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		bld := must.Value(NewBuild(*ConfigFrom(inf, nil)))

		// --- When ---
		have := bld.Cmd()

		// --- Then ---
		want := []string{
			"build",
			"--platform", "linux/amd64",
			"--ssh", "default=ssh-socket",
			"-t", "dki-project:v1.2.3",
			"-t", "dki-project:latest",
			"--build-arg", "C42_BUILD_DATE=2000-01-02T03:04:05.6Z",
			"--build-arg", "C42_CCID=cicd-tag",
			"--build-arg", "C42_SCM_HASH=" + cm.Hash,
			"--build-arg", "C42_SCM_REPO=" + prjkit.GitOrigin,
			"--build-arg", "C42_SCM_REV=v1.2.3",
			"--build-arg", "SSH_AUTH_SOCK=ssh-socket",
			"--file", "Dockerfile",
			".",
		}
		assert.Equal(t, want, have)
	})

	t.Run("full example without latest", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-socket")
		rng.EnvSet(xdef.EnvCCID, "cicd-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		bld := must.Value(NewBuild(*ConfigFrom(inf, nil)))
		bld.latest = false

		// --- When ---
		have := bld.Cmd()

		// --- Then ---
		want := []string{
			"build",
			"--platform", "linux/amd64",
			"--ssh", "default=ssh-socket",
			"-t", "dki-project:v1.2.3",
			"--build-arg", "C42_BUILD_DATE=2000-01-02T03:04:05.6Z",
			"--build-arg", "C42_CCID=cicd-tag",
			"--build-arg", "C42_SCM_HASH=" + cm.Hash,
			"--build-arg", "C42_SCM_REPO=" + prjkit.GitOrigin,
			"--build-arg", "C42_SCM_REV=v1.2.3",
			"--build-arg", "SSH_AUTH_SOCK=ssh-socket",
			"--file", "Dockerfile",
			".",
		}
		assert.Equal(t, want, have)
	})

	t.Run("full example with no cache", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-socket")
		rng.EnvSet(xdef.EnvCCID, "cicd-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		cfg := ConfigFrom(inf, nil)
		cfg.noCache = true
		bld := must.Value(NewBuild(*cfg))

		// --- When ---
		have := bld.Cmd()

		// --- Then ---
		want := []string{
			"build",
			"--platform", "linux/amd64",
			"--ssh", "default=ssh-socket",
			"-t", "dki-project:v1.2.3",
			"-t", "dki-project:latest",
			"--no-cache",
			"--build-arg", "C42_BUILD_DATE=2000-01-02T03:04:05.6Z",
			"--build-arg", "C42_CCID=cicd-tag",
			"--build-arg", "C42_SCM_HASH=" + cm.Hash,
			"--build-arg", "C42_SCM_REPO=" + prjkit.GitOrigin,
			"--build-arg", "C42_SCM_REV=v1.2.3",
			"--build-arg", "SSH_AUTH_SOCK=ssh-socket",
			"--file", "Dockerfile",
			".",
		}
		assert.Equal(t, want, have)
	})

	t.Run("full example with private repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-socket")
		rng.EnvSet(xdef.EnvCCID, "cicd-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		bld := must.Value(NewBuild(*ConfigFrom(inf, nil)))

		// --- When ---
		have := bld.Cmd()

		// --- Then ---
		want := []string{
			"build",
			"--platform", "linux/amd64",
			"--ssh", "default=ssh-socket",
			"-t", "my.nexus.dev/repo/dki-project:v1.2.3",
			"-t", "my.nexus.dev/repo/dki-project:latest",
			"--build-arg", "C42_BUILD_DATE=2000-01-02T03:04:05.6Z",
			"--build-arg", "C42_CCID=cicd-tag",
			"--build-arg", "C42_REG_HOST=my.nexus.dev",
			"--build-arg", "C42_REG_REPO=my.nexus.dev/repo",
			"--build-arg", "C42_SCM_HASH=" + cm.Hash,
			"--build-arg", "C42_SCM_REPO=" + prjkit.GitOrigin,
			"--build-arg", "C42_SCM_REV=v1.2.3",
			"--build-arg", "SSH_AUTH_SOCK=ssh-socket",
			"--file", "Dockerfile",
			".",
		}
		assert.Equal(t, want, have)
	})

	t.Run("with additional args from project config file", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.CfgAdd("KEY0", "VAL0")
		prj.CfgAdd("KEY1", "VAL1")
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		bld := must.Value(NewBuild(*ConfigFrom(inf, nil)))

		// --- When ---
		have := bld.Cmd()

		// --- Then ---
		want := []string{
			"build",
			"--platform", "linux/amd64",
			"-t", "dki-project:v1.2.3",
			"-t", "dki-project:latest",
			"--build-arg", "C42_BUILD_DATE=2000-01-02T03:04:05.6Z",
			"--build-arg", "C42_CCID=unknown",
			"--build-arg", "C42_SCM_HASH=" + cm.Hash,
			"--build-arg", "C42_SCM_REPO=" + prjkit.GitOrigin,
			"--build-arg", "C42_SCM_REV=v1.2.3",
			"--build-arg", "KEY0=VAL0",
			"--build-arg", "KEY1=VAL1",
			"--file", "Dockerfile",
			".",
		}
		assert.Equal(t, want, have)
	})

	t.Run("with target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		cfg := ConfigFrom(inf, nil)
		cfg.target = "target"
		bld := must.Value(NewBuild(*cfg))

		// --- When ---
		have := bld.Cmd()

		// --- Then ---
		want := []string{
			"build",
			"--platform", "linux/amd64",
			"-t", "dki-project-target:v1.2.3",
			"-t", "dki-project-target:latest",
			"--target", "target",
			"--build-arg", "C42_BUILD_DATE=2000-01-02T03:04:05.6Z",
			"--build-arg", "C42_CCID=unknown",
			"--build-arg", "C42_SCM_HASH=" + cm.Hash,
			"--build-arg", "C42_SCM_REPO=" + prjkit.GitOrigin,
			"--build-arg", "C42_SCM_REV=v1.2.3",
			"--file", "Dockerfile",
			".",
		}
		assert.Equal(t, want, have)
	})
}

func Test_Build_String(t *testing.T) {
	t.Run("with build kit", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-socket")
		rng.EnvSet(xdef.EnvCCID, "cicd-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		bld := must.Value(NewBuild(*ConfigFrom(inf, nil)))

		// --- When ---
		have := bld.String()

		// --- Then ---
		want := "" +
			"DOCKER_BUILDKIT=1" +
			" docker" +
			" build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-socket" +
			" -t dki-project:v1.2.3" +
			" -t dki-project:latest" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=cicd-tag" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=" + prjkit.GitOrigin +
			" --build-arg C42_SCM_REV=v1.2.3" +
			" --build-arg SSH_AUTH_SOCK=ssh-socket" +
			" --file Dockerfile ."
		assert.Equal(t, want, have)
	})

	t.Run("without build kit", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-socket")
		rng.EnvSet(xdef.EnvCCID, "cicd-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))
		bld := must.Value(NewBuild(*ConfigFrom(inf, nil)))
		bld.kit = false

		// --- When ---
		have := bld.String()

		// --- Then ---
		want := "" +
			"docker" +
			" build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-socket" +
			" -t dki-project:v1.2.3" +
			" -t dki-project:latest" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=cicd-tag" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=" + prjkit.GitOrigin +
			" --build-arg C42_SCM_REV=v1.2.3" +
			" --build-arg SSH_AUTH_SOCK=ssh-socket" +
			" --file Dockerfile ."
		assert.Equal(t, want, have)
	})
}
