// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmclog

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// releaseHeaderRx is a regular expression matching a release header.
var releaseHeaderRx = regexp.MustCompile(`^## (.*?) \((.+)\)$`)

// WithNoFormatting is a constructor option turning off change-line formatting.
func WithNoFormatting(rel *Release) { rel.formatChanges = false }

// Release represents a single release in a changelog file.
//
// Release starts with a second-level Markdown header followed by multiple lines
// describing what changes have been implemented. Example release:
//
//	## v0.1.6 (Sun, 02 Jan 2000 03:04:06 UTC)
//	- line 1
//	- line 2
//
// The release ends with one empty line.
type Release struct {
	Version       *semver.Version // Semantic version.
	Date          time.Time       // Date release was created.
	Changes       []string        // Release changes.
	formatChanges bool            // Controls if changes are prefixed with "- ".
}

// NewRelease parses ver as a Semantic Version string and calls the
// NewSemVerRelease constructor function. When version parsing fails, it
// returns an error wrapping ErrInvRelVersion and the one returned by the
// semver package.
func NewRelease(
	ver string,
	date time.Time,
	opts ...func(*Release),
) (*Release, error) {

	sv, err := semver.NewVersion(ver)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvRelVersion, err)
	}
	return NewSemVerRelease(sv, date, opts...), nil
}

// NewSemVerRelease returns a new instance of Release with options applied. The
// date is always changed to UTC.
func NewSemVerRelease(
	ver *semver.Version,
	date time.Time,
	opts ...func(*Release),
) *Release {

	rel := &Release{
		Version:       ver,
		Date:          date.UTC(),
		formatChanges: true,
	}
	for _, opt := range opts {
		opt(rel)
	}
	return rel
}

// ReleaseFromHeader creates a new instance of Release based on the header line
// of a release in a changelog.
func ReleaseFromHeader(lin string, opts ...func(*Release)) (*Release, error) {
	parts := releaseHeaderRx.FindStringSubmatch(lin)
	if len(parts) != 3 {
		return nil, ErrInvRelHeader
	}
	parts[1] = strings.TrimSpace(parts[1])
	parts[2] = strings.TrimSpace(parts[2])
	tim, err := time.Parse(time.RFC1123, parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvRelDate, err)
	}
	return NewRelease(parts[1], tim, opts...)
}

// AddChange adds new changes to the release.
func (rel *Release) AddChange(changes ...string) {
	rel.Changes = append(rel.Changes, changes...)
}

// Compare compares two release versions. Returns -1, 0, or 1 if this version is
// smaller than, equal to, or larger than the other version.
func (rel *Release) Compare(other *Release) int {
	return rel.Version.Compare(other.Version)
}

func (rel *Release) String() string {
	tim := rel.Date.UTC().Format(time.RFC1123)
	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, "## %s (%s)\n", rel.Version.Original(), tim)
	var prefix string
	if rel.formatChanges {
		prefix = "- "
	}
	for _, lin := range rel.Changes {
		if rel.formatChanges && !strings.HasSuffix(lin, ".") {
			lin += "."
		}
		_, _ = fmt.Fprintf(&buf, "%s%s\n", prefix, lin)
	}
	buf.WriteString("\n")
	return buf.String()
}

// finalize removes empty lines from the end of the "changes" slice.
func (rel *Release) finalize() {
	for i := len(rel.Changes) - 1; i >= 0; i-- {
		if strings.TrimSpace(rel.Changes[i]) != "" {
			break
		}
		rel.Changes = rel.Changes[:i]
	}
}

// ReleaseSlice attaches the methods of [sort.Interface] to []*Release, sorting
// in increasing version order.
type ReleaseSlice []*Release

func (rls ReleaseSlice) Len() int { return len(rls) }
func (rls ReleaseSlice) Less(i, j int) bool {
	return rls[i].Compare(rls[j]) == -1
}
func (rls ReleaseSlice) Swap(i, j int) { rls[i], rls[j] = rls[j], rls[i] }
