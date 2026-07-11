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

	"github.com/ctx42/gmtask/internal/gmtest"
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
			"Enter a version number [v0.0.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)
		assert.Equal(t, "v0.0.0", cl.Releases[0].Version.Original())
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
		assert.ErrorIs(t, gitaid.ErrNotClean, err)
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
		assert.ErrorIs(t, gitaid.ErrNotClean, err)
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
			"Enter a version number [v0.0.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
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
			"Enter a version number [v0.6.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.6.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.6.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 2.", rel.Changes[0])
	})

	t.Run("no previous commits", func(t *testing.T) {
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
			"Enter a version number [v0.0.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
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
			"Enter a version number [v0.0.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.0.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.0.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
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
			"Enter a version number [v0.1.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.1.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.1.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
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
			"Enter a version number [v0.1.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n" +
			"\n" +
			"Use\n" +
			"\tgo get example.com/comp/project@v0.1.0\n" +
			"to update upstreams.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.1.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.1.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
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
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.1.1", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.1.1", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
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
			"Enter a version number [v0.1.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		assert.Equal(t, want, tst.Stdout())
		assert.Equal(t, "v0.10.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.10.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
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
		assert.Contain(t, "Current tag: v0.0.1\n", tst.Stdout())
		assert.Equal(t, "v0.10.0", oskit.ReadFileStr(t, prj.Root(), "VER"))

		cl := must.Value(gmclog.ReadReleases(prj.Root(), "CHANGELOG.md"))
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.10.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
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
			"Enter a version number [v0.0.0]: "
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
			"Enter a version number [v0.0.0]: "
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
			"Enter a version number [v0.0.0]: " +
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
		assert.Equal(t, "v0.1.0", rel.Version.Original())
		assert.Equal(t, prj.GitCommitLog().Latest().Date, rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 2.", rel.Changes[0])

		rel = cl.Releases[1]
		assert.Equal(t, "v0.0.0", rel.Version.Original())
		assert.Within(t, "2023-04-07T18:37:35Z", "1s", rel.Date)
		assert.Len(t, 1, rel.Changes)
		assert.Equal(t, "- test commit 1.", rel.Changes[0])

		want := "" +
			"Current tag: v0.0.0\n" +
			"Enter a version number [v0.1.0]: " +
			"Now you may edit CHANGELOG.md. Then press ENTER to continue.\n" +
			"Continuing.\n" +
			"Done.\n"
		have := tst.Stdout()
		assert.Equal(t, want, have)
	})
}

func Test_getSemVer(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		prj := gmtest.NewProject(t)
		prj.Close()

		// --- When ---
		curr, next, skipped, err := getSemVer(ctx, prj.Root(), "", false)

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
		assert.Empty(t, curr)
		assert.Nil(t, next)
		assert.Nil(t, skipped)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		prj := gmtest.NewProject(t)
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		curr, next, skipped, err := getSemVer(ctx, prj.Root(), "", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, curr)
		assert.Equal(t, StartSemVer, next.Original())
		assert.Empty(t, skipped)
	})

	t.Run("one commit no tags", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		curr, next, skipped, err := getSemVer(ctx, prj.Root(), "", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, curr)
		assert.Equal(t, StartSemVer, next.Original())
		assert.Empty(t, skipped)
	})

	t.Run("one commit with invalid tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("not-sem-ver")
		prj.Close()

		// --- When ---
		curr, next, skipped, err := getSemVer(ctx, prj.Root(), "", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, curr)
		assert.Equal(t, StartSemVer, next.Original())
		assert.Equal(t, []string{"not-sem-ver"}, skipped)
	})

	t.Run("one commit after invalid tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("not-sem-ver")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		curr, next, skipped, err := getSemVer(ctx, prj.Root(), "", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, curr)
		assert.Equal(t, StartSemVer, next.Original())
		assert.Equal(t, []string{"not-sem-ver"}, skipped)
	})

	t.Run("HEAD tagged", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 0", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("not-sem-ver")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("v0.1.0")
		prj.Close()

		// --- When ---
		curr, next, skipped, err := getSemVer(ctx, prj.Root(), "", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", curr.Original())
		assert.Equal(t, "v0.2.0", next.Original())
		assert.Empty(t, skipped)
	})

	t.Run("one commit after tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("not-sem-ver")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.GitCommit("v0.1.0")
		prj.CreateFileWith("file0 4", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		curr, next, skipped, err := getSemVer(ctx, prj.Root(), "", false)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", curr.Original())
		assert.Equal(t, "v0.2.0", next.Original())
		assert.Empty(t, skipped)
	})

	t.Run("bump patch version", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		prj := gmtest.NewProject(t)
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()

		// --- When ---
		curr, next, skipped, err := getSemVer(ctx, prj.Root(), "", true)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", curr.Original())
		assert.Equal(t, "v0.1.1", next.Original())
		assert.Empty(t, skipped)
	})
}
