// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"os"
	"testing"
	"time"

	"github.com/ctx42/dotenv/pkg/dotenv"
	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmgo"
)

func Test_NewInfo(t *testing.T) {
	t.Run("nothing set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvUnset(xdef.EnvBldDate)

		// --- When ---
		have := NewInfo(rng.EnvAll())

		// --- Then ---
		assert.Empty(t, have.Config)
		assert.Empty(t, have.Other)
		assert.Within(t, time.Now(), "1s", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
	})

	t.Run("SSH_AUTH_SOCK set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHAuthSock, "socket")
		rng.EnvUnset(xdef.EnvBldDate)

		// --- When ---
		have := NewInfo(rng.EnvAll())

		// --- Then ---
		assert.Empty(t, have.Config)
		want := map[string]string{EnvSSHAuthSock: "socket"}
		assert.Equal(t, want, have.Other)
		assert.Within(t, time.Now(), "1s", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
	})

	t.Run("C42_BLD_DATE set", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		// --- When ---
		have := NewInfo(rng.EnvAll())

		// --- Then ---
		assert.Empty(t, have.Config)
		assert.Empty(t, have.Other)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
	})

	t.Run("C42_BLD_DATE invalid", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "invalid")

		// --- When ---
		have := NewInfo(rng.EnvAll())

		// --- Then ---
		assert.Empty(t, have.Config)
		assert.Empty(t, have.Other)
		assert.Within(t, time.Now(), "1s", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
	})
}

func Test_GetInfo(t *testing.T) {
	t.Run("no go mod under ancestor module", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvUnset(EnvSSHAuthSock)

		parent := t.TempDir()
		mod := "module example.com/parent\n\ngo 1.26\n"
		oskit.Write(t, mod, parent, "go.mod")
		root := oskit.MkdirAll(t, parent, "proj")
		oskit.Write(t, "", oskit.MkdirAll(t, root, "configs"), CfgFile)

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), root)

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have.LDFlags)
	})

	t.Run("error - invalid build date", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet(xdef.EnvBldDate, "2020-01-01")

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorContain(t, "invalid C42_BLD_DATE: \"2020-01-01\"", err)
		assert.Nil(t, have)
	})

	t.Run("minimal project structure", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), have.Root)
		assert.Equal(t, gitaid.Version{}, have.Version)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=2000-01-02T03:04:05.600Z",
			xdef.EnvPrjName + "=project",
			xdef.EnvScmState + "=" + ScmNo,
		}
		assert.Equal(t, want, have.Env())
		assert.Nil(t, have.Overrides)
		assert.Fields(t, 8, Info{})
	})

	t.Run("SSH socket set from environment", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHAuthSock, "socket")
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), have.Root)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=" + have.BuildDateFmt(),
			xdef.EnvPrjName + "=project",
			xdef.EnvScmState + "=" + ScmNo,
			EnvSSHAuthSock + "=socket",
		}
		assert.Equal(t, want, have.Env())
		assert.Nil(t, have.Overrides)
		assert.Fields(t, 8, Info{})
	})

	t.Run("config file loaded", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.CfgAdd("KEY", "VAL")
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), have.Root)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
		assert.HasKeyValue(t, "KEY", "VAL", have.Config)
		assert.Len(t, 1, have.Config)
	})

	t.Run("config overridden from environment", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet("KEY_B", "env-b")
		rng.EnvSet("KEY_A", "")

		prj := gmtest.NewProject(t)
		prj.CfgAdd("KEY_A", "file-a")
		prj.CfgAdd("KEY_B", "file-b")
		prj.CfgAdd("KEY_C", "file-c")
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := map[string]string{
			"KEY_A": "",
			"KEY_B": "env-b",
			"KEY_C": "file-c",
		}
		assert.Equal(t, want, have.Config)
		assert.Equal(t, []string{"KEY_A", "KEY_B"}, have.Overrides)
	})

	t.Run("environment variable not in config ignored", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet("OTHER", "env-other")

		prj := gmtest.NewProject(t)
		prj.CfgAdd("KEY", "VAL")
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"KEY": "VAL"}, have.Config)
		assert.Nil(t, have.Overrides)
	})

	t.Run("initialized git repo", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), have.Root)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=" + have.BuildDateFmt(),
			xdef.EnvPrjName + "=project",
			xdef.EnvScmState + "=" + ScmNo,
		}
		assert.Equal(t, want, have.Env())
	})

	t.Run("added not committed files", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Exe("git", "init")
		prj.Exe("git", "add", "-A")
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), have.Root)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=" + have.BuildDateFmt(),
			xdef.EnvPrjName + "=project",
			xdef.EnvScmState + "=" + ScmNo,
		}
		assert.Equal(t, want, have.Env())
	})

	t.Run("with commits and clean working dir", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		cm := prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), have.Root)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=" + have.BuildDateFmt(),
			xdef.EnvPrjName + "=project",
			xdef.EnvScmHash + "=" + cm.Hash,
			xdef.EnvScmRev + "=v0.0.1-dev.1+g" + cm.Hash,
			xdef.EnvScmState + "=" + gitaid.StateClean,
		}
		assert.Equal(t, want, have.Env())
	})

	t.Run("with tag and clean working dir", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), have.Root)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=" + have.BuildDateFmt(),
			xdef.EnvPrjName + "=project",
			xdef.EnvScmHash + "=" + cm.Hash,
			xdef.EnvScmRev + "=v1.2.3",
			xdef.EnvScmState + "=" + gitaid.StateClean,
		}
		assert.Equal(t, want, have.Env())
	})

	t.Run("remote and clean", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		cm := prj.GitInitAddAll()
		prj.Exe("git", "remote", "add", "origin", prjkit.GitOrigin)
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prj.Root(), have.Root)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		assert.Empty(t, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=" + have.BuildDateFmt(),
			xdef.EnvPrjName + "=project",
			xdef.EnvScmHash + "=" + cm.Hash,
			xdef.EnvScmRepo + "=" + prjkit.GitOrigin,
			xdef.EnvScmRev + "=v0.0.1-dev.1+g" + cm.Hash,
			xdef.EnvScmState + "=" + gitaid.StateClean,
		}
		assert.Equal(t, want, have.Env())
	})

	t.Run("with go module", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.WithConfig()
		cm := prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		wLDFlags := "" +
			"-X 'example.com/comp/project.bldDate=2000-01-02T03:04:05.600Z'" +
			" -X 'example.com/comp/project.scmRev=" +
			"v0.0.1-dev.1+g" + cm.Hash + "'" +
			" -X 'example.com/comp/project.scmHash=" + cm.Hash + "'" +
			" -X 'example.com/comp/project.scmState=" +
			gitaid.StateClean + "'"
		assert.Equal(t, wLDFlags, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=" + have.BuildDateFmt(),
			xdef.EnvPrjName + "=project",
			xdef.EnvScmHash + "=" + cm.Hash,
			xdef.EnvScmRev + "=v0.0.1-dev.1+g" + cm.Hash,
			xdef.EnvScmState + "=" + gitaid.StateClean,
		}
		assert.Equal(t, want, have.Env())
	})

	t.Run("with go module and tags", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHAuthSock)
		rng.EnvSet(xdef.EnvBldDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Time(t, "2000-01-02T03:04:05.6Z", have.BuildDate)
		assert.False(t, have.HasDockerfile)
		wLDFlags := "" +
			"-X 'example.com/comp/project.bldDate=2000-01-02T03:04:05.600Z'" +
			" -X 'example.com/comp/project.scmRev=v1.2.3'" +
			" -X 'example.com/comp/project.scmHash=" + cm.Hash + "'" +
			" -X 'example.com/comp/project.scmState=" +
			gitaid.StateClean + "'"
		assert.Equal(t, wLDFlags, have.LDFlags)
		want := []string{
			xdef.EnvBldDate + "=" + have.BuildDateFmt(),
			xdef.EnvPrjName + "=project",
			xdef.EnvScmHash + "=" + cm.Hash,
			xdef.EnvScmRev + "=v1.2.3",
			xdef.EnvScmState + "=" + gitaid.StateClean,
		}
		assert.Equal(t, want, have.Env())
	})

	t.Run("with Dockerfile", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.WithDockerfile()
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have.HasDockerfile)
	})

	t.Run("error - empty directory", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid go mod", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFile("go.mod")
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gmgo.ErrImpPath, err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid config file", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.CreateFileWith("INVALID LINE\n", CfgPath)
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, dotenv.ErrInvLine, err)
		assert.Nil(t, have)
	})

	t.Run("error - no config file", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.Close()

		// --- When ---
		have, err := GetInfo(t.Context(), rng.EnvAll(), prj.Root())

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
		assert.ErrorIs(t, ErrNoConfig, err)
		assert.Nil(t, have)
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
		have := inf.CfgGet(xdef.EnvPrjName)

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
		hVal, hExist := inf.CfgLookup("CFG1")

		// --- Then ---
		assert.Equal(t, "CV1", hVal)
		assert.True(t, hExist)
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
		hVal, hExist := inf.CfgLookup("CFG2")

		// --- Then ---
		assert.Equal(t, "", hVal)
		assert.False(t, hExist)
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
		hVal, hExist := inf.CfgLookup(xdef.EnvPrjName)

		// --- Then ---
		assert.Equal(t, "", hVal)
		assert.False(t, hExist)
	})
}

func Test_Info_Set(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		// --- Given ---
		var inf Info

		// --- When ---
		inf.Set("FLD0", "FV0")

		// --- Then ---
		assert.Equal(t, "FV0", inf.Get("FLD0"))
	})

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
		hVal, hExist := inf.Lookup("EXTRA")

		// --- Then ---
		assert.Equal(t, "field", hVal)
		assert.True(t, hExist)
	})

	t.Run("added empty", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}
		inf.Set("EXTRA", "")

		// --- When ---
		hVal, hExist := inf.Lookup("EXTRA")

		// --- Then ---
		assert.Equal(t, "", hVal)
		assert.True(t, hExist)
	})

	t.Run("empty field", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}
		inf.Set("EXTRA", "")

		// --- When ---
		hVal, hExist := inf.Lookup(xdef.EnvPrjName)

		// --- Then ---
		assert.Equal(t, "", hVal)
		assert.False(t, hExist)
	})

	t.Run("unknown field", func(t *testing.T) {
		// --- Given ---
		inf := &Info{
			Other: make(map[string]string),
		}

		// --- When ---
		hVal, hExist := inf.Lookup("EXTRA")

		// --- Then ---
		assert.Equal(t, "", hVal)
		assert.False(t, hExist)
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
