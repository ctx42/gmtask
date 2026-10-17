// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/dkrkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmprj"
)

// runFixture holds the Docker resources a test of [Run.Clean] creates: those
// labeled with the run ID, one labeled with another run's ID, and an
// unlabeled image the ids file lists.
type runFixture struct {
	runID   string // The run ID the resources are labeled with.
	ctr     string // Container labeled with runID.
	net     string // Network labeled with runID.
	vol     string // Volume labeled with runID.
	img     string // Image labeled with runID.
	other   string // Container labeled with another run's ID.
	listed  string // Unlabeled image the ids file lists.
	idsFile string // Path of the ids file.
}

// newRunFixture creates the resources of a [runFixture] and registers their
// removal with t.Cleanup.
func newRunFixture(t tester.T) runFixture {
	t.Helper()
	dkrkit.NewT(t).ImgPull(dkrkit.TestImgRef)
	runID := "gmtask-test-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	lab := xdef.LabCCRunID + "=" + runID
	dkf := "FROM " + dkrkit.TestImgRef + "\n"

	fix := runFixture{runID: runID}
	fix.img = dockerCLI(t, dkf, "build", "--quiet", "--label", lab, "-")
	fix.ctr = dockerCLI(t, "", "create", "--label", lab, fix.img, "true")
	fix.net = dockerCLI(t, "", "network", "create", "--label", lab, runID)
	fix.vol = dockerCLI(t, "", "volume", "create", "--label", lab, runID)
	other := xdef.LabCCRunID + "=other-" + runID
	fix.other = dockerCLI(
		t,
		"",
		"create",
		"--label", other,
		dkrkit.TestImgRef,
		"true",
	)
	listed := "gmtask.test.listed=" + runID
	fix.listed = dockerCLI(t, dkf, "build", "--quiet", "--label", listed, "-")

	fix.idsFile = filepath.Join(t.TempDir(), "ids")
	gone := "sha256:" + strings.Repeat("0", 64)
	ids := "" +
		fix.listed + " example.com/listed:v1\n" +
		gone + " example.com/gone:v1\n"
	oskit.Write(t, ids, fix.idsFile)

	t.Cleanup(func() {
		dockerTry("container", "rm", "--force", fix.ctr, fix.other)
		dockerTry("network", "rm", fix.net)
		dockerTry("volume", "rm", fix.vol)
		dockerTry("image", "rm", "--force", fix.img, fix.listed)
	})
	return fix
}

func Test_Run_Clean(t *testing.T) {
	t.Run("removes the run's resources", func(t *testing.T) {
		// --- Given ---
		fix := newRunFixture(t)

		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--ids-file", fix.idsFile)
		rng.EnvSet(xdef.EnvCCRunID, fix.runID)

		// --- When ---
		err := Run{}.Clean(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "docker container rm --force "+fix.ctr, tst.Stderr())
		assert.Contain(t, "docker image rm --force "+fix.listed, tst.Stderr())
		assert.False(t, dockerHas("container", fix.ctr))
		assert.False(t, dockerHas("network", fix.net))
		assert.False(t, dockerHas("volume", fix.vol))
		assert.False(t, dockerHas("image", fix.img))
		assert.False(t, dockerHas("image", fix.listed))
		assert.True(t, dockerHas("container", fix.other))
		assert.NoFileExist(t, fix.idsFile)
	})

	t.Run("dry run removes nothing", func(t *testing.T) {
		// --- Given ---
		fix := newRunFixture(t)

		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--dry-run", "--ids-file", fix.idsFile)
		rng.EnvSet(xdef.EnvCCRunID, fix.runID)

		// --- When ---
		err := Run{}.Clean(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "docker volume rm "+fix.vol, tst.Stderr())
		assert.True(t, dockerHas("container", fix.ctr))
		assert.True(t, dockerHas("volume", fix.vol))
		assert.True(t, dockerHas("image", fix.listed))
		assert.FileExist(t, fix.idsFile)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := Run{}.Clean(t.Context(), tst.Ring("--help"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Usage of :docker:run:clean:", tst.Stderr())
	})

	t.Run("error - no run ID", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t, ring.WithEnv([]string{"HOME=/home"}))

		// --- When ---
		err := Run{}.Clean(t.Context(), tst.Ring())

		// --- Then ---
		assert.ErrorIs(t, ErrNoRunID, err)
		assert.ErrorEqual(t, "no CI/CD run ID: set C42_CC_RUN_ID", err)
	})

	t.Run("error - too many arguments", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		// --- When ---
		err := Run{}.Clean(t.Context(), tst.Ring("extra"))

		// --- Then ---
		assert.ErrorIs(t, gmprj.ErrTooManyArgs, err)
	})

	t.Run("error - unknown flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := Run{}.Clean(t.Context(), tst.Ring("--unknown"))

		// --- Then ---
		assert.ErrorContain(t, "-unknown", err)
		assert.Contain(t, "Usage of :docker:run:clean:", tst.Stderr())
	})
}

func Test_cleanRun(t *testing.T) {
	t.Run("missing ids file", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		fls := &Flags{IDsFile: filepath.Join(t.TempDir(), "missing")}

		// --- When ---
		err := cleanRun(t.Context(), tst.Ring(), "gmtask-test-none", fls)

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - unreadable ids file", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		fls := &Flags{IDsFile: t.TempDir()} // A directory is not readable.

		// --- When ---
		err := cleanRun(t.Context(), tst.Ring(), "gmtask-test-none", fls)

		// --- Then ---
		assert.ErrorContain(t, "read ids file", err)
	})
}

func Test_removeResource(t *testing.T) {
	t.Run("already gone counts as removed", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rm := []string{"container", "rm", "--force"}

		// --- When ---
		err := removeResource(t.Context(), tst.Ring(), rm, "gone-0", false)

		// --- Then ---
		assert.NoError(t, err)
		want := "#gomake INFO# docker container rm --force gone-0\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("dry run only prints", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rm := []string{"no-such-command"}

		// --- When ---
		err := removeResource(t.Context(), tst.Ring(), rm, "id", true)

		// --- Then ---
		assert.NoError(t, err)
		want := "#gomake INFO# docker no-such-command id\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - other failure", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rm := []string{"no-such-command"}

		// --- When ---
		err := removeResource(t.Context(), tst.Ring(), rm, "id", false)

		// --- Then ---
		assert.Error(t, err)
		assert.False(t, isGone(err))
		assert.Contain(t, "docker no-such-command id", tst.Stderr())
	})
}

func Test_isGone_tabular(t *testing.T) {
	tt := []struct {
		testN string

		msg  string
		want bool
	}{
		{"container", "Error response: No such container: c1", true},
		{"image", "Error response: No such image: i1", true},
		{"volume", "Error response: get v1: no such volume", true},
		{"network", "Error response: network n1 not found", true},
		{"other", "permission denied", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isGone(errors.New(tc.msg))

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_dockerQuiet(t *testing.T) {
	t.Run("returns output and prints nothing", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		args := []string{"version", "--format", "{{.Client.Os}}"}

		// --- When ---
		have, err := dockerQuiet(t.Context(), tst.Ring(), args...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "linux", have)
	})

	t.Run("error", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		// --- When ---
		have, err := dockerQuiet(t.Context(), tst.Ring(), "no-such-command")

		// --- Then ---
		assert.Error(t, err)
		assert.Empty(t, have)
	})
}

func Test_recordImageID(t *testing.T) {
	t.Run("appends the image ID and reference", func(t *testing.T) {
		// --- Given ---
		dkrkit.NewT(t).ImgPull(dkrkit.TestImgRef)
		tst := ringtest.New(t)
		pth := oskit.Write(t, "sha256:1 example.com/a:v1\n", t.TempDir(), "ids")
		ref := dkrkit.TestImgRef

		// --- When ---
		err := recordImageID(t.Context(), tst.Ring(), pth, ref)

		// --- Then ---
		assert.NoError(t, err)
		id := dockerCLI(t, "", "image", "inspect", "--format={{.Id}}", ref)
		want := "sha256:1 example.com/a:v1\n" + id + " " + ref + "\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, pth))
	})

	t.Run("error - unknown image", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		pth := filepath.Join(t.TempDir(), "ids")
		ref := "example.com/no-such-image:v0"

		// --- When ---
		err := recordImageID(t.Context(), tst.Ring(), pth, ref)

		// --- Then ---
		assert.ErrorContain(t, "record image "+ref, err)
		assert.NoFileExist(t, pth)
	})

	t.Run("error - file cannot be opened", func(t *testing.T) {
		// --- Given ---
		dkrkit.NewT(t).ImgPull(dkrkit.TestImgRef)
		tst := ringtest.New(t)
		pth := t.TempDir() // A directory cannot be opened for writing.

		// --- When ---
		err := recordImageID(t.Context(), tst.Ring(), pth, dkrkit.TestImgRef)

		// --- Then ---
		assert.ErrorContain(t, "record image "+dkrkit.TestImgRef, err)
	})
}

func Test_forgetImageRef(t *testing.T) {
	t.Run("removes the lines of the reference", func(t *testing.T) {
		// --- Given ---
		ids := "" +
			"sha256:1 example.com/a:v1\n" +
			"sha256:2 example.com/b:v1\n" +
			"sha256:3 example.com/a:v1\n"
		pth := oskit.Write(t, ids, t.TempDir(), "ids")

		// --- When ---
		err := forgetImageRef(pth, "example.com/a:v1")

		// --- Then ---
		assert.NoError(t, err)
		want := "sha256:2 example.com/b:v1\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, pth))
	})

	t.Run("missing file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "ids")

		// --- When ---
		err := forgetImageRef(pth, "example.com/a:v1")

		// --- Then ---
		assert.NoError(t, err)
		assert.NoFileExist(t, pth)
	})

	t.Run("error - unreadable file", func(t *testing.T) {
		// --- When ---
		err := forgetImageRef(t.TempDir(), "example.com/a:v1")

		// --- Then ---
		assert.ErrorContain(t, "forget image example.com/a:v1", err)
	})
}

func Test_readImageIDs(t *testing.T) {
	t.Run("IDs once in file order", func(t *testing.T) {
		// --- Given ---
		ids := "" +
			"sha256:2 example.com/b:v1\n" +
			"\n" +
			"sha256:1 example.com/a:v1\n" +
			"sha256:2 example.com/c:v1\n"
		pth := oskit.Write(t, ids, t.TempDir(), "ids")

		// --- When ---
		have, err := readImageIDs(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"sha256:2", "sha256:1"}, have)
	})

	t.Run("missing file", func(t *testing.T) {
		// --- When ---
		have, err := readImageIDs(filepath.Join(t.TempDir(), "ids"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, have)
	})

	t.Run("error - unreadable file", func(t *testing.T) {
		// --- When ---
		have, err := readImageIDs(t.TempDir())

		// --- Then ---
		assert.ErrorContain(t, "read ids file", err)
		assert.False(t, errors.Is(err, fs.ErrNotExist))
		assert.Nil(t, have)
	})
}

func Test_uniqueFields_tabular(t *testing.T) {
	tt := []struct {
		testN string

		str  string
		want []string
	}{
		{"empty", "", nil},
		{"one per line", "a\nb\n", []string{"a", "b"}},
		{"duplicates keep first", "b a b\na", []string{"b", "a"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := uniqueFields(tc.str)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
