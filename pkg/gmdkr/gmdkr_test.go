// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/dkrkit"
	"github.com/ctx42/testkit/pkg/exekit"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmprj"
)

func Test_Docker_Login(t *testing.T) {
	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Docker{}.Login(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "" +
			"Usage of :docker:login:\n" +
			"  -h, --help    show help\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Docker{}.Login(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})

	t.Run("error - project without private repo in config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Docker{}.Login(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "docker private repo not configured", err)
		assert.ErrorContain(t, gmprj.CfgPath, err)
	})

	t.Run("error - no interactive login possible", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Docker{}.Login(ctx, rng)

		// --- Then ---
		assert.Equal(t, "login to my.nexus.dev/repo\n", tst.Stdout())
		// Docker's exact wording/case varies by CLI version; match the
		// version-stable fragment.
		assert.ErrorContain(t, "interactive login", err)
		assert.Contain(t, "interactive login", tst.Stderr())
	})
}

func Test_Image_Build(t *testing.T) {
	t.Run("build", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(
			"--name", prj.ImgName(),
			"--tag", prj.ImgTag(),
			"-l",
		)

		// --- When ---
		err := Image{}.Build(ctx, rng)

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

	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Image{}.Build(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "" +
			"Usage of :docker:image:build:\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n" +
			"  -l, --latest     add latest image tag\n" +
			"  -n, --name       docker image name\n" +
			"  -r, --rebuild    rebuild image\n" +
			"  -t, --tag        docker image tag\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		// --- When ---
		err := Image{}.Build(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :docker:image:build:\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n" +
			"  -l, --latest     add latest image tag\n" +
			"  -n, --name       docker image name\n" +
			"  -r, --rebuild    rebuild image\n" +
			"  -t, --tag        docker image tag\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Build(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})
}

func Test_Image_Push(t *testing.T) {
	t.Run("push", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(
			"--name", prj.ImgName(),
			"--tag", prj.ImgTag(),
			"--dry-run",
		)

		// --- When ---
		err := Image{}.Push(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		ref, refLatest := prj.ImgRef(), prj.ImgRefLatest()
		eoutS := tst.Stderr()

		// Test build log.
		assert.Contain(t, ref, eoutS)
		assert.NotContain(t, refLatest, eoutS)
		assert.Count(t, 1, "#gomake INFO# docker push", eoutS)

		// Test image exist.
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(ref))
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByRef(refLatest))
	})

	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("-h")

		// --- When ---
		err := Image{}.Push(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "" +
			"Usage of :docker:image:push:\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n" +
			"  -n, --name       docker image name\n" +
			"  -t, --tag        docker image tag\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		// --- When ---
		err := Image{}.Push(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :docker:image:push:\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n" +
			"  -n, --name       docker image name\n" +
			"  -t, --tag        docker image tag\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(
			"--name", prj.ImgName(),
			"--tag", prj.ImgTag(),
		)

		// --- When ---
		err := Image{}.Push(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})
}

func Test_Image_Run(t *testing.T) {
	t.Run("run", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout().WetStderr()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(
			"--name", prj.ImgName(),
			"--tag", prj.ImgTag(),
		)

		// --- When ---
		err := Image{}.Run(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		want := "#gomake INFO# DOCKER_BUILDKIT=1 docker build"
		assert.Count(t, 1, want, tst.Stderr())
		want = "#gomake INFO# docker run --rm -v %s:/ctx42/project:ro %s"
		want = fmt.Sprintf(want, prj.Root(), prj.ImgRef())
		assert.Contain(t, want, tst.Stderr())
		assert.Equal(t, "third image\n", tst.Stdout())
	})

	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		// --- When ---
		err := Image{}.Run(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)

		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :docker:image:run:\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n" +
			"  -n, --name       docker image name\n" +
			"  -r, --rebuild    rebuild image\n" +
			"  -t, --tag        docker image tag\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(
			"--name", prj.ImgName(),
			"--tag", prj.ImgTag(),
		)

		// --- When ---
		err := Image{}.Run(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Image{}.Run(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :docker:image:run:\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n" +
			"  -n, --name       docker image name\n" +
			"  -r, --rebuild    rebuild image\n" +
			"  -t, --tag        docker image tag\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Image_RunProj(t *testing.T) {
	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		// --- When ---
		err := Image{}.RunProj(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)

		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :docker:image:run-proj:\n" +
			"  -c, --cmd        command to run inside the container\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.RunProj(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Image{}.RunProj(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :docker:image:run-proj:\n" +
			"  -c, --cmd        command to run inside the container\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Image_Sh(t *testing.T) {
	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		// --- When ---
		err := Image{}.Sh(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)

		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :docker:image:sh:\n" +
			"  -c, --cmd        command to run inside the container\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n" +
			"  -n, --name       docker image name\n" +
			"  -t, --tag        docker image tag\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(
			"--name", prj.ImgName(),
			"--tag", prj.ImgTag(),
		)

		// --- When ---
		err := Image{}.Sh(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Image{}.Sh(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :docker:image:sh:\n" +
			"  -c, --cmd        command to run inside the container\n" +
			"  -d, --dry-run    dry run\n" +
			"  -h, --help       show help\n" +
			"  -n, --name       docker image name\n" +
			"  -t, --tag        docker image tag\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Image_Reference(t *testing.T) {
	t.Run("with private repo and git tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.GitInitAddAll("v0.1.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "my.nexus.dev/repo/dki-project:v0.1.0", tst.Stdout())
	})

	t.Run("without private repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.GitInitAddAll("v0.1.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "dki-project:v0.1.0", tst.Stdout())
	})

	t.Run("with multiple targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrReqTarget, err)
	})

	t.Run("with private repo and multiple targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrReqTarget, err)
	})

	t.Run("multiple targets one picked via argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second")
		prj.WithDockerfile()
		prj.GitInitAddAll("v0.1.0")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--targets", "first")

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "dki-project-first:v0.1.0", tst.Stdout())
	})

	t.Run("error - not existing target picked", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--targets", "unknown")

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoTarget, err)
	})

	t.Run("error - multiple targets picked", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CfgBldTargets("first,second")
		prj.WithDockerfile()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--targets", "first,second")

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, ErrMultiTarget, err)
	})

	t.Run("without git tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithDockerfile()
		prj.WithConfig()
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "dki-project:"+prj.GitHash(), tst.Stdout())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})

	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)

		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :docker:image:reference:\n" +
			"  -h, --help       show help\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Image{}.Reference(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :docker:image:reference:\n" +
			"  -h, --help       show help\n" +
			"  -T, --targets    comma separated docker targets\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Image_Env(t *testing.T) {
	t.Run("single target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		envMap := gomake.EnvSplit(strings.Split(tst.Stdout(), "\n"))
		assert.HasKeyValue(t, EnvDkrImgName, "dki-project", envMap)
		assert.HasKeyValue(t, EnvDkrImgTag, "v1.1.1", envMap)
		assert.HasKeyValue(t, EnvDkrImgRef, "dki-project:v1.1.1", envMap)

		assert.HasNoKey(t, EnvDkrImgNameStem, envMap)
		assert.HasNoKey(t, EnvDkrImgNames, envMap)
		assert.HasNoKey(t, EnvDkrImgRefs, envMap)
	})

	t.Run("multiple targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		envMap := gomake.EnvSplit(strings.Split(tst.Stdout(), "\n"))
		assert.HasKeyValue(
			t,
			EnvDkrImgNameStem,
			"my.nexus.dev/repo/dki-project",
			envMap,
		)
		assert.HasKeyValue(t, EnvDkrImgTag, "v1.1.1", envMap)
		wantNames := "" +
			"my.nexus.dev/repo/dki-project-first," +
			"my.nexus.dev/repo/dki-project-second," +
			"my.nexus.dev/repo/dki-project-third"
		assert.HasKeyValue(t, EnvDkrImgNames, wantNames, envMap)
		wantRefs := "" +
			"my.nexus.dev/repo/dki-project-first:v1.1.1," +
			"my.nexus.dev/repo/dki-project-second:v1.1.1," +
			"my.nexus.dev/repo/dki-project-third:v1.1.1"
		assert.HasKeyValue(t, EnvDkrImgRefs, wantRefs, envMap)
	})

	t.Run("multiple targets with export", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--export")

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		lines := strings.Split(strings.TrimSpace(tst.Stdout()), "\n")
		for _, line := range lines {
			assert.True(t, strings.HasPrefix(line, "export "))
		}
	})

	t.Run("print single value", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(EnvDkrImgRefs)

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		wantRefs := "" +
			"my.nexus.dev/repo/dki-project-first:v1.1.1," +
			"my.nexus.dev/repo/dki-project-second:v1.1.1," +
			"my.nexus.dev/repo/dki-project-third:v1.1.1"
		assert.Equal(t, wantRefs, tst.Stdout())
	})

	t.Run("single positional value after export flag", func(t *testing.T) {
		// A flag preceding a positional argument must not be counted as a
		// positional argument (regression: it wrongly tripped ErrTooManyArgs).

		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.WithDockerfile()
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second,third")
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("--export", "unknown")

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tst.Stdout())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})

	t.Run("print unknown environment variable", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.WithDockerfile()
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second,third")
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("unknown")

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tst.Stdout())
	})

	t.Run("error - too many args", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring(EnvDkrImgRefs, EnvDkrImgTag)

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrTooManyArgs, err)
	})

	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :docker:image:env:\n" +
			"  -e, --export    export variables\n" +
			"  -h, --help      show help\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Image{}.Env(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "Usage of :docker:image:env:\n" +
			"  -e, --export    export variables\n" +
			"  -h, --help      show help\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Image_Info(t *testing.T) {
	t.Run("single target", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		envMap := infoToEnv(t, tst.Stdout())
		assert.HasKeyValue(t, EnvDkrImgName, "dki-project", envMap)
		assert.HasKeyValue(t, EnvDkrImgTag, "v1.1.1", envMap)
		assert.HasKeyValue(t, EnvDkrImgRef, "dki-project:v1.1.1", envMap)

		assert.HasNoKey(t, EnvDkrImgNameStem, envMap)
		assert.HasNoKey(t, EnvDkrImgNames, envMap)
		assert.HasNoKey(t, EnvDkrImgRefs, envMap)
	})

	t.Run("multiple targets", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		envMap := infoToEnv(t, tst.Stdout())
		wantStem := "my.nexus.dev/repo/dki-project"
		assert.HasKeyValue(t, EnvDkrImgNameStem, wantStem, envMap)
		assert.HasKeyValue(t, EnvDkrImgTag, "v1.1.1", envMap)
		wantNames := "" +
			"my.nexus.dev/repo/dki-project-first," +
			"my.nexus.dev/repo/dki-project-second," +
			"my.nexus.dev/repo/dki-project-third"
		assert.HasKeyValue(t, EnvDkrImgNames, wantNames, envMap)
		wantRefs := "" +
			"my.nexus.dev/repo/dki-project-first:v1.1.1," +
			"my.nexus.dev/repo/dki-project-second:v1.1.1," +
			"my.nexus.dev/repo/dki-project-third:v1.1.1"
		assert.HasKeyValue(t, EnvDkrImgRefs, wantRefs, envMap)
	})

	t.Run("print single value", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second,third")
		prj.WithDockerfile()
		prj.GitInitAddAll("v1.1.1")
		prj.Close()
		prj.Chdir()

		rng := tst.Ring(EnvDkrImgRefs)

		// --- When ---
		err := Image{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		wantRefs := "" +
			"my.nexus.dev/repo/dki-project-first:v1.1.1," +
			"my.nexus.dev/repo/dki-project-second:v1.1.1," +
			"my.nexus.dev/repo/dki-project-third:v1.1.1"
		assert.Equal(t, wantRefs, tst.Stdout())
	})

	t.Run("error - no project config", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Info(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrNoConfig, err)
	})

	t.Run("print unknown environment variable", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.WithDockerfile()
		prj.CfgRegRepoDef()
		prj.CfgBldTargets("first,second,third")
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()

		rng := tst.Ring("unknown")

		// --- When ---
		err := Image{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", tst.Stdout())
	})

	t.Run("error - too many args", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring(EnvDkrImgRefs, EnvDkrImgTag)

		// --- When ---
		err := Image{}.Info(ctx, rng)

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrTooManyArgs, err)
	})

	t.Run("error - unknown argument", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--unknown")

		// --- When ---
		err := Image{}.Info(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		want := "" +
			"flag provided but not defined: -unknown\n" +
			"Usage of :docker:image:info:\n" +
			"  -h, --help    show help\n\n" +
			"EXAMPLES:\n" +
			"\t:docker:image:info\n" +
			"\t:docker:image:info ENV_VAR_NAME\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		err := Image{}.Info(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "Usage of :docker:image:info:\n" +
			"  -h, --help    show help\n\n" +
			"EXAMPLES:\n" +
			"\t:docker:image:info\n" +
			"\t:docker:image:info ENV_VAR_NAME\n"
		assert.Equal(t, want, tst.Stderr())
	})
}

func Test_Image_Clean(t *testing.T) {
	// NOTE: Clean prunes ALL dangling docker images on the host, not only the
	// one this test creates. Run only in a disposable docker environment.
	t.Run("removes dangling images and keeps fresh test images", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		dt := dkrkit.NewT(t)

		// The legacy builder is required: re-tagging then leaves the prior
		// build as a classic dangling <none> image, which BuildKit does not do.
		env := append(os.Environ(), "DOCKER_BUILDKIT=0")
		exe := exekit.New(
			t,
			exekit.WithWd("testdata"),
			exekit.WithTimeout(90*time.Second),
			exekit.WithEnv(env),
		)

		// Build the same ctx42-tst-img reference twice with distinct labels so
		// each build is a distinct image; the second build takes the tag and
		// orphans the first into a dangling image.
		ref := dkrkit.RandRef()
		exe.Exe("docker", "build", "-t", ref, "--label=ctx42-tst-v1", ".")
		orphan := dt.ImgLs().FindByRef(ref)
		assert.NotNil(t, orphan)
		orphanID := orphan.ID

		exe.Exe("docker", "build", "-t", ref, "--label=ctx42-tst-v2", ".")
		t.Cleanup(func() { dt.ImgRm(ref, dkrkit.WithImgRmIgnoreErrors()) })

		danglers := dt.ImgLs(dkrkit.WithImgLsFilter("dangling=true"))
		assert.NotNil(t, danglers.FindByID(orphanID))

		rng := tst.Ring()

		// --- When ---
		err := Image{}.Clean(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		danglers = dt.ImgLs(dkrkit.WithImgLsFilter("dangling=true"))
		assert.Nil(t, danglers.FindByID(orphanID))

		// The freshly tagged image is younger than an hour, so Clean keeps it.
		assert.NotNil(t, dt.ImgLs().FindByRef(ref))
	})
}
