// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmclog

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/pathkit"
)

func Test_CreateFile(t *testing.T) {
	t.Run("not existing file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "CHANGELOG.md")

		// --- When ---
		err := CreateFile(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, pth)
	})

	t.Run("does not truncate existing", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, "abc", t.TempDir(), "CHANGELOG.md")

		// --- When ---
		err := CreateFile(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, pth)
		assert.Equal(t, "abc", oskit.ReadFileStr(t, pth))
	})

	t.Run("error - parent is a file", func(t *testing.T) {
		// --- Given ---
		fil := oskit.Create(t, "x", t.TempDir(), "file")

		// --- When ---
		err := CreateFile(filepath.Join(fil, "child"))

		// --- Then ---
		assert.ErrorIs(t, syscall.ENOTDIR, err)
	})

	t.Run("error - parent does not exist", func(t *testing.T) {
		// --- When ---
		err := CreateFile(filepath.Join(t.TempDir(), "nope", "CHANGELOG.md"))

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
	})

	t.Run("error - path is a directory", func(t *testing.T) {
		// --- When ---
		err := CreateFile(t.TempDir())

		// --- Then ---
		assert.ErrorContain(t, "is a directory", err)
	})
}

func Test_ReadChangelog(t *testing.T) {
	t.Run("single release", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/changelog_1.md"
		absPath := pathkit.AbsPath(t, relPath)

		// --- When ---
		cl, err := ReadChangelog(relPath)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, absPath, cl.pth)
		assert.Nil(t, cl.Releases)

		have := string(cl.contents)
		want := oskit.ReadFileStr(t, absPath)
		assert.Equal(t, want, have)
	})

	t.Run("keeps title above releases", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"# Changelog\n" +
			"\n" +
			"## v0.1.0 (Sun, 02 Jan 2000 00:00:00 UTC)\n" +
			"- Change 1.\n" +
			"\n"
		pth := oskit.Create(t, src, t.TempDir(), "CHANGELOG.md")

		// --- When ---
		clg, err := ReadChangelog(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "# Changelog\n\n", string(clg.preamble))
		assert.Equal(t, src[len("# Changelog\n\n"):], string(clg.contents))
	})

	t.Run("title without releases", func(t *testing.T) {
		// --- Given ---
		src := "# Changelog\n\nNothing released yet.\n"
		pth := oskit.Create(t, src, t.TempDir(), "CHANGELOG.md")

		// --- When ---
		clg, err := ReadChangelog(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "# Changelog\n\n", string(clg.preamble))
		assert.Equal(t, "Nothing released yet.\n", string(clg.contents))
	})

	t.Run("error - not existing file", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/not_existing.md"

		// --- When ---
		cl, err := ReadChangelog(relPath)

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
		assert.Nil(t, cl)
	})
}

func Test_ReadReleases(t *testing.T) {
	t.Run("single release", func(t *testing.T) {
		// --- When ---
		cl, err := ReadReleases("testdata", "changelog_0.md")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, pathkit.AbsPath(t, "testdata", "changelog_0.md"), cl.pth)
		assert.Len(t, 1, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.1.5", rel.Version.Original())
		assert.Time(t, time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC), rel.Date)
		assert.Equal(t, []string{"- Change 1.", "- Change 2."}, rel.Changes)
	})

	t.Run("multiple releases", func(t *testing.T) {
		// --- When ---
		cl, err := ReadReleases("testdata", "changelog_1.md")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, pathkit.AbsPath(t, "testdata", "changelog_1.md"), cl.pth)
		assert.Len(t, 2, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.1.6", rel.Version.Original())
		assert.Time(t, time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC), rel.Date)
		assert.Equal(t, []string{"- Change 3.", "- Change 4."}, rel.Changes)

		rel = cl.Releases[1]
		assert.Equal(t, "v0.1.5", rel.Version.Original())
		assert.Time(t, time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC), rel.Date)
		assert.Equal(t, []string{"- Change 1.", "- Change 2."}, rel.Changes)
	})

	t.Run("multi line changes", func(t *testing.T) {
		// --- When ---
		cl, err := ReadReleases("testdata", "changelog_2.md")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, pathkit.AbsPath(t, "testdata", "changelog_2.md"), cl.pth)
		assert.Len(t, 2, cl.Releases)

		rel := cl.Releases[0]
		assert.Equal(t, "v0.1.6", rel.Version.Original())
		assert.Time(t, time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC), rel.Date)
		assert.Equal(t, []string{"- Change 3.", "- Change 4."}, rel.Changes)

		rel = cl.Releases[1]
		assert.Equal(t, "v0.1.5", rel.Version.Original())
		assert.Time(t, time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC), rel.Date)
		want := []string{
			"- Change 1.",
			"- Change 2. \\",
			"  Change 2.1 \\",
			"  Change 2.2",
			"- Change 3.",
		}
		assert.Equal(t, want, rel.Changes)
	})

	t.Run("hash-prefixed change line is not a release header", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"## v0.1.5 (Sun, 02 Jan 2000 03:04:05 UTC)\n" +
			"- Change 1.\n" +
			"## Not a header.\n" +
			"- Change 2.\n"
		pth := oskit.Create(t, src, t.TempDir(), "CHANGELOG.md")

		// --- When ---
		cl, err := ReadReleases(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, cl.Releases)

		want := []string{"- Change 1.", "## Not a header.", "- Change 2."}
		assert.Equal(t, want, cl.Releases[0].Changes)
	})

	t.Run("parenthesized title is not a release header", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"## v0.1.5 (Sun, 02 Jan 2000 03:04:05 UTC)\n" +
			"- Change 1.\n" +
			"## Notes (draft)\n"
		pth := oskit.Create(t, src, t.TempDir(), "CHANGELOG.md")

		// --- When ---
		cl, err := ReadReleases(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, cl.Releases)
		want := []string{"- Change 1.", "## Notes (draft)"}
		assert.Equal(t, want, cl.Releases[0].Changes)
	})

	t.Run("error - changelog not found", func(t *testing.T) {
		// --- When ---
		cl, err := ReadReleases("testdata", "not_existing.md")

		// --- Then ---
		assert.ErrorIs(t, os.ErrNotExist, err)
		assert.Nil(t, cl)
	})

	t.Run("error - invalid release header", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, "## v0.1.0 (not-a-date)\n", t.TempDir(), "CL.md")

		// --- When ---
		cl, err := ReadReleases(pth)

		// --- Then ---
		assert.ErrorIs(t, ErrInvRelDate, err)
		assert.Nil(t, cl)
	})

	t.Run("error - names the bad line", func(t *testing.T) {
		// --- Given ---
		src := "# Changelog\n\n## v0.1.0 (not-a-date)\n"
		pth := oskit.Create(t, src, t.TempDir(), "CL.md")

		// --- When ---
		cl, err := ReadReleases(pth)

		// --- Then ---
		assert.ErrorContain(t, "parse release header on line 3", err)
		assert.Nil(t, cl)
	})

	t.Run("error - scan fails", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, strings.Repeat("x", 70000), t.TempDir(), "CL.md")

		// --- When ---
		cl, err := ReadReleases(pth)

		// --- Then ---
		assert.ErrorContain(t, "scan changelog", err)
		assert.Nil(t, cl)
	})
}

func Test_Changelog_ReadReleases_Save_round_trip(t *testing.T) {
	t.Run("save does not duplicate parsed releases", func(t *testing.T) {
		// --- Given ---
		src := oskit.ReadFileStr(t, "testdata/changelog_1.md")
		pth := oskit.Create(t, src, t.TempDir(), "CHANGELOG.md")
		cl := must.Value(ReadReleases(pth))

		// --- When ---
		err := cl.Save()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, src, oskit.ReadFileStr(t, pth))
	})

	t.Run("preserves preamble before the first release", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"# Changelog\n" +
			"\n" +
			"## v0.1.5 (Sun, 02 Jan 2000 03:04:05 UTC)\n" +
			"- Change 1.\n" +
			"- Change 2.\n" +
			"\n"
		pth := oskit.Create(t, src, t.TempDir(), "CHANGELOG.md")
		cl := must.Value(ReadReleases(pth))

		// --- When ---
		err := cl.Save()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, src, oskit.ReadFileStr(t, pth))
	})
}

func Test_Changelog_AddRelease(t *testing.T) {
	t.Run("add releases and sorts", func(t *testing.T) {
		// --- Given ---
		rel0 := must.Value(NewRelease("v0.1.1", time.Now()))
		rel1 := must.Value(NewRelease("v0.1.2", time.Now()))
		rel2 := must.Value(NewRelease("v0.1.3", time.Now()))

		cl := &Changelog{
			Releases: []*Release{rel0},
		}

		// --- When ---
		cl.AddRelease(rel1, rel2)

		// --- Then ---
		assert.Len(t, 3, cl.Releases)
		assert.Same(t, rel2, cl.Releases[0])
		assert.Same(t, rel1, cl.Releases[1])
		assert.Same(t, rel0, cl.Releases[2])
	})
}

func Test_Changelog_Save(t *testing.T) {
	t.Run("save not existing file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "CHANGELOG.md")

		tim0 := must.Value(time.Parse(time.RFC3339, "2000-01-02T00:00:00Z"))
		rel0 := must.Value(NewRelease("v0.1.0", tim0))
		rel0.AddChange("Change 1", "Change 2")

		tim1 := must.Value(time.Parse(time.RFC3339, "2000-01-02T01:00:00Z"))
		rel1 := must.Value(NewRelease("v0.1.1", tim1))
		rel1.AddChange("Change 3", "Change 4")

		cl := &Changelog{pth: pth}
		cl.AddRelease(rel0)
		cl.AddRelease(rel1)

		// --- When ---
		err := cl.Save()

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"## v0.1.1 (Sun, 02 Jan 2000 01:00:00 UTC)\n" +
			"- Change 3.\n" +
			"- Change 4.\n" +
			"\n" +
			"## v0.1.0 (Sun, 02 Jan 2000 00:00:00 UTC)\n" +
			"- Change 1.\n" +
			"- Change 2.\n" +
			"\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, pth))
	})

	t.Run("save existing file", func(t *testing.T) {
		// --- Given ---
		content := "" +
			"## v0.1.0 (Sun, 02 Jan 2000 00:00:00 UTC)\n" +
			"- Change 1.\n" +
			"- Change 2.\n" +
			"\n"
		pth := oskit.Create(t, content, t.TempDir(), "CHANGELOG.md")

		tim1 := must.Value(time.Parse(time.RFC3339, "2000-01-02T01:00:00Z"))
		rel1 := must.Value(NewRelease("v0.1.1", tim1))
		rel1.AddChange("Change 3", "Change 4")

		tim2 := must.Value(time.Parse(time.RFC3339, "2000-01-02T02:00:00Z"))
		rel2 := must.Value(NewRelease("v0.1.2", tim2))
		rel2.AddChange("Change 5", "Change 6")

		cl := must.Value(ReadChangelog(pth))
		cl.AddRelease(rel1)
		cl.AddRelease(rel2)

		// --- When ---
		err := cl.Save()

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"## v0.1.2 (Sun, 02 Jan 2000 02:00:00 UTC)\n" +
			"- Change 5.\n" +
			"- Change 6.\n" +
			"\n" +
			"## v0.1.1 (Sun, 02 Jan 2000 01:00:00 UTC)\n" +
			"- Change 3.\n" +
			"- Change 4.\n" +
			"\n" +
			"## v0.1.0 (Sun, 02 Jan 2000 00:00:00 UTC)\n" +
			"- Change 1.\n" +
			"- Change 2.\n" +
			"\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, pth))
	})

	t.Run("keeps file mode", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, "", t.TempDir(), "CHANGELOG.md")
		must.Nil(os.Chmod(pth, 0o640))
		clg := must.Value(ReadChangelog(pth))

		// --- When ---
		err := clg.Save()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o640), oskit.Stat(t, pth).Mode().Perm())
	})

	t.Run("new file mode", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "CHANGELOG.md")
		clg := &Changelog{pth: pth}

		// --- When ---
		err := clg.Save()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), oskit.Stat(t, pth).Mode().Perm())
	})

	t.Run("updates symlink target", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		docs := oskit.MkdirAll(t, dir, "docs")
		dst := oskit.Create(t, "", docs, "CHANGELOG.md")
		lnk := filepath.Join(dir, "CHANGELOG.md")
		must.Nil(os.Symlink(dst, lnk))

		tim := must.Value(time.Parse(time.RFC3339, "2000-01-02T00:00:00Z"))
		rel := must.Value(NewRelease("v0.1.0", tim))
		clg := must.Value(ReadChangelog(lnk))
		clg.AddRelease(rel)

		// --- When ---
		err := clg.Save()

		// --- Then ---
		assert.NoError(t, err)
		lst := must.Value(os.Lstat(lnk))
		assert.True(t, lst.Mode()&os.ModeSymlink != 0)
		want := "## v0.1.0 (Sun, 02 Jan 2000 00:00:00 UTC)\n\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, dst))
	})

	t.Run("new release below title", func(t *testing.T) {
		// --- Given ---
		src := "" +
			"# Changelog\n" +
			"\n" +
			"## v0.1.0 (Sun, 02 Jan 2000 00:00:00 UTC)\n" +
			"- Change 1.\n" +
			"\n"
		pth := oskit.Create(t, src, t.TempDir(), "CHANGELOG.md")

		tim := must.Value(time.Parse(time.RFC3339, "2000-01-02T01:00:00Z"))
		rel := must.Value(NewRelease("v0.1.1", tim))
		rel.AddChange("Change 2")
		clg := must.Value(ReadChangelog(pth))
		clg.AddRelease(rel)

		// --- When ---
		err := clg.Save()

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"# Changelog\n" +
			"\n" +
			"## v0.1.1 (Sun, 02 Jan 2000 01:00:00 UTC)\n" +
			"- Change 2.\n" +
			"\n" +
			"## v0.1.0 (Sun, 02 Jan 2000 00:00:00 UTC)\n" +
			"- Change 1.\n" +
			"\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, pth))
	})

	t.Run("error - create fails", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "nope", "CHANGELOG.md")
		cl := &Changelog{pth: pth}

		// --- When ---
		err := cl.Save()

		// --- Then ---
		assert.ErrorContain(t, "create temp changelog file", err)
	})

	t.Run("error - rename fails", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := oskit.MkdirAll(t, dir, "CHANGELOG.md")
		oskit.Create(t, "x", pth, "file")
		clg := &Changelog{pth: pth}

		// --- When ---
		err := clg.Save()

		// --- Then ---
		assert.ErrorContain(t, "replace changelog file", err)
		assert.Equal(t, []string{"CHANGELOG.md"}, oskit.Readdirnames(t, dir))
	})
}
