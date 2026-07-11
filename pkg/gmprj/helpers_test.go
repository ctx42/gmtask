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
		{"repo", "ssh://git@example.com:vr/skw-proj.git", "skw-proj"},
		{"repo no .git", "ssh://git@example.com:vr/skw-proj", "skw-proj"},
		{"repo no ssh", "git@bitbucket.org:vrinf/skw-vdef.git", "skw-vdef"},
		{"go import spec", "example.com/vr/skw-proj", "skw-proj"},
		{"empty", "", ""},
	}

	for _, tc := range tt {
		tc := tc
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
		{"repo", "ssh://git@example.com:vr/skw-proj.git", "example.com/vr/skw-proj"},
		{"repo no .git", "ssh://git@example.com:vr/skw-proj", "example.com/vr/skw-proj"},
		{"repo no ssh", "git@bitbucket.org:vrinf/skw-vdef.git", "bitbucket.org/vrinf/skw-vdef"},
		{"go import spec", "example.com/vr/skw-proj", "example.com/vr/skw-proj"},
		{"empty", "", ""},
		{"multiple", "skw-dki-proj", "skw-dki-proj"},
	}

	for _, tc := range tt {
		tc := tc
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
		{"repo", "ssh://git@example.com:vr/skw-proj.git", "proj"},
		{"repo no .git", "ssh://git@example.com:vr/skw-proj", "proj"},
		{"repo no ssh", "git@bitbucket.org:vrinf/skw-vdef.git", "vdef"},
		{"go import spec", "example.com/vr/skw-proj", "proj"},
		{"empty", "", ""},
		{"multiple", "skw-dki-proj", "proj"},
	}

	for _, tc := range tt {
		tc := tc
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
