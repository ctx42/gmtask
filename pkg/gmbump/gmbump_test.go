// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmbump

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xflag/pkg/xflag"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmgo"
	"github.com/ctx42/gmtask/pkg/lib/gmclog"
)

func Test_Bump(t *testing.T) {
	t.Run("current working directory", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()
		defer prj.ChdirBack()

		// --- When ---
		err := Bump(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		assert.Equal(t, "v0.0.1", cl.Releases[0].Version.Original())
	})
}

func Test_BumpTarget(t *testing.T) {
	t.Run("help flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("-h")

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :bump:\n" +
			"  -f, --force    release when origin cannot be checked\n" +
			"  -h, --help     show help\n" +
			"  -M, --major    force a major version bump\n" +
			"  -m, --minor    force a minor version bump\n" +
			"  -p, --patch    force a patch version bump\n" +
			"  -s, --set      release the given version without asking\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - invalid flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("-nope")

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined", err)
		assert.Contain(t, "Usage of :bump:", tst.Stderr())
	})

	t.Run("error - conflicting bump flags", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("-p", "--minor")

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrBumpFlags, err)
		assert.ErrorContain(t, "--patch and --minor", err)
	})

	t.Run("error - set with bump flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("-s", "v1.0.0", "-m")

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrBumpFlags, err)
		assert.ErrorContain(t, "--set and --minor", err)
	})

	t.Run("error - invalid set version on dirty tree", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("-s", "v1.2")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrBadVersion, err)
		assert.ErrorContain(t, `"v1.2"`, err)
	})

	t.Run("error - set metadata in go module", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring("-s", "v0.0.2+b")

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrBadVersion, err)
		assert.ErrorContain(t, "build metadata in a Go module", err)
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
	})

	t.Run("error - uncommitted changes", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotClean, err)
	})

	t.Run("error - untracked files", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file1 1", "file1.txt")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotClean, err)
	})

	t.Run("error - dirty tree on another branch", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotClean, err)
		assert.ErrorIsNot(t, ErrNotDefBranch, err)
	})

	t.Run("error - detached head", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.GitDetach()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrDetached, err)
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
		assert.Equal(t, "", prj.ExeStdout("git", "tag"))
	})

	t.Run("error - detached head on version tag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.GitDetach()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrDetached, err)
	})

	t.Run("main branch", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("main"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("push to origin", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("main"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		origin := t.TempDir()
		prj.Exe("git", "init", "--bare", origin)
		prj.GitSetRemote(origin)
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		head := prj.ExeStdout("git", "rev-parse", "HEAD")
		branch := prj.ExeStdout("git", "-C", origin, "rev-parse", "main")
		assert.Equal(t, "v0.0.1\n", prj.ExeStdout("git", "-C", origin, "tag"))
		assert.Equal(t, head, branch)
	})

	t.Run("approved other branch", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("y\n\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: \n" +
			"Release from branch \"feature/x\"? [y/N]: " +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("approval YES", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("YES\n\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "Release from branch \"feature/x\"? [y/N]: "
		assert.Contain(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("nothing to do off default branch", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.0.1\n" +
			"HEAD on tag. Nothing to do.\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("error - other branch declined", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotDefBranch, err)
		want := "" +
			"bump aborted: branch \"feature/x\" is not a default " +
			"branch (master, main)"
		assert.ErrorEqual(t, want, err)
		want = "" +
			"Current tag: \n" +
			"Release from branch \"feature/x\"? [y/N]: "
		assert.Equal(t, want, tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("error - empty answer", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotDefBranch, err)
		want := "" +
			"Current tag: \n" +
			"Release from branch \"feature/x\"? [y/N]: "
		assert.Equal(t, want, tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
	})

	t.Run("error - EOF reading approval", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)
		assert.ErrorIsNot(t, ErrNotDefBranch, err)
		assert.ErrorEqual(t, "read approval input: EOF", err)
		want := "" +
			"Current tag: \n" +
			"Release from branch \"feature/x\"? [y/N]: "
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("approval without newline", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("y")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorEqual(t, "read version input: EOF", err)
		want := "" +
			"Current tag: \n" +
			"Release from branch \"feature/x\"? [y/N]: " +
			"Enter a version number [v0.0.1]: "
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("invalid semver tag", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "not-sem-ver")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Skipping tag: \"not-sem-ver\"\n" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.1", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- Initial commit.", rel.Changes[0])
	})

	t.Run("skip to valid semver tag", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.5.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Exe("git", "tag", "not-sem-ver")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Skipping tag: \"not-sem-ver\"\n" +
			"Current tag: v0.5.0\n" +
			"Enter a version number [v0.5.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.5.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
		summary := prj.GitCommitLog().Latest().Summary
		assert.Equal(t, "Bump version to v0.5.1.", summary)
		format := "--format=%(contents)"
		msg := prj.ExeStdout("git", "tag", "-l", format, "v0.5.1")
		assert.Equal(t, "Tag version v0.5.1.\n\n", msg)
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.5.1", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 2.", rel.Changes[0])
	})

	t.Run("only initial commit no tags", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.1", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- Initial commit.", rel.Changes[0])
	})

	t.Run("previous commits but no tags", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.1", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 2, rel.Changes)
		assert.Equal(t, "- Initial commit.", rel.Changes[0])
		assert.Equal(t, "- test commit 2.", rel.Changes[1])
	})

	t.Run("previous commits and HEAD on tag", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Exe("git", "tag", "v0.0.1")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.0.1\n" +
			"HEAD on tag. Nothing to do.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("previous commits and tags", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Exe("git", "tag", "v0.0.1")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.GitCommit("", "test commit 3")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.0.1\n" +
			"Enter a version number [v0.0.2]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.2", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.2", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 3.", rel.Changes[0])
	})

	t.Run("bumping go module repo", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Exe("git", "tag", "v0.0.1")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.GitCommit("", "test commit 3")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.0.1\n" +
			"Enter a version number [v0.0.2]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n" +
			"\n" +
			"Use\n" +
			"\tgo get example.com/comp/project@v0.0.2\n" +
			"to update upstreams.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.2", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.2", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 3.", rel.Changes[0])
	})

	t.Run("bump patch version", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-p")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.1.0\n" +
			"Enter a version number [v0.1.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.1.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.1.1", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- feat: a feature.", rel.Changes[0])
	})

	t.Run("bump minor version", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-m")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "fix: a defect")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Enter a version number [v0.2.0]: ", tst.Stdout())
		assert.Equal(t, "v0.2.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("bump major version", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-M")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v1.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "fix: a defect")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Enter a version number [v2.0.0]: ", tst.Stdout())
		assert.Equal(t, "v2.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("major bump on 0 major", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("--major")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "fix: a defect")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Enter a version number [v0.2.0]: ", tst.Stdout())
		assert.Equal(t, "v0.2.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("force custom tag", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("v0.10.0\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("v0.0.1")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.0.1\n" +
			"Enter a version number [v0.0.2]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.10.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.10.0", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- commit 2.", rel.Changes[0])
	})

	t.Run("custom tag without v prefix", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("0.10.0\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("v0.0.1")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.0.1\n" +
			"Enter a version number [v0.0.2]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.10.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.10.0", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- commit 2.", rel.Changes[0])
	})

	t.Run("short version asked again", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("v1.2\nv1.0.0\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.0.1\n" +
			"Enter a version number [v0.0.2]: " +
			"Rejected: invalid version: \"v1.2\": invalid semantic version\n" +
			"Enter a version number [v0.0.2]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v1.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
		tags := prj.ExeStdout("git", "tag", "--list")
		assert.Equal(t, "v0.0.1\nv1.0.0\n", tags)
	})

	t.Run("not newer version asked again", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("v0.4.0\n\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.5.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.5.0\n" +
			"Enter a version number [v0.5.1]: " +
			"Rejected: version not newer than the current tag: " +
			"v0.4.0 is not newer than v0.5.0\n" +
			"Enter a version number [v0.5.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.5.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("error - EOF reading version", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: "
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("error - EOF after invalid version", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("not-a-version\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Rejected: invalid version: \"not-a-version\": " +
			"invalid semantic version\n" +
			"Enter a version number [v0.0.1]: "
		assert.Equal(t, want, tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("last answer without newline", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("v0.0.2\nok")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.2", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("error - EOF reading continue", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)
		assert.ErrorContain(t, "CHANGELOG.md and VER may be modified", err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("existing tag asked again", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("v0.0.2\nv0.0.3\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.Exe("git", "checkout", "-q", "-b", "other")
		prj.CreateFileWith("other", "other.txt")
		prj.GitCommit("v0.0.2")
		prj.Exe("git", "checkout", "-q", "-")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.0.1\n" +
			"Enter a version number [v0.0.2]: " +
			"Rejected: tag already exists: v0.0.2\n" +
			"Enter a version number [v0.0.2]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.3", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("error - origin unreachable", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.GitSetRemote(filepath.Join(t.TempDir(), "missing"))
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrRemote, err)
		assert.ErrorContain(t, "--force skips", err)
		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: "
		assert.Equal(t, want, tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("error - push fails when forced", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-f")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.GitSetRemote(filepath.Join(t.TempDir(), "missing"))
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		want := "release v0.0.1 committed and tagged locally; push it manually"
		assert.ErrorContain(t, want, err)
		assert.Contain(t, "Continuing.\n", tst.Stdout())
		assert.Equal(t, "v0.0.1\n", prj.ExeStdout("git", "tag"))
	})

	t.Run("set version", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-s", "v1.0.0")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "fix: a defect")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.1.0\n" +
			"Version: v1.0.0\n" +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v1.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
		tags := prj.ExeStdout("git", "tag", "--list")
		assert.Equal(t, "v0.1.0\nv1.0.0\n", tags)
	})

	t.Run("set version off default branch", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("y\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("--set", "1.0.0")

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: \n" +
			"Release from branch \"feature/x\"? [y/N]: " +
			"Version: v1.0.0\n" +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v1.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("set version with HEAD on tag", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-s", "v1.0.0")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v1.0.0-rc.1")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v1.0.0-rc.1\n" +
			"Version: v1.0.0\n" +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v1.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
		tags := prj.ExeStdout("git", "tag", "--list")
		assert.Equal(t, "v1.0.0\nv1.0.0-rc.1\n", tags)
	})

	t.Run("error - set version not newer with HEAD on tag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("-s", "v0.1.0")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotNewer, err)
		assert.ErrorContain(t, "v0.1.0 is not newer than v0.1.0", err)
		want := "" +
			"Current tag: v0.1.0\n" +
			"Version: v0.1.0\n"
		assert.Equal(t, want, tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("set version below proposal", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-s", "v0.1.1")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Current tag: v0.1.0\n" +
			"Version: v0.1.1\n" +
			"Warning: v0.1.1 is lower than the proposed v0.2.0.\n" +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.1.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("typed version below proposal", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("v0.1.1\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := "Warning: v0.1.1 is lower than the proposed v0.2.0.\n"
		assert.Contain(t, want, tst.Stdout())
		assert.Equal(t, "v0.1.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("set pre-release of proposal", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-s", "v0.2.0-rc.1")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.NotContain(t, "Warning:", tst.Stdout())
		have := oskit.ReadFileStr(t, prj.Root(), "VER")
		assert.Equal(t, "v0.2.0-rc.1", have)
	})

	t.Run("set version far ahead", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-s", "v7.0.0")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.NotContain(t, "Warning:", tst.Stdout())
		assert.Equal(t, "v7.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("set metadata outside go module", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring("-s", "v0.0.2+b")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Version: v0.0.2+b\n", tst.Stdout())
		assert.Equal(t, "v0.0.2+b", oskit.ReadFileStr(t, prj.Root(), "VER"))
		tags := prj.ExeStdout("git", "tag", "--list")
		assert.Equal(t, "v0.0.1\nv0.0.2+b\n", tags)
	})

	t.Run("error - set version not newer", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("-s", "v0.5.0")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.5.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotNewer, err)
		assert.ErrorContain(t, "v0.5.0 is not newer than v0.5.0", err)
		want := "" +
			"Current tag: v0.5.0\n" +
			"Version: v0.5.0\n"
		assert.Equal(t, want, tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("error - set version tag on other branch", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("-s", "v0.0.2")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.Exe("git", "checkout", "-q", "-b", "other")
		prj.CreateFileWith("other", "other.txt")
		prj.GitCommit("v0.0.2")
		prj.Exe("git", "checkout", "-q", "-")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrTagExists, err)
		assert.ErrorContain(t, "tag already exists: v0.0.2", err)
		assert.Contain(t, "Version: v0.0.2\n", tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("error - set version tag only on origin", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("-s", "v0.0.2")

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("main"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		origin := t.TempDir()
		prj.Exe("git", "init", "--bare", origin)
		prj.GitSetRemote(origin)
		prj.Exe("git", "push", "origin", "main")
		prj.Exe("git", "-C", origin, "tag", "v0.0.2", "main")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrTagExists, err)
		assert.ErrorContain(t, "tag already exists on origin: v0.0.2", err)
		assert.Contain(t, "Version: v0.0.2\n", tst.Stdout())
		assert.Empty(t, prj.ExeStdout("git", "tag", "--list"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("error - force keeps local tag check", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("-f", "-s", "v0.0.2")

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.Exe("git", "checkout", "-q", "-b", "other")
		prj.CreateFileWith("other", "other.txt")
		prj.GitCommit("v0.0.2")
		prj.Exe("git", "checkout", "-q", "-")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("")
		prj.GitSetRemote(filepath.Join(t.TempDir(), "missing"))
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrTagExists, err)
		assert.Contain(t, "Version: v0.0.2\n", tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
	})

	t.Run("changelog in reverse", func(t *testing.T) {
		// --- Given ---
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		content := "" +
			"## v0.0.0 (Fri, 07 Apr 2023 18:37:35 UTC)\n" +
			"- test commit 1.\n" +
			"\n"
		prj.CreateFileWith(content, "CHANGELOG.md")
		prj.GitInitAddAll("v0.0.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Close()

		// --- When ---
		err := BumpTarget(t.Context(), rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 2, cl.Releases)
		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.1", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 2.", rel.Changes[0])
		rel = cl.Releases[1]
		assert.Equal(t, "v0.0.0", rel.Version.Original())
		assert.Within(t, "2023-04-07T18:37:35Z", "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 1.", rel.Changes[0])
		want := "" +
			"Current tag: v0.0.0\n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"No remote configured; skip push.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
	})
}

func Test_guard_choose(t *testing.T) {
	t.Run("set version", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		grd := guard{repo: prj.Root()}
		out := &bytes.Buffer{}
		set := semver.MustParse("v1.0.0")
		proposal := semver.MustParse("v0.0.1")

		// --- When ---
		have, err := grd.choose(t.Context(), out, nil, set, proposal)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, set, have)
		assert.Equal(t, "Version: v1.0.0\n", out.String())
	})

	t.Run("typed version below proposal", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		grd := guard{repo: prj.Root()}
		out := &bytes.Buffer{}
		rdr := bufio.NewReader(bytes.NewBufferString("v0.1.9-rc.1\n"))
		proposal := semver.MustParse("v0.2.0")

		// --- When ---
		have, err := grd.choose(t.Context(), out, rdr, nil, proposal)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.9-rc.1", have.Original())
		want := "" +
			"Enter a version number [v0.2.0]: " +
			"Warning: v0.1.9-rc.1 is lower than the proposed v0.2.0.\n"
		assert.Equal(t, want, out.String())
	})

	t.Run("error - set version refused", func(t *testing.T) {
		// --- Given ---
		grd := guard{cur: "v1.0.0"}
		out := &bytes.Buffer{}
		set := semver.MustParse("v0.9.0")
		proposal := semver.MustParse("v1.0.1")

		// --- When ---
		have, err := grd.choose(t.Context(), out, nil, set, proposal)

		// --- Then ---
		assert.ErrorIs(t, ErrNotNewer, err)
		assert.Nil(t, have)
		assert.Equal(t, "Version: v0.9.0\n", out.String())
	})
}

func Test_guard_ask(t *testing.T) {
	t.Run("empty answer takes default", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		grd := guard{repo: prj.Root()}
		out := &bytes.Buffer{}
		rdr := bufio.NewReader(bytes.NewBufferString("\n"))
		def := semver.MustParse("v0.0.1")

		// --- When ---
		have, err := grd.ask(t.Context(), out, rdr, def)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.0.1", have.Original())
		assert.Equal(t, "Enter a version number [v0.0.1]: ", out.String())
	})

	t.Run("padded version asked again", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		grd := guard{repo: prj.Root()}
		out := &bytes.Buffer{}
		rdr := bufio.NewReader(bytes.NewBufferString(" 1.2.3\r\n1.2.3\r\n"))
		def := semver.MustParse("v0.0.1")

		// --- When ---
		have, err := grd.ask(t.Context(), out, rdr, def)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.2.3", have.Original())
		want := "" +
			"Enter a version number [v0.0.1]: " +
			"Rejected: invalid version: \" 1.2.3\": " +
			"invalid characters in version\n" +
			"Enter a version number [v0.0.1]: "
		assert.Equal(t, want, out.String())
	})

	t.Run("error - check fails", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.Close()

		grd := guard{repo: prj.Root()}
		out := &bytes.Buffer{}
		rdr := bufio.NewReader(bytes.NewBufferString("v1.0.0\n"))
		def := semver.MustParse("v0.0.1")

		// --- When ---
		have, err := grd.ask(t.Context(), out, rdr, def)

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
		assert.ErrorContain(t, "check tags", err)
		assert.Nil(t, have)
		assert.Equal(t, "Enter a version number [v0.0.1]: ", out.String())
	})
}

func Test_guard_check(t *testing.T) {
	t.Run("no current tag", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		grd := guard{repo: prj.Root()}

		// --- When ---
		err := grd.check(t.Context(), semver.MustParse("v0.0.1"))

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - same core with other metadata", func(t *testing.T) {
		// --- Given ---
		grd := guard{cur: "v1.2.3+a"}

		// --- When ---
		err := grd.check(t.Context(), semver.MustParse("v1.2.3+b"))

		// --- Then ---
		assert.ErrorIs(t, ErrNotNewer, err)
	})

	t.Run("unreachable origin forced", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		missing := filepath.Join(t.TempDir(), "missing")
		prj.GitSetRemote(missing)
		prj.Close()

		grd := guard{repo: prj.Root(), origin: missing, force: true}

		// --- When ---
		err := grd.check(t.Context(), semver.MustParse("v0.0.1"))

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.Close()

		grd := guard{repo: prj.Root()}

		// --- When ---
		err := grd.check(t.Context(), semver.MustParse("v0.0.1"))

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
		assert.ErrorContain(t, "check tags", err)
	})
}

func Test_parseVersion(t *testing.T) {
	t.Run("v prefix", func(t *testing.T) {
		// --- When ---
		have, err := parseVersion("v1.2.3", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.2.3", have.Original())
	})

	t.Run("no v prefix", func(t *testing.T) {
		// --- When ---
		have, err := parseVersion("1.2.3", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.2.3", have.Original())
	})

	t.Run("pre-release in go module", func(t *testing.T) {
		// --- When ---
		have, err := parseVersion("v1.0.0-rc.1", true)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.0.0-rc.1", have.Original())
	})

	t.Run("metadata outside go module", func(t *testing.T) {
		// --- When ---
		have, err := parseVersion("1.0.0-rc.1+b.5", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.0.0-rc.1+b.5", have.Original())
	})

	t.Run("error - metadata in go module", func(t *testing.T) {
		// --- When ---
		have, err := parseVersion("v1.2.3+b", true)

		// --- Then ---
		assert.ErrorIs(t, ErrBadVersion, err)
		assert.ErrorContain(t, "build metadata in a Go module", err)
		assert.Nil(t, have)
	})
}

func Test_parseVersion_tabular(t *testing.T) {
	tt := []struct {
		testN string

		txt string
	}{
		{"empty", ""},
		{"major only", "1"},
		{"no patch", "v1.2"},
		{"leading zero major", "01.2.3"},
		{"leading zero minor", "1.02.3"},
		{"leading zero pre-release", "1.2.3-rc.01"},
		{"upper case prefix", "V1.2.3"},
		{"double prefix", "vv1.2.3"},
		{"leading space", " 1.2.3"},
		{"trailing space", "1.2.3 "},
		{"four parts", "1.2.3.4"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, err := parseVersion(tc.txt, false)

			// --- Then ---
			assert.ErrorIs(t, ErrBadVersion, err)
			assert.Nil(t, have)
		})
	}
}

func Test_nextRelease(t *testing.T) {
	t.Run("development rev", func(t *testing.T) {
		// --- Given ---
		ver := gitaid.Version{Rev: "v0.4.1-dev.3.dirty+g7f93fb4"}

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.4.1", have.Original())
	})

	t.Run("release rev", func(t *testing.T) {
		// --- Given ---
		ver := gitaid.Version{Rev: "v1.2.3"}

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.2.3", have.Original())
	})

	t.Run("error - not a version", func(t *testing.T) {
		// --- Given ---
		ver := gitaid.Version{Rev: "nightly"}

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.ErrorIs(t, semver.ErrInvalidSemVer, err)
		assert.Nil(t, have)
	})

	t.Run("fix commit", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.9.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "fix: a defect")
		prj.Close()

		ver := must.Value(gmgo.ProjectVersion(t.Context(), rng, prj.Root(), ""))

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.9.1", have.Original())
	})

	t.Run("feat commit", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.9.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		ver := must.Value(gmgo.ProjectVersion(t.Context(), rng, prj.Root(), ""))

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.10.0", have.Original())
	})

	t.Run("forced patch", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		bump := gitaid.BumpPatch
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.9.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		ver := must.Value(gmgo.ProjectVersion(ctx, rng, prj.Root(), bump))

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.9.1", have.Original())
	})

	t.Run("non-version tag skipped", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.9.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("not-sem-ver", "fix: a defect")
		prj.Close()

		ver := must.Value(gmgo.ProjectVersion(t.Context(), rng, prj.Root(), ""))

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.9.0", ver.Tag)
		assert.Equal(t, "v0.9.1", have.Original())
	})
}

func Test_setVersion(t *testing.T) {
	t.Run("not given", func(t *testing.T) {
		// --- Given ---
		fs := xflag.NewFlagSet("test", flag.ContinueOnError)
		fs.StringSL("set", "s", "", "")
		must.Nil(fs.Parse(nil))

		// --- When ---
		have, err := setVersion(fs, "", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, have)
	})

	t.Run("given", func(t *testing.T) {
		// --- Given ---
		fs := xflag.NewFlagSet("test", flag.ContinueOnError)
		fs.StringSL("set", "s", "", "")
		must.Nil(fs.Parse([]string{"-s", "1.2.3"}))

		// --- When ---
		have, err := setVersion(fs, "", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.2.3", have.Original())
	})

	t.Run("error - empty", func(t *testing.T) {
		// --- Given ---
		fs := xflag.NewFlagSet("test", flag.ContinueOnError)
		fs.StringSL("set", "s", "", "")
		must.Nil(fs.Parse([]string{"--set", ""}))

		// --- When ---
		have, err := setVersion(fs, "", false)

		// --- Then ---
		assert.ErrorIs(t, ErrBadVersion, err)
		assert.Nil(t, have)
	})

	t.Run("error - bump level forced", func(t *testing.T) {
		// --- Given ---
		fs := xflag.NewFlagSet("test", flag.ContinueOnError)
		fs.StringSL("set", "s", "", "")
		must.Nil(fs.Parse([]string{"-s", "1.2.3"}))

		// --- When ---
		have, err := setVersion(fs, gitaid.BumpMajor, false)

		// --- Then ---
		assert.ErrorIs(t, ErrBumpFlags, err)
		assert.ErrorContain(t, "--set and --major", err)
		assert.Nil(t, have)
	})
}

func Test_forcedBump(t *testing.T) {
	t.Run("error - two levels forced", func(t *testing.T) {
		// --- Given ---
		fs := xflag.NewFlagSet("test", flag.ContinueOnError)
		fs.BoolSL(gitaid.BumpPatch, "p", false, "")
		fs.BoolSL(gitaid.BumpMinor, "m", false, "")
		fs.BoolSL(gitaid.BumpMajor, "M", false, "")
		must.Nil(fs.Parse([]string{"-m", "-M"}))

		// --- When ---
		have, err := forcedBump(fs)

		// --- Then ---
		assert.ErrorIs(t, ErrBumpFlags, err)
		assert.ErrorContain(t, "--minor and --major", err)
		assert.Empty(t, have)
	})
}

func Test_forcedBump_tabular(t *testing.T) {
	tt := []struct {
		testN string

		args []string
		want string
	}{
		{"none", nil, ""},
		{"patch", []string{"-p"}, gitaid.BumpPatch},
		{"minor", []string{"--minor"}, gitaid.BumpMinor},
		{"major", []string{"-M"}, gitaid.BumpMajor},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			fs := xflag.NewFlagSet("test", flag.ContinueOnError)
			fs.BoolSL(gitaid.BumpPatch, "p", false, "")
			fs.BoolSL(gitaid.BumpMinor, "m", false, "")
			fs.BoolSL(gitaid.BumpMajor, "M", false, "")
			must.Nil(fs.Parse(tc.args))

			// --- When ---
			have, err := forcedBump(fs)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have)
		})
	}
}
