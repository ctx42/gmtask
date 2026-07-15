// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"fmt"
	"testing"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/dkrkit"
	"github.com/ctx42/testkit/pkg/netkit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmprj"
)

func Test_NewDockerCmd(t *testing.T) {
	// --- Given ---
	ta := &Flags{}

	// --- When ---
	dc := NewDockerCmd(ta)

	// --- Then ---
	assert.Same(t, ta, dc.Flags)
}

func Test_DockerCmd_Init(t *testing.T) {
	t.Run("default target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-sock")
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		cm := prj.GitInitAddAll("v1.1.1")
		prj.GitSetRemote()
		prj.Close()

		fls := NewFlags("name")
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)

		// --- When ---
		err := dc.Init(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		assert.Equal(t, "dki-project", dc.Config.name)
		assert.Equal(t, "v1.1.1", dc.Config.tag)
		assert.True(t, dc.Config.latest)
		assert.Equal(t, "", dc.Config.target)
		assert.Equal(t, "ssh-sock", dc.Config.ssh)
		assert.Equal(t, "linux/amd64", dc.Config.platform)
		wArgs := map[string]string{
			EnvSSHSock:        "ssh-sock",
			xdef.EnvBuildDate: "2000-01-02T03:04:05.6Z",
			xdef.EnvCCID:      xdef.PhUnknown,
			xdef.EnvScmRepo:   prjkit.GitOrigin,
			xdef.EnvScmHash:   cm.Hash,
			xdef.EnvScmRev:    "v1.1.1",
		}
		assert.Equal(t, wArgs, dc.Config.args)
		assert.True(t, dc.Config.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", dc.Config.buildDate)

		custom := gomake.EnvSplit(dc.Info.Custom())
		assert.HasKeyValue(t, EnvDkrImgName, "dki-project", custom)
		assert.HasKeyValue(t, EnvDkrImgTag, "v1.1.1", custom)
		assert.HasKeyValue(t, EnvDkrImgRef, "dki-project:v1.1.1", custom)

		assert.Len(t, 1, dc.Builds)
		want := "" +
			"DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project:v1.1.1" +
			" -t dki-project:latest" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=%s" +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=v1.1.1" +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile ."
		want = fmt.Sprintf(want, cm.Hash)
		assert.Equal(t, want, dc.Builds[0].String())
	})

	t.Run("multiple targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-sock")
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.CfgBldTargets("first,second,third")
		cm := prj.GitInitAddAll("v1.1.1")
		prj.GitSetRemote()
		prj.Close()

		fls := NewFlags("name")
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)

		// --- When ---
		err := dc.Init(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		assert.Equal(t, "dki-project", dc.Config.name)
		assert.Equal(t, "v1.1.1", dc.Config.tag)
		assert.True(t, dc.Config.latest)
		assert.Equal(t, "", dc.Config.target)
		assert.Equal(t, "ssh-sock", dc.Config.ssh)
		assert.Equal(t, "linux/amd64", dc.Config.platform)
		wArgs := map[string]string{
			EnvSSHSock:        "ssh-sock",
			xdef.EnvBuildDate: "2000-01-02T03:04:05.6Z",
			xdef.EnvCCID:      xdef.PhUnknown,
			xdef.EnvBldTargets:     "first,second,third",
			xdef.EnvScmRepo:   prjkit.GitOrigin,
			xdef.EnvScmHash:   cm.Hash,
			xdef.EnvScmRev:    "v1.1.1",
		}
		assert.Equal(t, wArgs, dc.Config.args)
		assert.True(t, dc.Config.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", dc.Config.buildDate)

		custom := gomake.EnvSplit(dc.Info.Custom())
		assert.HasKeyValue(t, EnvDkrImgNameStem, "dki-project", custom)
		want := "dki-project-first,dki-project-second,dki-project-third"
		assert.HasKeyValue(t, EnvDkrImgNames, want, custom)
		assert.HasKeyValue(t, EnvDkrImgTag, "v1.1.1", custom)
		want = "" +
			"dki-project-first:v1.1.1," +
			"dki-project-second:v1.1.1," +
			"dki-project-third:v1.1.1"
		assert.HasKeyValue(t, EnvDkrImgRefs, want, custom)

		assert.Len(t, 3, dc.Builds)
		want = "" +
			"DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project-first:v1.1.1" +
			" -t dki-project-first:latest" +
			" --target first" +
			" --build-arg C42_BLD_TARGETS=first,second,third" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=v1.1.1" +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile ."
		assert.Equal(t, want, dc.Builds[0].String())

		want = "" +
			"DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project-second:v1.1.1" +
			" -t dki-project-second:latest" +
			" --target second" +
			" --build-arg C42_BLD_TARGETS=first,second,third" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=v1.1.1" +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile ."
		assert.Equal(t, want, dc.Builds[1].String())

		want = "" +
			"DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project-third:v1.1.1" +
			" -t dki-project-third:latest" +
			" --target third" +
			" --build-arg C42_BLD_TARGETS=first,second,third" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=v1.1.1" +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile ."
		assert.Equal(t, want, dc.Builds[2].String())
	})

	t.Run("pick targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-sock")
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.CfgBldTargets("first,second,third")
		cm := prj.GitInitAddAll("v1.1.1")
		prj.GitSetRemote()
		prj.Close()

		fls := NewFlags("name")
		fls.Targets = []string{"second", "third"}
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)

		// --- When ---
		err := dc.Init(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		assert.Equal(t, "dki-project", dc.Config.name)
		assert.Equal(t, "v1.1.1", dc.Config.tag)
		assert.True(t, dc.Config.latest)
		assert.Equal(t, "", dc.Config.target)
		assert.Equal(t, "ssh-sock", dc.Config.ssh)
		assert.Equal(t, "linux/amd64", dc.Config.platform)
		wArgs := map[string]string{
			EnvSSHSock:        "ssh-sock",
			xdef.EnvBuildDate: "2000-01-02T03:04:05.6Z",
			xdef.EnvCCID:      xdef.PhUnknown,
			xdef.EnvBldTargets:     "first,second,third",
			xdef.EnvScmRepo:   prjkit.GitOrigin,
			xdef.EnvScmHash:   cm.Hash,
			xdef.EnvScmRev:    "v1.1.1",
		}
		assert.Equal(t, wArgs, dc.Config.args)
		assert.True(t, dc.Config.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", dc.Config.buildDate)

		custom := gomake.EnvSplit(dc.Info.Custom())
		assert.HasKeyValue(t, EnvDkrImgNameStem, "dki-project", custom)
		want := "dki-project-second,dki-project-third"
		assert.HasKeyValue(t, EnvDkrImgNames, want, custom)
		assert.HasKeyValue(t, EnvDkrImgTag, "v1.1.1", custom)
		want = "dki-project-second:v1.1.1,dki-project-third:v1.1.1"
		assert.HasKeyValue(t, EnvDkrImgRefs, want, custom)

		assert.Len(t, 2, dc.Builds)
		want = "" +
			"DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project-second:v1.1.1" +
			" -t dki-project-second:latest" +
			" --target second" +
			" --build-arg C42_BLD_TARGETS=first,second,third" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=v1.1.1" +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile ."
		assert.Equal(t, want, dc.Builds[0].String())

		want = "" +
			"DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project-third:v1.1.1" +
			" -t dki-project-third:latest" +
			" --target third" +
			" --build-arg C42_BLD_TARGETS=first,second,third" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=v1.1.1" +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile ."
		assert.Equal(t, want, dc.Builds[1].String())
	})

	t.Run("error - not a project directory", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		fls := NewFlags("name")
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)

		// --- When ---
		err := dc.Init(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})

	t.Run("error - not a git repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.Close()

		fls := NewFlags("name")
		dc := NewDockerCmd(fls)

		// --- When ---
		err := dc.Init(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyTag, err)
	})

	t.Run("error - no Dockerfile", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		fls := NewFlags("name")
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)

		// --- When ---
		err := dc.Init(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNoDockerfile, err)
	})

	t.Run("error - pick not existing target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.CfgBldTargets("first,second,third")
		prj.GitInitAddAll("v1.1.1")
		prj.Close()

		fls := NewFlags("name")
		fls.Targets = []string{"unknown"}
		fls.ImgLatest = true

		dc := NewDockerCmd(fls)

		// --- When ---
		err := dc.Init(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNoTarget, err)
		assert.ErrorContain(t, "[unknown]", err)
	})
}

func Test_DockerCmd_Build(t *testing.T) {
	t.Run("default target no SCM", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		ref, refLatest := prj.ImgRef(), prj.ImgRefLatest()
		eoutS := tst.Stderr()

		// Test build log.
		assert.Contain(t, ref, eoutS)
		assert.Contain(t, refLatest, eoutS)
		assert.Count(t, 1, "#gomake INFO# DOCKER_BUILDKIT=1 docker build", eoutS)

		// Test image exist.
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(ref))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refLatest))

		// Run images and test output.
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(ref))
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(refLatest))
	})

	t.Run("env-labels when no SCM no cc-tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")
		rng.EnvUnset(xdef.EnvCCID)

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotEmpty(t, tst.Stderr())
		ref := prj.TgtRef("third")
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(ref))

		wEnv := map[string]string{
			xdef.EnvImgCreated: "2000-01-02T03:04:05.6Z",
			xdef.EnvImgRefName: xdef.PhUnknown,
			xdef.EnvImgSrc:     xdef.PhUnknown,
			xdef.EnvImgRev:     xdef.PhHash,
			xdef.EnvImgVer:     xdef.PhRev,
			xdef.EnvImgTitle:   "third",
		}
		assert.MapSubset(t, wEnv, dkrkit.NewT(t).Envs(ref))

		wLabel := map[string]string{
			xdef.LabImgCreated: "2000-01-02T03:04:05.6Z",
			xdef.LabImgRefName: xdef.PhUnknown,
			xdef.LabImgSrc:     xdef.PhUnknown,
			xdef.LabImgRev:     xdef.PhHash,
			xdef.LabImgVer:     xdef.PhRev,
			xdef.LabImgTitle:   "third",
		}
		assert.MapSubset(t, wLabel, dkrkit.NewT(t).Labels(ref))
	})

	t.Run("env-labels with SCM and cc-tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")
		rng.EnvSet(xdef.EnvCCID, "cc-tag")

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.GitSetRemote()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = false
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotEmpty(t, tst.Stderr())
		ref := prj.TgtRef("third")
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(ref))

		wEnv := map[string]string{
			xdef.EnvImgCreated: "2000-01-02T03:04:05.6Z",
			xdef.EnvImgRefName: "cc-tag",
			xdef.EnvImgSrc:     prjkit.GitOrigin,
			xdef.EnvImgRev:     cm.Hash,
			xdef.EnvImgVer:     "v1.2.3",
			xdef.EnvImgTitle:   "third",
		}
		assert.MapSubset(t, wEnv, dkrkit.NewT(t).Envs(ref))

		wLabel := map[string]string{
			xdef.LabImgCreated: "2000-01-02T03:04:05.6Z",
			xdef.LabImgRefName: "cc-tag",
			xdef.LabImgSrc:     prjkit.GitOrigin,
			xdef.LabImgRev:     cm.Hash,
			xdef.LabImgVer:     "v1.2.3",
			xdef.LabImgTitle:   "third",
		}
		assert.MapSubset(t, wLabel, dkrkit.NewT(t).Labels(ref))
	})

	t.Run("dry run default target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-sock")
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		cm := prj.GitInitAddAll("v1.1.1")
		prj.GitSetRemote()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgLatest = true
		fls.DryRun = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"#gomake INFO# DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project:v1.1.1" +
			" -t dki-project:latest" +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=v1.1.1" +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile .\n"
		assert.Equal(t, want, tst.Stderr())

		ref, refLatest := prj.ImgRef(), prj.ImgRefLatest()
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(ref))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refLatest))
	})

	t.Run("many targets and C42_BLD_TARGETS has all of them", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		refFirst, refFirstLatest := prj.TgtRef("first"), prj.TgtRefLatest("first")
		refSecond, refSecondLatest := prj.TgtRef("second"), prj.TgtRefLatest("second")
		refThird, refThirdLatest := prj.TgtRef("third"), prj.TgtRefLatest("third")
		eoutS := tst.Stderr()

		// Test build log.
		assert.Contain(t, refFirst, eoutS)
		assert.Contain(t, refFirstLatest, eoutS)
		assert.Contain(t, refSecond, eoutS)
		assert.Contain(t, refSecondLatest, eoutS)
		assert.Contain(t, refThird, eoutS)
		assert.Contain(t, refThirdLatest, eoutS)
		assert.Count(t, 3, "#gomake INFO# DOCKER_BUILDKIT=1 docker build", eoutS)

		// Test images exist.
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refFirst))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refFirstLatest))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refSecond))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refSecondLatest))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refThird))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refThirdLatest))

		// Run images and test output.
		assert.Equal(t, "first image", dkrkit.NewT(t).CtrRun(refFirst))
		assert.Equal(t, "first image", dkrkit.NewT(t).CtrRun(refFirstLatest))
		assert.Equal(t, "second image", dkrkit.NewT(t).CtrRun(refSecond))
		assert.Equal(t, "second image", dkrkit.NewT(t).CtrRun(refSecondLatest))
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(refThird))
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(refThirdLatest))
	})

	t.Run("many targets C42_BLD_TARGETS set for some of them", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		refFirst, refFirstLatest := prj.TgtRef("first"), prj.TgtRefLatest("first")
		refSecond, refSecondLatest := prj.TgtRef("second"), prj.TgtRefLatest("second")
		refThird, refThirdLatest := prj.TgtRef("third"), prj.TgtRefLatest("third")
		eoutS := tst.Stderr()

		// Test build log.
		assert.Contain(t, refFirst, eoutS)
		assert.Contain(t, refFirstLatest, eoutS)
		assert.Contain(t, refSecond, eoutS)
		assert.Contain(t, refSecondLatest, eoutS)
		assert.NotContain(t, refThird, eoutS)
		assert.NotContain(t, refThirdLatest, eoutS)
		assert.Count(t, 2, "#gomake INFO# DOCKER_BUILDKIT=1 docker build", eoutS)

		// Test images exist.
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refFirst))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refFirstLatest))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refSecond))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refSecondLatest))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refThird))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refThirdLatest))

		// Run images and test output.
		assert.Equal(t, "first image", dkrkit.NewT(t).CtrRun(refFirst))
		assert.Equal(t, "first image", dkrkit.NewT(t).CtrRun(refFirstLatest))
		assert.Equal(t, "second image", dkrkit.NewT(t).CtrRun(refSecond))
		assert.Equal(t, "second image", dkrkit.NewT(t).CtrRun(refSecondLatest))
	})

	t.Run("build single target out of multiple", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"first"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		refFirst, refFirstLatest := prj.TgtRef("first"), prj.TgtRefLatest("first")
		refSecond, refSecondLatest := prj.TgtRef("second"), prj.TgtRefLatest("second")
		eoutS := tst.Stderr()

		// Test build log.
		assert.Contain(t, refFirst, eoutS)
		assert.Contain(t, refFirstLatest, eoutS)
		assert.NotContain(t, refSecond, eoutS)
		assert.NotContain(t, refSecondLatest, eoutS)
		assert.Count(t, 1, "#gomake INFO# DOCKER_BUILDKIT=1 docker build", eoutS)

		// Test images exist.
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refFirst))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refFirstLatest))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refSecond))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refSecondLatest))

		// Run images and test output.
		assert.Equal(t, "first image", dkrkit.NewT(t).CtrRun(refFirst))
		assert.Equal(t, "first image", dkrkit.NewT(t).CtrRun(refFirstLatest))
	})

	t.Run("build multiple targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"first", "third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		refFirst, refFirstLatest := prj.TgtRef("first"), prj.TgtRefLatest("first")
		refSecond, refSecondLatest := prj.TgtRef("second"), prj.TgtRefLatest("second")
		refThird, refThirdLatest := prj.TgtRef("third"), prj.TgtRefLatest("third")
		eoutS := tst.Stderr()

		// Test build log.
		assert.Contain(t, refFirst, eoutS)
		assert.Contain(t, refFirstLatest, eoutS)
		assert.NotContain(t, refSecond, eoutS)
		assert.NotContain(t, refSecondLatest, eoutS)
		assert.Contain(t, refThird, eoutS)
		assert.Contain(t, refThirdLatest, eoutS)
		assert.Count(t, 2, "#gomake INFO# DOCKER_BUILDKIT=1 docker build", eoutS)

		// Test images exist.
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refFirst))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refFirstLatest))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refSecond))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refSecondLatest))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refThird))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refThirdLatest))

		// Run images and test output.
		assert.Equal(t, "first image", dkrkit.NewT(t).CtrRun(refFirst))
		assert.Equal(t, "first image", dkrkit.NewT(t).CtrRun(refFirstLatest))
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(refThird))
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(refThirdLatest))
	})

	t.Run("warn SSH_AUTH_SOCK not set", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		ref, refLatest := prj.ImgRef(), prj.ImgRefLatest()
		eoutS := tst.Stderr()

		assert.Contain(t, ref, eoutS)
		assert.Contain(t, refLatest, eoutS)
		assert.Contain(t, "#gomake WARN# SSH_AUTH_SOCK", eoutS)
		assert.Contain(t, "#gomake INFO# DOCKER_BUILDKIT=1 docker build", eoutS)
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(ref))
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(refLatest))
	})

	t.Run("error - invalid docker host", func(t *testing.T) {
		// --- Given ---
		port := must.Value(netkit.GetFreePort())
		host := fmt.Sprintf("tcp://127.0.0.1:%d", port)

		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvSet("DOCKER_HOST", host)

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Build(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "Cannot connect to the Docker daemon", err)
		ref, refLatest := prj.ImgRef(), prj.ImgRefLatest()
		eoutS := tst.Stderr()
		assert.Contain(t, "Cannot connect to the Docker daemon", eoutS)
		assert.Contain(t, host, eoutS)
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(ref))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refLatest))
	})
}

func Test_DockerCmd_Push(t *testing.T) {
	t.Run("error - no private repo in project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Push(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoPrvRepo, err)
	})

	t.Run("dry run default target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgLatest = true
		fls.DryRun = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Push(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "#gomake INFO# docker push my.nexus.dev/repo/dki-project:v1.1.1\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("dry run multi target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.CfgRegRepoDef()
		prj.CfgAdd(xdef.EnvBldTargets, "first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.DryRun = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Push(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"#gomake INFO# docker push my.nexus.dev/repo/dki-project-first:v1.1.1\n" +
			"#gomake INFO# docker push my.nexus.dev/repo/dki-project-second:v1.1.1\n" +
			"#gomake INFO# docker push my.nexus.dev/repo/dki-project-third:v1.1.1\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_DockerCmd_Run(t *testing.T) {
	t.Run("error - no targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		dc := &DockerCmd{}

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoTarget, err)
	})

	t.Run("run default target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Count(t, 1, want, tst.Stderr())
		want = "#gomake INFO# docker run --rm -v %s:/ctx42/project:ro %s"
		want = fmt.Sprintf(want, prj.Root(), prj.ImgRef())
		assert.Contain(t, want, tst.Stderr())
		assert.Equal(t, "third image\n", tst.Stdout())
	})

	t.Run("run command custom command in the container", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfileNEP()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Args = []string{"ls"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Count(t, 1, want, tst.Stderr())
		want = "#gomake INFO# docker run --rm -v %s:/ctx42/project:ro %s ls"
		want = fmt.Sprintf(want, prj.Root(), prj.ImgRef())
		assert.Contain(t, want, tst.Stderr())
		assert.Equal(t, "Dockerfile\nconfigs\n", tst.Stdout())
	})

	t.Run("error - running command in the container", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfileNEP()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Args = []string{"unknown"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		// Docker's exact wording varies by CLI version; match the
		// version-stable fragment.
		assert.ErrorContain(t, "executable file not found in $PATH", err)

		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Count(t, 1, want, tst.Stderr())
		want = "#gomake INFO# docker run --rm -v %s:/ctx42/project:ro %s unknown"
		want = fmt.Sprintf(want, prj.Root(), prj.ImgRef())
		assert.Contain(t, want, tst.Stderr())
	})

	t.Run("dry run default target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-sock")
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		cm := prj.GitInitAddAll()
		prj.GitSetRemote()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.DryRun = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"#gomake INFO# DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project:" + cm.Hash +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=" + cm.Hash +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile .\n" +
			"" +
			"#gomake INFO# docker run --rm" +
			" -v %s:/ctx42/project:ro" +
			" dki-project:" + cm.Hash +
			"\n"
		want = fmt.Sprintf(want, prj.Root())
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("pick target to run", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"second"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Count(t, 1, want, tst.Stderr())
		want = "#gomake INFO# docker run --rm -v %s:/ctx42/project:ro %s"
		want = fmt.Sprintf(want, prj.Root(), prj.TgtRef("second"))
		assert.Contain(t, want, tst.Stderr())
		assert.Equal(t, "second image\n", tst.Stdout())
	})

	t.Run("error - can run only one target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"second", "third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrMultiTarget, err)
	})

	t.Run("builds target only once", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"second"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// Build once.
		assert.NoError(t, dc.Build(ctx, rng))
		tst.ResetStdout()
		tst.ResetStderr()

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Count(t, 0, want, tst.Stderr())
		want = "#gomake INFO# docker run --rm -v %s:/ctx42/project:ro %s"
		want = fmt.Sprintf(want, prj.Root(), prj.TgtRef("second"))
		assert.Contain(t, want, tst.Stderr())
		assert.Equal(t, "second image\n", tst.Stdout())
	})

	t.Run("builds target only once unless rebuild flag used", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"second"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		fls.Rebuild = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// Build once.
		assert.NoError(t, dc.Build(ctx, rng))
		tst.ResetStdout()
		tst.ResetStderr()

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Count(t, 1, want, tst.Stderr())
		want = "#gomake INFO# docker run --rm -v %s:/ctx42/project:ro %s"
		want = fmt.Sprintf(want, prj.Root(), prj.TgtRef("second"))
		assert.Contain(t, want, tst.Stderr())
		assert.Equal(t, "second image\n", tst.Stdout())
	})

	t.Run("error - must pick target when multiple defined", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrReqTarget, err)
	})

	t.Run("error - cannot pick more than one target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"second", "third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrMultiTarget, err)
	})

	t.Run("error - unknown target name", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("unknown")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"unknown"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Run(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Contain(t, want, tst.Stderr())
		want = "ERROR: failed to build: failed to solve: target stage"
		assert.Contain(t, want, tst.Stderr())
	})
}

func Test_DockerCmd_Sh(t *testing.T) {
	t.Run("error - no targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		dc := &DockerCmd{}

		// --- When ---
		err := dc.Sh(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoTarget, err)
	})

	t.Run("error - must pick target when multiple defined", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Sh(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrReqTarget, err)
	})

	t.Run("dry run default target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-sock")
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		cm := prj.GitInitAddAll()
		prj.GitSetRemote()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.DryRun = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Sh(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"#gomake INFO# DOCKER_BUILDKIT=1 docker build" +
			" --platform linux/amd64" +
			" --ssh default=ssh-sock" +
			" -t dki-project:" + cm.Hash +
			" --build-arg C42_BUILD_DATE=2000-01-02T03:04:05.6Z" +
			" --build-arg C42_CCID=unknown" +
			" --build-arg C42_SCM_HASH=" + cm.Hash +
			" --build-arg C42_SCM_REPO=git@example.com:comp/project.git" +
			" --build-arg C42_SCM_REV=" + cm.Hash +
			" --build-arg SSH_AUTH_SOCK=ssh-sock" +
			" --file Dockerfile .\n" +
			"" +
			"#gomake INFO# docker run --rm -it" +
			" -v %s:/ctx42/project:ro" +
			" -v ssh-sock:/ssh-sock" +
			" -e SSH_AUTH_SOCK=/ssh-sock" +
			" dki-project:%s /bin/sh --login\n"
		want = fmt.Sprintf(want, prj.Root(), cm.Hash)
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - unknown target name", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("unknown")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"unknown"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Sh(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Contain(t, want, tst.Stderr())
		want = "ERROR: failed to build: failed to solve: target stage"
		assert.Contain(t, want, tst.Stderr())
	})

	t.Run("error - cannot pick more than one target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"second", "third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.Sh(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrMultiTarget, err)
	})
}

func Test_DockerCmd_Reference(t *testing.T) {
	t.Run("no need to pick target when one defined", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		imgName := prj.ImgName()
		imgTag := prj.ImgTag()

		fls := NewFlags("name")
		fls.ImgName = imgName
		fls.ImgTag = imgTag
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// Build once.
		assert.NoError(t, dc.Build(ctx, rng))
		tst.ResetStdout()
		tst.ResetStderr()

		// --- When ---
		have, err := dc.Reference()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, imgName+"-first:"+imgTag, have)
	})

	t.Run("pick target when multiple defined", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		imgName := prj.ImgName()
		imgTag := prj.ImgTag()

		fls := NewFlags("name")
		fls.Targets = []string{"second"}
		fls.ImgName = imgName
		fls.ImgTag = imgTag
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// Build once.
		assert.NoError(t, dc.Build(ctx, rng))
		tst.ResetStdout()
		tst.ResetStderr()

		// --- When ---
		have, err := dc.Reference()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, imgName+"-second:"+imgTag, have)
	})

	t.Run("error - must pick target when multiple defined", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		imgName := prj.ImgName()
		imgTag := prj.ImgTag()

		fls := NewFlags("name")
		fls.ImgName = imgName
		fls.ImgTag = imgTag
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// Build once.
		assert.NoError(t, dc.Build(ctx, rng))
		tst.ResetStdout()
		tst.ResetStderr()

		// --- When ---
		have, err := dc.Reference()

		// --- Then ---
		assert.ErrorIs(t, ErrReqTarget, err)
		assert.Empty(t, have)
	})

	t.Run("error - cannot pick multiple targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		imgName := prj.ImgName()
		imgTag := prj.ImgTag()

		fls := NewFlags("name")
		fls.Targets = []string{"first", "second", "third"}
		fls.ImgName = imgName
		fls.ImgTag = imgTag
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// Build once.
		assert.NoError(t, dc.Build(ctx, rng))
		tst.ResetStdout()
		tst.ResetStderr()

		// --- When ---
		have, err := dc.Reference()

		// --- Then ---
		assert.ErrorIs(t, ErrMultiTarget, err)
		assert.Empty(t, have)
	})
}

func Test_DockerCmd_build(t *testing.T) {
	t.Run("build once", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		assert.NoError(t, dc.build(ctx, rng, prj.TgtRef("third"), false))
		assert.NoError(t, dc.build(ctx, rng, prj.TgtRef("third"), false))

		// --- Then ---
		ref, refLatest := prj.TgtRef("third"), prj.TgtRefLatest("third")
		eoutS := tst.Stderr()

		// Test build log.
		assert.Count(t, 1, "#gomake INFO# DOCKER_BUILDKIT=1 docker build", eoutS)

		// Test image exist.
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(ref))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refLatest))

		// Run images and test output.
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(ref))
	})

	t.Run("build once unless rebuild true", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		dc := NewDockerCmd(fls)
		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		assert.NoError(t, dc.build(ctx, rng, prj.TgtRef("third"), false))
		assert.NoError(t, dc.build(ctx, rng, prj.TgtRef("third"), true))

		// --- Then ---
		ref, refLatest := prj.TgtRef("third"), prj.TgtRefLatest("third")
		eoutS := tst.Stderr()

		// Test build log.
		assert.Count(t, 2, "#gomake INFO# DOCKER_BUILDKIT=1 docker build", eoutS)

		// Test image exist.
		assert.NotNil(t, dkrkit.NewT(t).ImgLs().FindByRef(ref))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refLatest))

		// Run images and test output.
		assert.Equal(t, "third image", dkrkit.NewT(t).CtrRun(ref))
	})

	t.Run("error - invalid docker host", func(t *testing.T) {
		// --- Given ---
		port := must.Value(netkit.GetFreePort())
		host := fmt.Sprintf("tcp://127.0.0.1:%d", port)

		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet("DOCKER_HOST", host)

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		fls := NewFlags("name")
		fls.Targets = []string{"third"}
		fls.ImgName = prj.ImgName()
		fls.ImgTag = prj.ImgTag()
		fls.ImgLatest = true
		dc := NewDockerCmd(fls)

		assert.NoError(t, dc.Init(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		err := dc.build(ctx, rng, prj.TgtRef("third"), false)

		// --- Then ---
		assert.ErrorContain(t, "Cannot connect to the Docker daemon", err)
		ref, refLatest := prj.ImgRef(), prj.ImgRefLatest()
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(ref))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refLatest))
	})
}
