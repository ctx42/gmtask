// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"

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
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, GoModuleName(tc.origin))
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
