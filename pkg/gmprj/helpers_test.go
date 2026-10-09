// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/gmtask/internal/gmtest"
)

func Test_ProjectName_tabular(t *testing.T) {
	tt := []struct {
		testN string

		origin string
		want   string
	}{
		{"path", "/dir/dir-proj", "dir-proj"},
		{"dir", "dir-proj", "dir-proj"},
		{"repo", "ssh://git@example.com:comp/acme-proj.git", "acme-proj"},
		{"repo no .git", "ssh://git@example.com:comp/acme-proj", "acme-proj"},
		{"repo no ssh", "git@bitbucket.org:comp/acme-proj.git", "acme-proj"},
		{"go import spec", "example.com/comp/acme-proj", "acme-proj"},
		{"empty", "", ""},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, ProjectName(tc.origin))
		})
	}
}

func Test_GoModuleName_tabular(t *testing.T) {
	tt := []struct {
		testN string

		origin string
		want   string
	}{
		{"path", "/dir/dir-proj", "dir-proj"},
		{"dir", "dir-proj", "dir-proj"},
		{"repo", "ssh://git@example.com:comp/acme-proj.git", "example.com/comp/acme-proj"},
		{"repo no .git", "ssh://git@example.com:comp/acme-proj", "example.com/comp/acme-proj"},
		{"repo no ssh", "git@bitbucket.org:comp/acme-proj.git", "bitbucket.org/comp/acme-proj"},
		{"go import spec", "example.com/comp/acme-proj", "example.com/comp/acme-proj"},
		{"empty", "", ""},
		{"multiple", "acme-dki-proj", "acme-dki-proj"},
		{
			"https",
			"https://github.com/prj/repo.git",
			"github.com/prj/repo",
		},
		{
			"https with user",
			"https://user@github.com/prj/repo",
			"github.com/prj/repo",
		},
		{
			"ssh with port",
			"ssh://git@example.com:2222/comp/acme-proj.git",
			"example.com/comp/acme-proj",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, GoModuleName(tc.origin))
		})
	}
}

func Test_ExportEnv(t *testing.T) {
	// --- Given ---
	env := []string{"A=1", "B=a b", "C=it's", "D=", "E=x=y"}

	// --- When ---
	have := ExportEnv(env)

	// --- Then ---
	want := []string{
		"export A='1'",
		"export B='a b'",
		`export C='it'\''s'`,
		"export D=''",
		"export E='x=y'",
	}
	assert.Equal(t, want, have)
}

func Test_trimMajor_tabular(t *testing.T) {
	tt := []struct {
		testN string

		module string
		want   string
	}{
		{"none", "example.com/repo", "example.com/repo"},
		{"v2", "example.com/repo/v2", "example.com/repo"},
		{"v10", "example.com/repo/v10", "example.com/repo"},
		{"v1 is not a suffix", "example.com/repo/v1", "example.com/repo/v1"},
		{"v0 is not a suffix", "example.com/repo/v0", "example.com/repo/v0"},
		{"not last", "example.com/v2/repo", "example.com/v2/repo"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := trimMajor(tc.module)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_GoPkgName_tabular(t *testing.T) {
	tt := []struct {
		testN string

		origin string
		want   string
	}{
		{"path", "/dir/dir-proj", "proj"},
		{"dir", "dir-proj", "proj"},
		{"repo", "ssh://git@example.com:comp/acme-proj.git", "proj"},
		{"repo no .git", "ssh://git@example.com:comp/acme-proj", "proj"},
		{"repo no ssh", "git@bitbucket.org:comp/acme-proj.git", "proj"},
		{"go import spec", "example.com/comp/acme-proj", "proj"},
		{"empty", "", ""},
		{"multiple", "acme-dki-proj", "proj"},
		{"dot", "example.com/comp/my.app", "myapp"},
		{"trailing dash", "acme-", "acme"},
		{"leading digit", "123x", "pkg123x"},
		{"no valid rune", "---", ""},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, GoPkgName(tc.origin))
		})
	}
}

func Test_Root(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()
		root := prj.Root()

		// --- When ---
		have, err := Root(".")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, root, have)
	})

	t.Run("could not find project root", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		have, err := Root(dir)

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
		assert.ErrorContain(t, dir, err)
		assert.Equal(t, "", have)
	})

	t.Run("path", func(t *testing.T) {
		// --- Given ---
		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()
		prj.Chdir()
		root := prj.Root()

		// --- When ---
		have, err := Root(".", "pkg", "gmprj")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "pkg", "gmprj"), have)
	})
}

func Test_dirEmpty(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		have, err := dirEmpty(dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
	})

	t.Run("directory with a file", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, []byte("content"), dir, "file0.txt")

		// --- When ---
		have, err := dirEmpty(dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, have)
	})

	t.Run("directory with only a dot entry", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.MkdirAll(t, dir, ".git")

		// --- When ---
		have, err := dirEmpty(dir)

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, have)
	})

	t.Run("error - directory does not exist", func(t *testing.T) {
		// --- Given ---
		dir := filepath.Join(t.TempDir(), "not_existing")

		// --- When ---
		have, err := dirEmpty(dir)

		// --- Then ---
		assert.ErrorContain(t, "no such file or directory", err)
		assert.False(t, have)
	})
}

func Test_toAlphabeticalEnv_tabular(t *testing.T) {
	tt := []struct {
		testN string

		env  map[string]string
		want []string
	}{
		{"nil", nil, []string{}},
		{"empty", map[string]string{}, []string{}},
		{"single", map[string]string{"KEY": "val"}, []string{"KEY=val"}},
		{
			"sorted by key",
			map[string]string{"CCC": "3", "AAA": "1", "BBB": "2"},
			[]string{"AAA=1", "BBB=2", "CCC=3"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, toAlphabeticalEnv(tc.env))
		})
	}
}
