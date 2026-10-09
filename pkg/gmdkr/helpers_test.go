// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/dkrkit"
	"github.com/ctx42/testkit/pkg/netkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmprj"
)

func Test_ImgName_tabular(t *testing.T) {
	tt := []struct {
		testN string

		projectName string
		want        string
	}{
		{"empty", "", ""},
		{"prefix added", "acme-proj", "dki-acme-proj"},
		{"prefix present", "dki-acme-proj", "dki-acme-proj"},
		{"infix present", "acme-dki-proj", "acme-dki-proj"},
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

func Test_ImgTag_tabular(t *testing.T) {
	tt := []struct {
		testN string

		rev  string
		want string
	}{
		{"release tag is untouched", "v1.2.3", "v1.2.3"},
		{
			"build metadata separator",
			"v0.4.1-dev.3+g7f93fb4",
			"v0.4.1-dev.3_g7f93fb4",
		},
		{
			"dirty development version",
			"v0.4.1-dev.3.dirty+g7f93fb4",
			"v0.4.1-dev.3.dirty_g7f93fb4",
		},
		{"pre-release without metadata", "v1.0.0-rc.1", "v1.0.0-rc.1"},
		{"empty", "", ""},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := ImgTag(tc.rev)

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
		// --- Given ---
		wantTgs := []string{"first"}

		// --- When ---
		have, err := pickTargets(nil, wantTgs)

		// --- Then ---
		assert.ErrorIs(t, ErrNoTargets, err)
		assert.ErrorContain(t, "first", err)
		assert.Nil(t, have)
	})

	t.Run("error - targets not defined but two requested", func(t *testing.T) {
		// --- Given ---
		wantTgs := []string{"first", "second"}

		// --- When ---
		have, err := pickTargets(nil, wantTgs)

		// --- Then ---
		assert.ErrorIs(t, ErrNoTargets, err)
		assert.ErrorContain(t, "first second", err)
		assert.Nil(t, have)
	})

	t.Run("error - contains only unknown target names", func(t *testing.T) {
		// --- Given ---
		tgs := []string{"first", "second", "third"}
		wantTgs := []string{"forth", "sixth"}

		// --- When ---
		have, err := pickTargets(tgs, wantTgs)

		// --- Then ---
		assert.ErrorIs(t, ErrNoTarget, err)
		assert.ErrorContain(t, "forth sixth", err)
		assert.Nil(t, have)
	})

	t.Run("error - contains some unknown target names", func(t *testing.T) {
		// --- Given ---
		tgs := []string{"first", "second", "third"}
		wantTgs := []string{"first", "forth", "second"}

		// --- When ---
		have, err := pickTargets(tgs, wantTgs)

		// --- Then ---
		assert.ErrorIs(t, ErrNoTarget, err)
		assert.ErrorContain(t, "[forth]", err)
		assert.Nil(t, have)
	})
}

func Test_pickTargets_duplicate(t *testing.T) {
	// --- When ---
	have, err := pickTargets([]string{"a", "b"}, []string{"a", "a"})

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, []string{"a"}, have)
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
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("version")

		// --- When ---
		hSO, hEO, err := runDockerCmd(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		// "docker version" always prints a "Client:" section header; assert on
		// its presence rather than the exact output, which varies by release.
		assert.Contain(t, "Client:", hSO)
		// The returned copy is the ring's stdout with surrounding whitespace
		// trimmed; comparing the trimmed streams is release independent.
		assert.Equal(t, strings.TrimSpace(tst.Stdout()), hSO)
		assert.Empty(t, hEO)
	})

	t.Run("error - unknown command", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("unknown")

		// --- When ---
		hSO, hEO, err := runDockerCmd(t.Context(), rng)

		// --- Then ---
		assert.ErrorContain(t, "docker: unknown command", err)
		assert.Contain(t, "docker: unknown command", hEO)
		assert.Contain(t, "docker: unknown command", tst.Stderr())
		assert.Empty(t, hSO)
	})

	t.Run("error - docker host from ring", func(t *testing.T) {
		// --- Given ---
		port := must.Value(netkit.GetFreePort())
		host := fmt.Sprintf("tcp://127.0.0.1:%d", port)

		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("images", "ls")

		rng.EnvSet("DOCKER_HOST", host)

		// --- When ---
		hSO, hEO, err := runDockerCmd(t.Context(), rng)

		// --- Then ---
		assert.ErrorContain(t, "Cannot connect to the Docker daemon", err)
		assert.Contain(t, "Cannot connect to the Docker daemon", hEO)
		assert.Contain(t, "Cannot connect to the Docker daemon", tst.Stderr())
		assert.Empty(t, hSO)
	})

	t.Run("arguments are interpolated", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("$DKR_CMD")

		rng.EnvSet("DKR_CMD", "version")

		// --- When ---
		hSO, hEO, err := runDockerCmd(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Client:", hSO)
		assert.Contain(t, "Client:", tst.Stdout())
		assert.Empty(t, hEO)
		assert.Equal(t, []string{"$DKR_CMD"}, rng.Args())
	})
}

func Test_runDockerCmd_context_done(t *testing.T) {
	// --- Given ---
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	rng := ringtest.New(t).Ring("events")

	// --- When ---
	_, _, err := runDockerCmd(ctx, rng)

	// --- Then ---
	assert.ErrorIs(t, context.DeadlineExceeded, err)
}

func Test_filterError_tabular(t *testing.T) {
	tt := []struct {
		testN string

		msg  string
		want string
	}{
		{
			"error lines among others",
			"abc\nERROR: def\nERROR: ghi\njkl",
			"ERROR: def\nERROR: ghi",
		},
		{
			"trailing newline",
			"ERROR: def\nERROR: ghi\n",
			"ERROR: def\nERROR: ghi",
		},
		{"single error line", "ERROR: def", "ERROR: def"},
		{"no error lines", "a\nb", "a\nb"},
		{
			"long output keeps last lines",
			strings.Repeat("x\n", 30) + strings.Repeat("y\n", 20),
			strings.TrimSuffix(strings.Repeat("y\n", 20), "\n"),
		},
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
		want := "empty docker error message and nil error parameter"
		assert.ErrorEqual(t, want, err)
	})

	t.Run("message and error", func(t *testing.T) {
		// --- When ---
		err := dockerErrorOr("abc", errTest)

		// --- Then ---
		assert.ErrorIs(t, errTest, err)
		assert.ErrorEqual(t, "abc: "+errTest.Error(), err)
	})

	t.Run("sentinel keeps error", func(t *testing.T) {
		// --- When ---
		err := dockerErrorOr("ERROR: failed to solve: target stage", errTest)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTarget, err)
		assert.ErrorIs(t, errTest, err)
	})

	t.Run("empty message custom error", func(t *testing.T) {
		// --- When ---
		err := dockerErrorOr("", errTest)

		// --- Then ---
		assert.ErrorIs(t, errTest, err)
	})
}

func Test_dockerErrorOr_tabular(t *testing.T) {
	tt := []struct {
		testN string

		msg  string
		want error
	}{
		{
			"unknown target stage",
			"ERROR: failed to solve: target stage",
			ErrUnkTarget,
		},
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

func Test_isFinal_tabular(t *testing.T) {
	tt := []struct {
		testN string

		tag  string
		want bool
	}{
		{"release", "v1.2.3", true},
		{"release without v", "1.2.3", true},
		{"release with build metadata", "v1.2.3+meta", true},
		{"pre-release", "v1.0.0-rc.1", false},
		{"development version", "v1.2.4-dev.3+g7f93fb4", false},
		{"not a version", "nightly", false},
		{"empty", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isFinal(tc.tag)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
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
			map[string]string{xdef.EnvRegHost: ""},
			false,
		},
		{
			"only repo key set but empty",
			map[string]string{xdef.EnvRegRepo: ""},
			false,
		},
		{
			"host and repo key values empty",
			map[string]string{xdef.EnvRegHost: "", xdef.EnvRegRepo: ""},
			false,
		},
		{
			"host and repo key set",
			map[string]string{xdef.EnvRegHost: "host", xdef.EnvRegRepo: "repo"},
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
		assert.ErrorContain(t, `getent group "ctx42-no-such-group"`, err)
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
		assert.ErrorContain(t, "unexpected getent response format: ", err)
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
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		_, iid := dkrkit.NewT(t).Build()

		// --- When ---
		err := deleteImage(t.Context(), tst.Ring(), iid)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, dkrkit.NewT(t).ImgLs().FindByID(iid))
	})

	t.Run("not existing ID", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.WithDockerfile()
		prj.Close()
		prj.Chdir()

		// --- When ---
		err := deleteImage(t.Context(), tst.Ring(), "not-existing")

		// --- Then ---
		assert.NoError(t, err)
	})
}

func Test_splitArgs_tabular(t *testing.T) {
	tt := []struct {
		testN string

		cmd  string
		want []string
	}{
		{"empty", "", nil},
		{"plain", "/bin/sh --login", []string{"/bin/sh", "--login"}},
		{"extra spaces", "  a   b ", []string{"a", "b"}},
		{
			"double quotes",
			`bash -c "ls -la"`,
			[]string{"bash", "-c", "ls -la"},
		},
		{
			"single quotes",
			`sh -c 'echo "$HOME"'`,
			[]string{"sh", "-c", `echo "$HOME"`},
		},
		{"escaped space", `a\ b c`, []string{"a b", "c"}},
		{"empty quoted", `a "" b`, []string{"a", "", "b"}},
		{"adjacent parts", `a"b c"d`, []string{"ab cd"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, err := splitArgs(tc.cmd)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_splitArgs(t *testing.T) {
	t.Run("error - unterminated quote", func(t *testing.T) {
		// --- When ---
		have, err := splitArgs(`bash -c "ls`)

		// --- Then ---
		assert.ErrorContain(t, "unterminated quote", err)
		assert.Nil(t, have)
	})
}
