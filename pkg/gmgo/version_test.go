// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"context"
	"testing"

	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/prjkit"
)

func Test_ProjectVersion(t *testing.T) {
	t.Run("infers the bump from the commits", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()
		rng.EnvUnset(EnvBldBump)

		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("", "feat: a feature")
		prj.Close()

		// --- When ---
		have, err := ProjectVersion(ctx, rng, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.5.0-dev.1+g"+cm.Hash, have.Rev)
	})

	t.Run("the environment overrides the scan", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()
		rng.EnvSet(EnvBldBump, gitaid.BumpMajor)

		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v1.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("", "fix: a defect")
		prj.Close()

		// --- When ---
		have, err := ProjectVersion(ctx, rng, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v2.0.0-dev.1+g"+cm.Hash, have.Rev)
	})

	t.Run("a release needs no bump at all", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()
		rng.EnvUnset(EnvBldBump)

		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.Close()

		// --- When ---
		have, err := ProjectVersion(ctx, rng, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.4.0", have.Rev)
		assert.True(t, have.Release)
	})

	t.Run("error - not a git repository", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		rng := ringtest.New(t).Ring()

		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := ProjectVersion(ctx, rng, prj.Root(), "")

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
		assert.Equal(t, gitaid.Version{}, have)
	})
}

func Test_InferBump(t *testing.T) {
	t.Run("anything unrecognized is a patch", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "chore: tidy up")
		prj.Close()

		// --- When ---
		have, err := InferBump(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gitaid.BumpPatch, have)
	})

	t.Run("a feat subject is a minor", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "feat(api): a feature")
		prj.Close()

		// --- When ---
		have, err := InferBump(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gitaid.BumpMinor, have)
	})

	t.Run("a bang subject is a major", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "refactor!: break it")
		prj.Close()

		// --- When ---
		have, err := InferBump(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gitaid.BumpMajor, have)
	})

	t.Run("a breaking footer outranks a feat", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "feat: a feature")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.GitCommit("", "fix: a defect\n\nBREAKING CHANGE: it moved")
		prj.Close()

		// --- When ---
		have, err := InferBump(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gitaid.BumpMajor, have)
	})

	t.Run("a feat in the body is not a subject", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "fix: a defect\n\nfeat: not a subject")
		prj.Close()

		// --- When ---
		have, err := InferBump(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gitaid.BumpPatch, have)
	})

	t.Run("with no version tag the whole history is scanned", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "feat: the first feature")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.GitCommit("", "fix: a defect")
		prj.Close()

		// --- When ---
		have, err := InferBump(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, gitaid.BumpMinor, have)
	})

	t.Run("error - not a git repository", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := InferBump(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
		assert.Empty(t, have)
	})
}

func Test_tagOf_tabular(t *testing.T) {
	tt := []struct {
		testN string

		desc string
		want string
	}{
		{"on the tag", "v0.4.0", "v0.4.0"},
		{"on the tag dirty", "v0.4.0-dirty", "v0.4.0"},
		{"past the tag", "v0.4.0-3-g7f93fb4", "v0.4.0"},
		{"past the tag dirty", "v0.4.0-3-g7f93fb4-dirty", "v0.4.0"},
		{"no usable tag", "v0.0.0-12-g7f93fb4", "v0.0.0"},
		{"pre-release tag", "v1.0.0-rc.1-1-g7f93fb4", "v1.0.0-rc.1"},
		{"tag holding -g", "v1.0.0-gamma", "v1.0.0-gamma"},
		{"tag holding -g past it", "v1.0.0-gamma-2-g7f93fb4", "v1.0.0-gamma"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := tagOf(tc.desc)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
