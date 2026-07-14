// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmclog

import (
	"sort"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testing/pkg/testcases"
)

func Test_WithNoFormatting(t *testing.T) {
	// --- Given ---
	rel := &Release{formatChanges: true}

	// --- When ---
	WithNoFormatting(rel)

	// --- Then ---
	assert.False(t, rel.formatChanges)
}

func Test_NewRelease(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		// --- Given ---
		now := time.Now().UTC()

		// --- When ---
		rel, err := NewRelease("v0.1.2", now)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.2", rel.Version.Original())
		assert.Exact(t, now, rel.Date)
		assert.Len(t, 0, rel.Changes)
		assert.True(t, rel.formatChanges)
	})

	t.Run("option passed to NewSemVerRelease constructor", func(t *testing.T) {
		// --- Given ---
		now := time.Now().UTC()

		// --- When ---
		rel, err := NewRelease("v0.1.2", now, WithNoFormatting)

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, rel.formatChanges)
	})

	t.Run("error - invalid semantic version", func(t *testing.T) {
		// --- Given ---
		now := time.Now().In(testcases.WAW)

		// --- When ---
		rel, err := NewRelease("invalid", now)

		// --- Then ---
		assert.ErrorIs(t, ErrInvRelVersion, err)
		assert.Nil(t, rel)
	})
}

func Test_NewSemVerRelease(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		// --- Given ---
		ver := semver.MustParse("v0.1.2")
		now := time.Now().UTC()

		// --- When ---
		rel := NewSemVerRelease(ver, now)

		// --- Then ---
		assert.Equal(t, "v0.1.2", rel.Version.Original())
		assert.Exact(t, now, rel.Date)
		assert.Len(t, 0, rel.Changes)
		assert.True(t, rel.formatChanges)
	})

	t.Run("option applied", func(t *testing.T) {
		// --- Given ---
		ver := semver.MustParse("v0.1.2")
		now := time.Now().UTC()

		// --- When ---
		rel := NewSemVerRelease(ver, now, WithNoFormatting)

		// --- Then ---
		assert.Len(t, 0, rel.Changes)
		assert.False(t, rel.formatChanges)
	})

	t.Run("date switched to UTC", func(t *testing.T) {
		// --- Given ---
		ver := semver.MustParse("v0.1.2")
		now := time.Now().In(testcases.WAW)

		// --- When ---
		rel := NewSemVerRelease(ver, now)

		// --- Then ---
		assert.Time(t, now, rel.Date)
		assert.Zone(t, time.UTC, rel.Date.Location())
	})
}

func Test_ReleaseFromHeader(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		lin := "## v0.1.6 (Sun, 02 Jan 2000 03:04:06 UTC)"

		// --- When ---
		rel, err := ReleaseFromHeader(lin)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.6", rel.Version.Original())
		assert.Time(t, "2000-01-02T03:04:06Z", rel.Date)
		assert.Len(t, 0, rel.Changes)
		assert.True(t, rel.formatChanges)
	})

	t.Run("whitespace between tokens does not matter", func(t *testing.T) {
		// --- Given ---
		lin := "##   v0.1.6   (Sun, 02 Jan 2000 03:04:06 UTC)"

		// --- When ---
		rel, err := ReleaseFromHeader(lin)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.6", rel.Version.Original())
		assert.Time(t, "2000-01-02T03:04:06Z", rel.Date)
		assert.Len(t, 0, rel.Changes)
		assert.True(t, rel.formatChanges)
	})

	t.Run("options passed", func(t *testing.T) {
		// --- Given ---
		lin := "##   v0.1.6   (Sun, 02 Jan 2000 03:04:06 UTC)"

		// --- When ---
		rel, err := ReleaseFromHeader(lin, WithNoFormatting)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 0, rel.Changes)
		assert.False(t, rel.formatChanges)
	})
}

func Test_ReleaseFromHeader_tabular(t *testing.T) {
	tt := []struct {
		testN string

		lin string
		ers []error
	}{
		{
			"error - no date",
			"## v0.1.6",
			[]error{ErrInvRelHeader},
		},
		{
			"error - no version",
			"## (Sun, 02 Jan 2000 03:04:06 UTC)",
			[]error{ErrInvRelHeader},
		},
		{
			"error - invalid version",
			"## a1 (Sun, 02 Jan 2000 03:04:06 UTC)",
			[]error{ErrInvRelVersion, semver.ErrInvalidSemVer},
		},
		{
			"error - invalid date",
			"## v0.1.2 (Sun, 02 Jan 2000 03:04:62 UTC)",
			[]error{ErrInvRelDate},
		},
		{
			"error - empty line",
			"",
			[]error{ErrInvRelHeader},
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			rel, err := ReleaseFromHeader(tc.lin)

			// --- Then ---
			for _, want := range tc.ers {
				assert.ErrorIs(t, want, err)
			}
			assert.Nil(t, rel)
		})
	}
}

func Test_Release_AddChange(t *testing.T) {
	t.Run("add nothing", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))

		// --- When ---
		rel.AddChange()

		// --- Then ---
		assert.Len(t, 0, rel.Changes)
	})

	t.Run("add to empty", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))

		// --- When ---
		rel.AddChange("a", "b")

		// --- Then ---
		assert.Equal(t, []string{"a", "b"}, rel.Changes)
	})

	t.Run("add existing", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))
		rel.Changes = []string{"a", "b"}

		// --- When ---
		rel.AddChange("c", "d")

		// --- Then ---
		assert.Equal(t, []string{"a", "b", "c", "d"}, rel.Changes)
	})
}

func Test_Release_Compare(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		// --- Given ---
		rel0 := must.Value(NewRelease("v0.1.2", time.Now()))
		rel1 := must.Value(NewRelease("v0.1.2", time.Now()))

		// --- When ---
		have := rel0.Compare(rel1)

		// --- Then ---
		assert.Equal(t, 0, have)
	})

	t.Run("less", func(t *testing.T) {
		// --- Given ---
		rel0 := must.Value(NewRelease("v0.1.2", time.Now()))
		rel1 := must.Value(NewRelease("v0.1.3", time.Now()))

		// --- When ---
		have := rel0.Compare(rel1)

		// --- Then ---
		assert.Equal(t, -1, have)
	})

	t.Run("more", func(t *testing.T) {
		// --- Given ---
		rel0 := must.Value(NewRelease("v0.1.2", time.Now()))
		rel1 := must.Value(NewRelease("v0.1.1", time.Now()))

		// --- When ---
		have := rel0.Compare(rel1)

		// --- Then ---
		assert.Equal(t, 1, have)
	})
}

func Test_Release_String(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		// --- Given ---
		date := time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC)
		rel := must.Value(NewRelease("v0.1.2", date))

		// --- When ---
		have := rel.String()

		// --- Then ---
		want := "" +
			"## v0.1.2 (Sun, 02 Jan 2000 03:04:06 UTC)\n" +
			"\n"
		assert.Equal(t, want, have)
	})

	t.Run("timezone always in UTC", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))
		rel.Date = time.Date(2000, 1, 2, 3, 4, 6, 0, testcases.WAW)

		// --- When ---
		have := rel.String()

		// --- Then ---
		want := "" +
			"## v0.1.2 (Sun, 02 Jan 2000 02:04:06 UTC)\n" +
			"\n"
		assert.Equal(t, want, have)
	})

	t.Run("couple of changes", func(t *testing.T) {
		// --- Given ---
		date := time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC)
		rel := must.Value(NewRelease("v0.1.2", date))
		rel.Changes = []string{"Change 1", "Change 2."}

		// --- When ---
		have := rel.String()

		// --- Then ---
		want := "" +
			"## v0.1.2 (Sun, 02 Jan 2000 03:04:06 UTC)\n" +
			"- Change 1.\n" +
			"- Change 2.\n" +
			"\n"
		assert.Equal(t, want, have)
	})

	t.Run("do not format changes", func(t *testing.T) {
		// --- Given ---
		date := time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC)
		rel := must.Value(NewRelease("v0.1.2", date, WithNoFormatting))
		rel.Changes = []string{"Change 1", "Change 2"}

		// --- When ---
		have := rel.String()

		// --- Then ---
		want := "" +
			"## v0.1.2 (Sun, 02 Jan 2000 03:04:06 UTC)\n" +
			"Change 1\n" +
			"Change 2\n" +
			"\n"
		assert.Equal(t, want, have)
	})
}

func Test_Release_finalize(t *testing.T) {
	t.Run("empty changes slice", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))

		// --- When ---
		rel.finalize()

		// --- Then ---
		assert.Nil(t, rel.Changes)
	})

	t.Run("no empty changes", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))
		rel.Changes = []string{"Change 1", "Change 2"}

		// --- When ---
		rel.finalize()

		// --- Then ---
		assert.Equal(t, []string{"Change 1", "Change 2"}, rel.Changes)
	})

	t.Run("one empty change", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))
		rel.Changes = []string{"Change 1", "Change 2", ""}

		// --- When ---
		rel.finalize()

		// --- Then ---
		assert.Equal(t, []string{"Change 1", "Change 2"}, rel.Changes)
	})

	t.Run("change trimmed before check empty", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))
		rel.Changes = []string{"Change 1", "Change 2", "   "}

		// --- When ---
		rel.finalize()

		// --- Then ---
		assert.Equal(t, []string{"Change 1", "Change 2"}, rel.Changes)
	})

	t.Run("multiple empty changes", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))
		rel.Changes = []string{"Change 1", "Change 2", "", ""}

		// --- When ---
		rel.finalize()

		// --- Then ---
		assert.Equal(t, []string{"Change 1", "Change 2"}, rel.Changes)
	})

	t.Run("internal empty line is preserved", func(t *testing.T) {
		// --- Given ---
		rel := must.Value(NewRelease("v0.1.2", time.Now()))
		rel.Changes = []string{"Change 1", "", "Change 2", ""}

		// --- When ---
		rel.finalize()

		// --- Then ---
		assert.Equal(t, []string{"Change 1", "", "Change 2"}, rel.Changes)
	})
}

func Test_ReleaseSlice(t *testing.T) {
	t.Run("sort sorted", func(t *testing.T) {
		// --- Given ---
		rel0 := must.Value(NewRelease("v0.1.0", time.Now()))
		rel1 := must.Value(NewRelease("v0.1.1", time.Now()))
		rel2 := must.Value(NewRelease("v0.1.2", time.Now()))

		rels := []*Release{rel0, rel1, rel2}

		// --- When ---
		sort.Sort(ReleaseSlice(rels))

		// --- Then ---
		assert.Same(t, rel0, rels[0])
		assert.Same(t, rel1, rels[1])
		assert.Same(t, rel2, rels[2])
	})

	t.Run("sort not sorted", func(t *testing.T) {
		// --- Given ---
		rel0 := must.Value(NewRelease("v0.1.0", time.Now()))
		rel1 := must.Value(NewRelease("v0.1.1", time.Now()))
		rel2 := must.Value(NewRelease("v0.1.2", time.Now()))

		rels := []*Release{rel2, rel0, rel1}

		// --- When ---
		sort.Sort(ReleaseSlice(rels))

		// --- Then ---
		assert.Same(t, rel0, rels[0])
		assert.Same(t, rel1, rels[1])
		assert.Same(t, rel2, rels[2])
	})

	t.Run("reverse sort not sorted", func(t *testing.T) {
		// --- Given ---
		rel0 := must.Value(NewRelease("v0.1.0", time.Now()))
		rel1 := must.Value(NewRelease("v0.1.1", time.Now()))
		rel2 := must.Value(NewRelease("v0.1.2", time.Now()))

		rels := []*Release{rel2, rel0, rel1}

		// --- When ---
		sort.Sort(sort.Reverse(ReleaseSlice(rels)))

		// --- Then ---
		assert.Same(t, rel2, rels[0])
		assert.Same(t, rel1, rels[1])
		assert.Same(t, rel0, rels[2])
	})
}
