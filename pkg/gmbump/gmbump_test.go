// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmbump

import (
	"bytes"
	"context"
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

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmgo"
	"github.com/ctx42/gmtask/pkg/lib/gmclog"
)

func Test_Bump(t *testing.T) {
	t.Run("bumps the current working directory", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()
		prj.Chdir()
		defer prj.ChdirBack()

		rng := tst.Ring()

		// --- When ---
		err := Bump(ctx, rng)

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
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring("-h")

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Usage of :bump:", tst.Stderr())
	})

	t.Run("error - invalid flag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring("-nope")

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined", err)
		assert.Contain(t, "Usage of :bump:", tst.Stderr())
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
	})

	t.Run("error - repo has uncommitted changes", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotClean, err)
	})

	t.Run("error - repo has untracked files", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file1 1", "file1.txt")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotClean, err)
	})

	t.Run("error - dirty tree on another branch", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotClean, err)
		assert.ErrorIsNot(t, ErrNotDefBranch, err)
	})

	t.Run("error - detached head", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.GitDetach()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrDetached, err)
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
		assert.Equal(t, "", prj.ExeStdout("git", "tag"))
	})

	t.Run("error - detached head on a version tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.GitDetach()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrDetached, err)
	})

	t.Run("releases from main without asking", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("main"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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

	t.Run("pushes the release to origin", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("main"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		origin := t.TempDir()
		prj.Exe("git", "init", "--bare", origin)
		prj.GitSetRemote(origin)
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		tags := prj.ExeStdout("git", "-C", origin, "tag")
		branch := prj.ExeStdout("git", "-C", origin, "rev-parse", "main")
		assert.Equal(t, "v0.0.1\n", tags)
		assert.Equal(t, head, branch)
	})

	t.Run("approves a release from another branch", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		sin := bytes.NewBufferString("y\n\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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

	t.Run("approval is case insensitive and takes yes", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		sin := bytes.NewBufferString("YES\n\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Release from branch \"feature/x\"? [y/N]: ",
			tst.Stdout())
		assert.Equal(t, "v0.0.1", oskit.ReadFileStr(t, prj.Root(), "VER"))
	})

	t.Run("nothing to do on another branch does not ask", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		tst := ringtest.New(t).WetStdout()

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.0.1")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		want := "" +
			"Current tag: v0.0.1\n" +
			"HEAD on tag. Nothing to do.\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("error - declines a release from another branch", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		sin := bytes.NewBufferString("n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotDefBranch, err)
		want := "bump aborted: branch \"feature/x\" is not a default " +
			"branch (master, main)"
		assert.ErrorEqual(t, want, err)

		want = "" +
			"Current tag: \n" +
			"Release from branch \"feature/x\"? [y/N]: "
		assert.Equal(t, want, tst.Stdout())
		assert.NoFileExist(t, filepath.Join(prj.Root(), "VER"))
		assert.NoFileExist(t, filepath.Join(prj.Root(), "CHANGELOG.md"))
	})

	t.Run("error - an empty answer declines", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		ctx := t.Context()
		sin := bytes.NewBufferString("")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t, prjkit.WithGitBranch("feature/x"))
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)
		assert.ErrorIsNot(t, ErrNotDefBranch, err)
		assert.ErrorEqual(t, "read approval input: EOF", err)

		want := "" +
			"Current tag: \n" +
			"Release from branch \"feature/x\"? [y/N]: "
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("invalid semver tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "not-sem-ver")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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

	t.Run("skip to the valid semver tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.5.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Exe("git", "tag", "not-sem-ver")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		assert.Equal(t,
			"Bump version to v0.5.1.",
			prj.GitCommitLog().Latest().Summary,
		)
		assert.Equal(t,
			"Tag version v0.5.1.\n\n",
			prj.ExeStdout("git", "tag", "-l", "--format=%(contents)", "v0.5.1"),
		)

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.5.1", rel.Version.Original())
		assert.Within(t, prj.GitCommitLog().Latest().Date, "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 2.", rel.Changes[0])
	})

	t.Run("only initial commit, no tags", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Exe("git", "tag", "v0.0.1")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Exe("git", "tag", "v0.0.1")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.GitCommit("", "test commit 3")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

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

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		rng := tst.Ring("-p")

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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
		assert.Equal(t, "- commit 1.", rel.Changes[0])
	})

	t.Run("force custom tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("v0.10.0\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("v0.0.1")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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

	t.Run("custom tag may have no v prefix", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("0.10.0\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("v0.0.1")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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

	t.Run("error - EOF reading version", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)

		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: "
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("error - invalid version input", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("not-a-version\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, semver.ErrInvalidSemVer, err)

		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: "
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("error - EOF reading continue", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, io.EOF, err)

		want := "" +
			"Current tag: \n" +
			"Enter a version number [v0.0.1]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("changelog is written in reverse", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		sin := bytes.NewBufferString("\n\n")
		tst := ringtest.New(t).WetStdout().SetStdin(sin)

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.CreateFileWith(
			"## v0.0.0 (Fri, 07 Apr 2023 18:37:35 UTC)\n- test commit 1.\n\n",
			"CHANGELOG.md",
		)
		prj.GitInitAddAll("v0.0.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "test commit 2")
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		err := BumpTarget(ctx, rng, prj.Root())

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

func Test_nextRelease(t *testing.T) {
	t.Run("the core of a development version", func(t *testing.T) {
		// --- Given ---
		ver := gitaid.Version{Rev: "v0.4.1-dev.3.dirty+g7f93fb4"}

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.4.1", have.Original())
	})

	t.Run("a release is already a core", func(t *testing.T) {
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
}

func Test_BumpTarget_proposal(t *testing.T) {
	t.Run("a fix commit proposes a patch", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.9.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "fix: a defect")
		prj.Close()

		ver := must.Value(gmgo.ProjectVersion(ctx, rng, prj.Root(), ""))

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.9.1", have.Original())
	})

	t.Run("a feat commit proposes a minor", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.9.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		ver := must.Value(gmgo.ProjectVersion(ctx, rng, prj.Root(), ""))

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.10.0", have.Original())
	})

	t.Run("the patch flag forces a patch", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.9.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.Close()

		bump := gitaid.BumpPatch
		ver := must.Value(gmgo.ProjectVersion(ctx, rng, prj.Root(), bump))

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.9.1", have.Original())
	})

	t.Run("a tag that is not a version is passed over", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.9.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("not-sem-ver", "fix: a defect")
		prj.Close()

		ver := must.Value(gmgo.ProjectVersion(ctx, rng, prj.Root(), ""))
		assert.Equal(t, "v0.9.0", ver.Tag)

		// --- When ---
		have, err := nextRelease(ver)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.9.1", have.Original())
	})
}
