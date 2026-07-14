// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/dkrkit"
	"github.com/ctx42/testkit/pkg/netkit"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmprj"
)

func Test_ImgName_tabular(t *testing.T) {
	tt := []struct {
		testN string

		projectName string
		want        string
	}{
		{"1", "", ""},
		{"2", "acme-proj", "dki-acme-proj"},
		{"3", "dki-acme-proj", "dki-acme-proj"},
		{"4", "acme-dki-proj", "acme-dki-proj"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := ImgName(tc.projectName)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_splitTargets_tabular(t *testing.T) {
	tt := []struct {
		testN string

		targets string
		want    []string
	}{
		{
			"empty",
			"",
			[]string{},
		},
		{
			"one",
			"first",
			[]string{"first"},
		},
		{
			"multiple",
			"first, second",
			[]string{"first", "second"},
		},
		{
			"empty values removed",
			"first,,second,",
			[]string{"first", "second"},
		},
		{
			"names trimmed",
			" first, second,third ",
			[]string{"first", "second", "third"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := splitTargets(tc.targets)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_pickTargets(t *testing.T) {
	t.Run("error - targets not defined but one requested", func(t *testing.T) {
		// --- When ---
		have, err := pickTargets(nil, []string{"first"})

		// --- Then ---
		assert.ErrorIs(t, ErrNoTargets, err)
		assert.ErrorContain(t, "first", err)
		assert.Nil(t, have)
	})

	t.Run("error - targets not defined but two requested", func(t *testing.T) {
		// --- When ---
		have, err := pickTargets(nil, []string{"first", "second"})

		// --- Then ---
		assert.ErrorIs(t, ErrNoTargets, err)
		assert.ErrorContain(t, "first second", err)
		assert.Nil(t, have)
	})

	t.Run("error - contains only unknown target names", func(t *testing.T) {
		// --- Given ---
		tgs := []string{"first", "second", "third"}

		// --- When ---
		have, err := pickTargets(tgs, []string{"forth", "sixth"})

		// --- Then ---
		assert.ErrorIs(t, ErrNoTarget, err)
		assert.ErrorContain(t, "forth sixth", err)
		assert.Nil(t, have)
	})

	t.Run("error - contains some unknown target names", func(t *testing.T) {
		// --- Given ---
		tgs := []string{"first", "second", "third"}

		// --- When ---
		have, err := pickTargets(tgs, []string{"first", "forth", "second"})

		// --- Then ---
		assert.ErrorIs(t, ErrNoTarget, err)
		assert.ErrorContain(t, "[forth]", err)
		assert.Nil(t, have)
	})
}

func Test_pickTargets_tabular(t *testing.T) {
	tt := []struct {
		testN string

		haveTgs []string
		wantTgs []string
		want    []string
	}{
		{
			"targets not defined and not requested",
			nil,
			nil,
			nil,
		},
		{
			"targets defined but none requested",
			[]string{"first", "second", "third"},
			nil,
			[]string{"first", "second", "third"},
		},
		{
			"targets defined one requested",
			[]string{"first", "second", "third"},
			[]string{"second"},
			[]string{"second"},
		},
		{
			"targets defined two requested",
			[]string{"first", "second", "third"},
			[]string{"second", "first"},
			[]string{"first", "second"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, err := pickTargets(tc.haveTgs, tc.wantTgs)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_updateInfo(t *testing.T) {
	t.Run("empty Build slice", func(t *testing.T) {
		// --- Given ---
		inf := &gmprj.Info{}

		// --- When ---
		updateInfo(inf, []*Build{})

		// --- Then ---
		assert.Len(t, 0, inf.Custom())
	})

	t.Run("one Build instance", func(t *testing.T) {
		// --- Given ---
		bld0 := &Build{hidBC{
			name: "name0",
			tag:  "tag0",
			repo: "repo0",
		}}
		inf := &gmprj.Info{
			Other: make(map[string]string),
		}

		// --- When ---
		updateInfo(inf, []*Build{bld0})

		// --- Then ---
		want := []string{
			"C42_DKI_NAME=repo0/name0",
			"C42_DKI_REF=repo0/name0:tag0",
			"C42_DKI_TAG=tag0",
		}
		assert.Equal(t, want, inf.Custom())
	})

	t.Run("multiple Build instance", func(t *testing.T) {
		// --- Given ---
		bld0 := &Build{hidBC{
			name: "name0",
			tag:  "tag0",
			repo: "repo0",
		}}
		bld1 := &Build{hidBC{
			name: "name1",
			tag:  "tag1",
			repo: "repo1",
		}}
		inf := &gmprj.Info{
			Other: make(map[string]string),
		}

		// --- When ---
		updateInfo(inf, []*Build{bld0, bld1})

		// --- Then ---
		want := []string{
			"C42_DKI_NAMES=repo0/name0,repo1/name1",
			"C42_DKI_NAME_STEM=repo0/name0",
			"C42_DKI_REFS=repo0/name0:tag0,repo1/name1:tag1",
			"C42_DKI_TAG=tag0",
		}
		assert.Equal(t, want, inf.Custom())
	})
}

func Test_sshAuthSock(t *testing.T) {
	t.Run("is set", func(t *testing.T) {
		// --- Given ---
		env := []string{"SSH_AUTH_SOCK=socket"}

		// --- When ---
		have := sshAuthSock(env)

		// --- Then ---
		assert.Equal(t, "socket", have)
	})

	t.Run("has empty value", func(t *testing.T) {
		// --- Given ---
		env := []string{"SSH_AUTH_SOCK="}

		// --- When ---
		have := sshAuthSock(env)

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("value is expanded", func(t *testing.T) {
		// --- Given ---
		env := []string{"abc=value", "SSH_AUTH_SOCK=$abc"}

		// --- When ---
		have := sshAuthSock(env)

		// --- Then ---
		assert.Equal(t, "value", have)
	})

	t.Run("does not exist", func(t *testing.T) {
		// --- Given ---
		env := []string{"abc=value"}

		// --- When ---
		have := sshAuthSock(env)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_runDockerCmd(t *testing.T) {
	t.Run("docker version", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring("version")

		// --- When ---
		haveSO, haveEO, err := runDockerCmd(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		// "docker version" always prints a "Client:" section header; assert on
		// its presence rather than the exact output, which varies by release.
		assert.Contain(t, "Client:", haveSO)
		// The returned copy is the ring's stdout with surrounding whitespace
		// trimmed; comparing the trimmed streams is release independent.
		assert.Equal(t, strings.TrimSpace(tst.Stdout()), haveSO)
		assert.Empty(t, haveEO)
	})

	t.Run("error - unknown command", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring("unknown")

		// --- When ---
		haveSO, haveEO, err := runDockerCmd(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "docker: unknown command", err)
		assert.Contain(t, "docker: unknown command", haveEO)
		assert.Contain(t, "docker: unknown command", tst.Stderr())
		assert.Empty(t, haveSO)
	})

	t.Run("environment passed via context", func(t *testing.T) {
		// --- Given ---
		port := must.Value(netkit.GetFreePort())
		host := fmt.Sprintf("tcp://127.0.0.1:%d", port)

		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring("images", "ls")
		rng.EnvSet("DOCKER_HOST", host)

		// --- When ---
		haveSO, haveEO, err := runDockerCmd(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "Cannot connect to the Docker daemon", err)
		assert.Contain(t, "Cannot connect to the Docker daemon", haveEO)
		assert.Contain(t, "Cannot connect to the Docker daemon", tst.Stderr())
		assert.Empty(t, haveSO)
	})

	t.Run("arguments are interpolated", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring("$DKR_CMD")
		rng.EnvSet("DKR_CMD", "version")

		// --- When ---
		haveSO, haveEO, err := runDockerCmd(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Client:", haveSO)
		assert.Contain(t, "Client:", tst.Stdout())
		assert.Empty(t, haveEO)

		assert.Equal(t, []string{"$DKR_CMD"}, rng.Args())
	})
}

func Test_filterError_tabular(t *testing.T) {
	tt := []struct {
		testN string

		msg  string
		want string
	}{
		{"1", "abc\nERROR: def\nERROR: ghi\njkl", "ERROR: def\nERROR: ghi"},
		{"2", "ERROR: def\nERROR: ghi\n", "ERROR: def\nERROR: ghi"},
		{"3", "ERROR: def", "ERROR: def"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := filterError(tc.msg)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_dockerErrorOr_sentinel_tabular(t *testing.T) {
	tt := []struct {
		testN string

		msg  string
		want error
	}{
		{"1", "ERROR: failed to solve: target stage", ErrUnkTarget},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			err := dockerErrorOr(tc.msg, nil)

			// --- Then ---
			assert.ErrorIs(t, tc.want, err)
		})
	}
}

func Test_dockerErrorOr(t *testing.T) {
	t.Run("not recognized message", func(t *testing.T) {
		// --- When ---
		err := dockerErrorOr("abc", nil)

		// --- Then ---
		assert.ErrorEqual(t, "abc", err)
	})

	t.Run("empty message nil error", func(t *testing.T) {
		// --- When ---
		err := dockerErrorOr("", nil)

		// --- Then ---
		assert.ErrorEqual(t, "empty docker error message and nil error parameter", err)
	})

	t.Run("empty message custom error", func(t *testing.T) {
		// --- When ---
		err := dockerErrorOr("", errTest)

		// --- Then ---
		assert.ErrorIs(t, errTest, err)
	})
}

func Test_isRemoteSet_tabular(t *testing.T) {
	tt := []struct {
		testN string

		cfg  map[string]string
		want bool
	}{
		{"nil", nil, false},
		{"not set", map[string]string{}, false},
		{
			"only host key set but empty",
			map[string]string{CfgDkrRegHost: ""},
			false,
		},
		{
			"only repo key set but empty",
			map[string]string{CfgDkrRepo: ""},
			false,
		},
		{
			"host and repo key values empty",
			map[string]string{CfgDkrRegHost: "", CfgDkrRepo: ""},
			false,
		},
		{
			"host and repo key set",
			map[string]string{CfgDkrRegHost: "host", CfgDkrRepo: "repo"},
			true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isRemoteSet(tc.cfg)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_GetGIDbyName(t *testing.T) {
	if _, err := exec.LookPath("getent"); err != nil {
		t.Skip("getent not available")
	}

	t.Run("known group", func(t *testing.T) {
		// --- When ---
		have, err := GetGIDbyName("root")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, have)
	})

	t.Run("error - unknown group", func(t *testing.T) {
		// --- When ---
		have, err := GetGIDbyName("ctx42-no-such-group")

		// --- Then ---
		assert.Error(t, err)
		assert.Equal(t, 0, have)
	})
}

func Test_parseGetent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- When ---
		have, err := parseGetent("docker:x:992:thor")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 992, have)
	})

	t.Run("error - invalid format", func(t *testing.T) {
		// --- When ---
		have, err := parseGetent("docker:x:992:thor:")

		// --- Then ---
		assert.ErrorEqual(t, "unexpected getent response format", err)
		assert.Equal(t, 0, have)
	})

	t.Run("error - invalid group ID", func(t *testing.T) {
		// --- When ---
		have, err := parseGetent("docker:x:ABC:thor")

		// --- Then ---
		assert.ErrorContain(t, `parsing group id "ABC"`, err)
		assert.Equal(t, 0, have)
	})
}

func Test_DockerSocket(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	t.Run("returns the current context socket path", func(t *testing.T) {
		// --- When ---
		have := DockerSocket()

		// --- Then ---
		assert.NotEmpty(t, have)
		assert.True(t, strings.HasPrefix(have, "/"))
		assert.False(t, strings.Contains(have, "://"))
	})
}

func Test_deleteImage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		_, iid := dkrkit.NewT(t).Build()

		// --- When ---
		err := deleteImage(ctx, tst.Ring(), iid)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByID(iid))
	})

	t.Run("not existing ID", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := deleteImage(ctx, tst.Ring(), "not-existing")

		// --- Then ---
		assert.NoError(t, err)
	})
}
