// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmmce

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

// Markdown documents the :doc:align tests read and write.
const (
	ragged = "" +
		"# Title\n\n" +
		"| Name | Value |\n" +
		"|---|---|\n" +
		"| a | 1234 |\n"

	aligned = "" +
		"# Title\n\n" +
		"| Name | Value |\n" +
		"|------|-------|\n" +
		"| a    | 1234  |\n"
)

func Test_Doc_Align(t *testing.T) {
	t.Run("aligns the files", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		one := oskit.Create(t, ragged, dir, "one.md")
		two := oskit.Create(t, aligned, dir, "two.md")
		tst := ringtest.New(t).WetStdout()

		// --- When ---
		err := Doc{}.Align(t.Context(), tst.Ring(one, two))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "Aligned "+one+"\n", tst.Stdout())
		assert.Equal(t, aligned, oskit.ReadFileStr(t, one))
		assert.Equal(t, aligned, oskit.ReadFileStr(t, two))
	})

	t.Run("check aligned files", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, aligned, t.TempDir(), "doc.md")
		tst := ringtest.New(t)

		// --- When ---
		err := Doc{}.Align(t.Context(), tst.Ring("--check", pth))

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("help", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := Doc{}.Align(t.Context(), tst.Ring("--help"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Usage of :doc:align: [flags] file...", tst.Stderr())
	})

	t.Run("error - check unaligned files", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, ragged, t.TempDir(), "doc.md")
		tst := ringtest.New(t).WetStdout()

		// --- When ---
		err := Doc{}.Align(t.Context(), tst.Ring("--check", pth))

		// --- Then ---
		assert.ErrorIs(t, ErrUnaligned, err)
		assert.ErrorContain(t, pth, err)
		assert.Equal(t, "Unaligned "+pth+"\n", tst.Stdout())
		assert.Equal(t, ragged, oskit.ReadFileStr(t, pth))
	})

	t.Run("error - no file", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)

		// --- When ---
		err := Doc{}.Align(t.Context(), tst.Ring())

		// --- Then ---
		assert.ErrorEqual(t, "no Markdown file given", err)
	})

	t.Run("error - missing file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "doc.md")
		tst := ringtest.New(t)

		// --- When ---
		err := Doc{}.Align(t.Context(), tst.Ring(pth))

		// --- Then ---
		assert.ErrorContain(t, "read markdown file: ", err)
	})

	t.Run("error - unknown flag", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()

		// --- When ---
		err := Doc{}.Align(t.Context(), tst.Ring("--unknown"))

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined", err)
		assert.Contain(t, "Usage of :doc:align:", tst.Stderr())
	})
}

func Test_alignTables_tabular(t *testing.T) {
	tt := []struct {
		testN string

		doc  string
		want string
	}{
		{"no table", "text\n| not a table |\n", "text\n| not a table |\n"},
		{"aligned", aligned, aligned},
		{"ragged", ragged, aligned},
		{
			"alignment colons",
			"| a | b | c |\n|:-|-:|:-:|\n| left | right | center |\n",
			"" +
				"| a    | b     | c      |\n" +
				"|:-----|------:|:------:|\n" +
				"| left | right | center |\n",
		},
		{
			"short row padded",
			"| a | b |\n|--|--|\n| x |\n",
			"| a | b |\n|---|---|\n| x |   |\n",
		},
		{
			"indented",
			"  | a | bb |\n  |-|-|\n",
			"  | a | bb |\n  |---|----|\n",
		},
		{
			"escaped pipe",
			"| a | b |\n|-|-|\n| x \\| y | z |\n",
			"" +
				"| a      | b |\n" +
				"|--------|---|\n" +
				"| x \\| y | z |\n",
		},
		{
			"characters not bytes",
			"| é | b |\n|-|-|\n| ab | c |\n",
			"| é  | b |\n|----|---|\n| ab | c |\n",
		},
		{
			"empty column",
			"| a |  |\n|-|-|\n",
			"| a |   |\n|---|---|\n",
		},
		{
			"carriage returns",
			"| a | bb |\r\n|-|-|\r\n",
			"| a | bb |\r\n|---|----|\r\n",
		},
		{
			"code block left alone",
			"```\n| a | b |\n|-|-|\n```\n",
			"```\n| a | b |\n|-|-|\n```\n",
		},
		{
			"table after a code block",
			"~~~\nx\n~~~\n| a | b |\n|-|-|\n",
			"~~~\nx\n~~~\n| a | b |\n|---|---|\n",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := alignTables(tc.doc)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_isSeparator_tabular(t *testing.T) {
	tt := []struct {
		testN string

		line string
		want bool
	}{
		{"dashes", "|---|---|", true},
		{"colons", "| :-- | --: | :-: |", true},
		{"no dash", "| : |", false},
		{"text", "| a | - |", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isSeparator(tc.line)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_splitRow_tabular(t *testing.T) {
	tt := []struct {
		testN string

		line string
		want []string
	}{
		{"cells", "| a | b |", []string{" a ", " b "}},
		{"escaped pipe", `| a \| b | c |`, []string{` a \| b `, " c "}},
		{"indented", "  |a|b|  ", []string{"a", "b"}},
		{"empty cell", "||x|", []string{"", "x"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := splitRow(tc.line)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_separatorCell_tabular(t *testing.T) {
	tt := []struct {
		testN string

		cell  string
		width int
		want  string
	}{
		{"plain", "---", 5, "-----"},
		{"left", ":--", 5, ":----"},
		{"right", "--:", 5, "----:"},
		{"center", ":-:", 5, ":---:"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := separatorCell(tc.cell, tc.width)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
