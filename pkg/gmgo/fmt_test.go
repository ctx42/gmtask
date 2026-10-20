// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/gmtask/internal/gmtest"
)

// longLine is a Go line 85 columns wide.
var longLine = "var Long = \"" + strings.Repeat("x", 72) + "\""

func Test_Go_Fmt(t *testing.T) {
	t.Run("clean changes", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.GitInitAddAll()
		prj.CreateFileWith("package project\n\nvar X = 1\n", "x.go")
		prj.Close()
		prj.Chdir()
		tst := ringtest.New(t)

		// --- When ---
		err := Go{}.Fmt(t.Context(), tst.Ring())

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("unformatted file and long line", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		prj.CreateFileWith("package project\n\n"+longLine+"\n", "old.go")
		prj.GitInitAddAll()
		prj.CreateFileWith("package project\n\nvar  X = 1\n", "x.go")
		long := strings.Replace(longLine, "Long", "Long2", 1)
		src := "package project\n\nvar Y = 1\n\n" + long + "\n"
		prj.CreateFileWith(src, "sub", "y.go")
		prj.Close()
		prj.Chdir()
		tst := ringtest.New(t).WetStdout()

		// --- When ---
		err := Go{}.Fmt(t.Context(), tst.Ring())

		// --- Then ---
		assert.ErrorIs(t, ErrFmt, err)
		assert.ErrorContain(t, "2 issues", err)
		want := "" +
			"sub/y.go:5: 86 columns, limit 80\n" +
			"x.go: not formatted\n"
		assert.Equal(t, want, tst.Stdout())
	})

	t.Run("editorconfig limit and older revision", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.GoModInit()
		prj.ProjectFrom("testdata/vet/success/project")
		cfg := "root = true\n\n[*.go]\nmax_line_length = 100\n"
		prj.CreateFileWith(cfg, ".editorconfig")
		prj.GitInitAddAll("v1.0.0")
		prj.CreateFileWith("package project\n\n"+longLine+"\n", "long.go")
		prj.GitCommit("")
		prj.Close()
		prj.Chdir()
		tst := ringtest.New(t)

		// --- When ---
		err := Go{}.Fmt(t.Context(), tst.Ring("--rev", "v1.0.0"))

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := Go{}.Fmt(t.Context(), tst.Ring("--help"))

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :go:fmt\n" +
			"  -h, --help    show help\n" +
			"  -r, --rev     revision to compare the changes with\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - not a repository", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.Close()
		prj.Chdir()
		tst := ringtest.New(t)

		// --- When ---
		err := Go{}.Fmt(t.Context(), tst.Ring())

		// --- Then ---
		assert.ErrorContain(t, "changed lines: not a git repository", err)
	})

	t.Run("error - unknown flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := Go{}.Fmt(t.Context(), tst.Ring("--unknown"))

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined", err)
		assert.Contain(t, "Usage of :go:fmt", tst.Stderr())
	})
}

func Test_fmtIssues(t *testing.T) {
	t.Run("formatted and short", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, "package x\n", t.TempDir(), "x.go")

		// --- When ---
		have, err := fmtIssues(pth, []int{1}, 80, 4)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, have)
	})

	t.Run("syntax error and long tab line", func(t *testing.T) {
		// --- Given ---
		src := "package x\n\nfunc f() {\n\t\treturn\n"
		pth := oskit.Create(t, src, t.TempDir(), "x.go")

		// --- When ---
		have, err := fmtIssues(pth, []int{4, 9}, 9, 4)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			pth + ": not formatted",
			pth + ":4: 14 columns, limit 9",
		}
		assert.Equal(t, want, have)
	})

	t.Run("no limit", func(t *testing.T) {
		// --- Given ---
		src := "package x\n\n" + longLine + "\n"
		pth := oskit.Create(t, src, t.TempDir(), "x.go")

		// --- When ---
		have, err := fmtIssues(pth, []int{3}, 0, 4)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, have)
	})

	t.Run("error - missing file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "x.go")

		// --- When ---
		have, err := fmtIssues(pth, []int{1}, 80, 4)

		// --- Then ---
		assert.ErrorContain(t, "read "+pth+": ", err)
		assert.Nil(t, have)
	})
}

func Test_lineWidth_tabular(t *testing.T) {
	tt := []struct {
		testN string

		line string
		tab  int
		want int
	}{
		{"plain", "abc", 4, 3},
		{"leading tabs", "\t\tx", 4, 9},
		{"tab after text", "ab\tc", 4, 5},
		{"tab width eight", "\tx", 8, 9},
		{"multibyte", "é\r", 4, 1},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := lineWidth(tc.line, tc.tab)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_goLineLimits(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		// --- When ---
		limit, tab, err := goLineLimits(t.TempDir())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 80, limit)
		assert.Equal(t, 4, tab)
	})

	t.Run("closer file wins up to the root", func(t *testing.T) {
		// --- Given ---
		top := t.TempDir()
		cfg := "[*]\nmax_line_length = 60\ntab_width = 3\n"
		oskit.Create(t, cfg, top, ".editorconfig")
		mid := oskit.MkdirAll(t, top, "mid")
		cfg = "root = true\n\n[*]\nindent_size = 2\n"
		oskit.Create(t, cfg, mid, ".editorconfig")
		dir := oskit.MkdirAll(t, mid, "pkg")
		cfg = "[*.go]\nmax_line_length = 100\n"
		oskit.Create(t, cfg, dir, ".editorconfig")

		// --- When ---
		limit, tab, err := goLineLimits(dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 100, limit)
		assert.Equal(t, 2, tab)
	})

	t.Run("limit off and tab width over indent size", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		cfg := "" +
			"root = true\n" +
			"[*.go]\n" +
			"max_line_length = off\n" +
			"indent_size = 2\n" +
			"tab_width = 8\n"
		oskit.Create(t, cfg, dir, ".editorconfig")

		// --- When ---
		limit, tab, err := goLineLimits(dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, limit)
		assert.Equal(t, 8, tab)
	})

	t.Run("error - unreadable editorconfig", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, ".editorconfig")

		// --- When ---
		_, _, err := goLineLimits(dir)

		// --- Then ---
		assert.ErrorContain(t, "editorconfig: ", err)
	})
}

func Test_editorConfigGo(t *testing.T) {
	t.Run("sections for Go files", func(t *testing.T) {
		// --- Given ---
		cfg := "" +
			"; comment\n" +
			"ROOT = True\n\n" +
			"[*]\n" +
			"indent_style = space\n" +
			"max_line_length = 120\n\n" +
			"# comment\n" +
			"[*.go]\n" +
			"indent_style = tab\n\n" +
			"[*.md]\n" +
			"max_line_length = 80\n" +
			"no value\n"
		pth := oskit.Create(t, cfg, t.TempDir(), ".editorconfig")

		// --- When ---
		props, root, err := editorConfigGo(pth)

		// --- Then ---
		assert.NoError(t, err)
		want := map[string]string{
			"indent_style":    "tab",
			"max_line_length": "120",
		}
		assert.Equal(t, want, props)
		assert.True(t, root)
	})

	t.Run("missing file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), ".editorconfig")

		// --- When ---
		props, root, err := editorConfigGo(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, props)
		assert.False(t, root)
	})
}

func Test_editorProp_tabular(t *testing.T) {
	tt := []struct {
		testN string

		line string
		key  string
		val  string
		ok   bool
	}{
		{"property", "Max_Line_Length = OFF", "max_line_length", "off", true},
		{"hash comment", "# a = b", "", "", false},
		{"semicolon comment", "; a = b", "", "", false},
		{"blank", "", "", "", false},
		{"no equals", "word", "word", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			key, val, ok := editorProp(tc.line)

			// --- Then ---
			assert.Equal(t, tc.key, key)
			assert.Equal(t, tc.val, val)
			assert.Equal(t, tc.ok, ok)
		})
	}
}

func Test_matchesGo_tabular(t *testing.T) {
	tt := []struct {
		testN string

		glob string
		want bool
	}{
		{"star", "*", true},
		{"go files", "*.go", true},
		{"brace list", "{*.go,*.mod}", true},
		{"brace extension", "*.{go,mod}", true},
		{"any depth", "**/*.go", true},
		{"other files", "*.md", false},
		{"subdirectory", "pkg/*.go", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := matchesGo(tc.glob)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_expandBraces_tabular(t *testing.T) {
	tt := []struct {
		testN string

		glob string
		want []string
	}{
		{"no braces", "*.go", []string{"*.go"}},
		{"one group", "*.{go,mod}", []string{"*.go", "*.mod"}},
		{"two groups", "{a,b}.{x,y}", []string{"a.x", "a.y", "b.x", "b.y"}},
		{"unclosed", "*.{go", []string{"*.{go"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := expandBraces(tc.glob)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
