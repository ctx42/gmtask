package gmprj

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ctx42/dotenv/pkg/dotenv"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmgo"
)

func Test_NewInfo(t *testing.T) {
	t.Run("SSH_AUTH_SOCK and BUILD_TIMESTAMP not set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvBuildDate)

		// --- When ---
		inf := NewInfo(rng.EnvAll())

		// --- Then ---
		assert.Empty(t, inf.Config)
		assert.Empty(t, inf.Other)
		assert.Within(t, time.Now(), "1s", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
	})

	t.Run("SSH_AUTH_SOCK set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHAuthSock, "socket")
		rng.EnvUnset(xdef.EnvBuildDate)

		// --- When ---
		inf := NewInfo(rng.EnvAll())

		// --- Then ---
		assert.Empty(t, inf.Config)
		want := map[string]string{EnvSSHAuthSock: "socket"}
		assert.Equal(t, want, inf.Other)
		assert.Within(t, time.Now(), "1s", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
	})

	t.Run("BUILD_TIMESTAMP set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		inf := NewInfo(rng.EnvAll())

		// --- Then ---
		assert.Empty(t, inf.Config)
		assert.Empty(t, inf.Other)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
	})

	t.Run("BUILD_TIMESTAMP set to invalid value", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "invalid")

		// --- When ---
		inf := NewInfo(rng.EnvAll())

		// --- Then ---
		assert.Empty(t, inf.Config)
		assert.Empty(t, inf.Other)
		assert.Within(t, time.Now(), "1s", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
	})
}

func Test_GetInfo(t *testing.T) {
	t.Run("minimal project structure", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z"),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmState, ScmNo),
		}
		assert.Equal(t, want, inf.Env())
		assert.Fields(t, 5, Info{})
	})

	t.Run("build date set from environment", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmState, ScmNo),
		}
		assert.Equal(t, want, inf.Env())
		assert.Fields(t, 5, Info{})
	})

	t.Run("SSH socket set from environment", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHAuthSock, "socket")
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmState, ScmNo),
			ev(EnvSSHAuthSock, "socket"),
		}
		assert.Equal(t, want, inf.Env())
		assert.Fields(t, 5, Info{})
	})

	t.Run("config file loaded", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.CfgAdd("KEY", "VAL")
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		assert.HasKeyValue(t, "KEY", "VAL", inf.Config)
		assert.Len(t, 1, inf.Config)
	})

	t.Run("CI/CD build tag set", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvCCID, "cicd-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, "cicd-tag"),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmState, ScmNo),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("Jenkins build tag set", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvCCID, "jenkins-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, "jenkins-tag"),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmState, ScmNo),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("initialized git repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmState, ScmNo),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("added not committed files", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Exe("git", "init")
		prj.Exe("git", "add", "-A")
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmState, ScmNo),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("with commits and clean working dir", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		cm := prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmHash, cm.Hash),
			ev(xdef.EnvScmRev, cm.Hash),
			ev(xdef.EnvScmState, ScmClean),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("with tag and clean working dir", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmHash, cm.Hash),
			ev(xdef.EnvScmRev, "v1.2.3"),
			ev(xdef.EnvScmState, ScmClean),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("remote repo and clean working directory", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		cm := prj.GitInitAddAll()
		prj.Exe("git", "remote", "add", "origin", prjkit.GitOrigin)
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		assert.Empty(t, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmHash, cm.Hash),
			ev(xdef.EnvScmRepo, prjkit.GitOrigin),
			ev(xdef.EnvScmRev, cm.Hash),
			ev(xdef.EnvScmState, ScmClean),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("with go module", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.WithConfig()
		cm := prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		wantLDFlags := "" +
			"-X 'example.com/comp/project.buildDate=2000-01-02T03:04:05.6Z'" +
			" -X 'example.com/comp/project.scmRev=" + cm.Hash + "'" +
			" -X 'example.com/comp/project.scmHash=" + cm.Hash + "'" +
			" -X 'example.com/comp/project.scmState=" + ScmClean + "'" +
			" -X 'example.com/comp/project.ccid=" + xdef.PhUnknown + "'"
		assert.Equal(t, wantLDFlags, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, xdef.PhUnknown),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjGoImpSpec, prjkit.GoModName),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmHash, cm.Hash),
			ev(xdef.EnvScmRev, cm.Hash),
			ev(xdef.EnvScmState, ScmClean),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("with go module and tags", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvCCID, "jenkins-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		assert.Time(t, "2000-01-02T03:04:05.6Z", inf.BuildDate)
		assert.False(t, inf.HasDockerfile)
		wantLDFlags := "" +
			"-X 'example.com/comp/project.buildDate=2000-01-02T03:04:05.6Z'" +
			" -X 'example.com/comp/project.scmRev=v1.2.3'" +
			" -X 'example.com/comp/project.scmHash=" + cm.Hash + "'" +
			" -X 'example.com/comp/project.scmState=" + ScmClean + "'" +
			" -X 'example.com/comp/project.ccid=jenkins-tag'"
		assert.Equal(t, wantLDFlags, inf.LDFlags)
		want := []string{
			ev(xdef.EnvBuildDate, inf.BuildDateFmt()),
			ev(xdef.EnvCCID, "jenkins-tag"),
			ev(xdef.EnvProjDistDir, prj.Path("dist")),
			ev(xdef.EnvProjGoImpSpec, prjkit.GoModName),
			ev(xdef.EnvProjName, "project"),
			ev(xdef.EnvProjRootDir, prj.Root()),
			ev(xdef.EnvScmHash, cm.Hash),
			ev(xdef.EnvScmRev, "v1.2.3"),
			ev(xdef.EnvScmState, ScmClean),
		}
		assert.Equal(t, want, inf.Env())
	})

	t.Run("with Dockerfile", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, inf.HasDockerfile)
	})

	t.Run("empty directory", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
		assert.Nil(t, inf)
	})

	t.Run("invalid go.mod", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFile("go.mod")
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gmgo.ErrImpPath, err)
		assert.Nil(t, inf)
	})

	t.Run("invalid config file", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.CreateFileWith("INVALID LINE\n", CfgPath)
		prj.GoModInit()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, dotenv.ErrInvLine, err)
		assert.Nil(t, inf)
	})

	t.Run("without config file", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()

		// --- When ---
		inf, err := GetInfo(ctx, rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
		assert.ErrorIs(t, ErrNoConfig, err)
		assert.Nil(t, inf)
	})
}

func Test_Info_BuildDateFmt(t *testing.T) {
	// --- Given ---
	tim := time.Date(2000, 1, 2, 3, 4, 5, 123_000_000, time.UTC)
	inf := &Info{BuildDate: tim}

	// --- When ---
	have := inf.BuildDateFmt()

	// --- Then ---
	assert.Equal(t, "2000-01-02T03:04:05.123Z", have)
}

func Test_Info_CfgGet(t *testing.T) {
	t.Run("get existing", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Config: map[string]string{
				"CFG0": "CV0",
				"CFG1": "CV1",
			},
		}

		// --- When ---
		have := inf.CfgGet("CFG1")

		// --- Then ---
		assert.Equal(t, "CV1", have)
	})

	t.Run("get not existing", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Config: map[string]string{
				"CFG0": "CV0",
				"CFG1": "CV1",
			},
		}

		// --- When ---
		have := inf.CfgGet("CFG2")

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("get non config", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Config: map[string]string{
				"CFG0": "CV0",
				"CFG1": "CV1",
			},
		}

		// --- When ---
		have := inf.CfgGet(xdef.EnvProjName)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_Info_CfgLookup(t *testing.T) {
	t.Run("get existing", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Config: map[string]string{
				"CFG0": "CV0",
				"CFG1": "CV1",
			},
		}

		// --- When ---
		haveVal, haveExist := inf.CfgLookup("CFG1")

		// --- Then ---
		assert.Equal(t, "CV1", haveVal)
		assert.True(t, haveExist)
	})

	t.Run("get not existing", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Config: map[string]string{
				"CFG0": "CV0",
				"CFG1": "CV1",
			},
		}

		// --- When ---
		haveVal, haveExist := inf.CfgLookup("CFG2")

		// --- Then ---
		assert.Equal(t, "", haveVal)
		assert.False(t, haveExist)
	})

	t.Run("get non config", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Config: map[string]string{
				"CFG0": "CV0",
				"CFG1": "CV1",
			},
		}

		// --- When ---
		haveVal, haveExist := inf.CfgLookup(xdef.EnvProjName)

		// --- Then ---
		assert.Equal(t, "", haveVal)
		assert.False(t, haveExist)
	})
}

func Test_Info_Set_Get(t *testing.T) {
	t.Run("get set values", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}

		// --- When ---
		inf.Set("FLD0", "FV0")
		inf.Set("FLD1", "FV1")

		// --- Then ---
		assert.Equal(t, "FV0", inf.Get("FLD0"))
		assert.Equal(t, "FV1", inf.Get("FLD1"))
	})

	t.Run("set overwrites", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}

		// --- When ---
		inf.Set("FLD0", "FV0")
		inf.Set("FLD1", "FV1")
		inf.Set("FLD0", "FVX")

		// --- Then ---
		assert.Equal(t, "FVX", inf.Get("FLD0"))
		assert.Equal(t, "FV1", inf.Get("FLD1"))
	})

	t.Run("get not existing", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}
		inf.Set("FLD0", "FV0")

		// --- When ---
		have := inf.Get("not_existing")

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_Info_Lookup(t *testing.T) {
	t.Run("added field", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}
		inf.Set("EXTRA", "field")

		// --- When ---
		haveVal, haveExist := inf.Lookup("EXTRA")

		// --- Then ---
		assert.Equal(t, "field", haveVal)
		assert.True(t, haveExist)
	})

	t.Run("added empty", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}
		inf.Set("EXTRA", "")

		// --- When ---
		haveVal, haveExist := inf.Lookup("EXTRA")

		// --- Then ---
		assert.Equal(t, "", haveVal)
		assert.True(t, haveExist)
	})

	t.Run("empty field", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}
		inf.Set("EXTRA", "")

		// --- When ---
		haveVal, haveExist := inf.Lookup(xdef.EnvProjName)

		// --- Then ---
		assert.Equal(t, "", haveVal)
		assert.False(t, haveExist)
	})

	t.Run("unknown field", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}

		// --- Then ---
		haveVal, haveExist := inf.Lookup("EXTRA")

		// --- Then ---
		assert.Equal(t, "", haveVal)
		assert.False(t, haveExist)
	})
}

func Test_Info_Custom(t *testing.T) {
	t.Run("no values", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}

		// --- When ---
		have := inf.Custom()

		// --- Then ---
		assert.Equal(t, []string{}, have)
	})

	t.Run("with values", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}
		inf.Set("FLD0", "FV0")
		inf.Set("FLD1", "FV1")

		// --- When ---
		have := inf.Custom()

		// --- Then ---
		assert.Equal(t, []string{"FLD0=FV0", "FLD1=FV1"}, have)
	})
}

func Test_Info_HasDockerfile(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.WithDockerfile()
		prj.Close()

		inf := must.Value(GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- Then ---
		assert.True(t, inf.HasDockerfile)
	})

	t.Run("false", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.Close()

		inf := must.Value(GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- Then ---
		assert.False(t, inf.HasDockerfile)
	})
}

func Test_Info_Env(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Config: map[string]string{
				"A": "VAL",
				"C": "VAL",
			},
			Other: map[string]string{
				"B": "VAL",
				"D": "VAL",
			},
		}

		// --- When ---
		have := inf.Env()

		// --- Then ---
		want := []string{
			"A=VAL",
			"B=VAL",
			"C=VAL",
			"D=VAL",
		}
		assert.Equal(t, want, have)
	})
}

func Test_Info_String(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Config: map[string]string{
				"A": "VAL",
				"C": "VAL",
			},
			Other: map[string]string{
				"B": "VAL",
				"D": "VAL",
			},
		}

		// --- When ---
		have := inf.String()

		// --- Then ---
		want := "" +
			"A    VAL\n" +
			"B    VAL\n" +
			"C    VAL\n" +
			"D    VAL\n"
		assert.Equal(t, want, have)
	})
}
