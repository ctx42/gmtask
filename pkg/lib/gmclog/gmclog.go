// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmclog reads, edits, and writes Markdown changelog files.
//
// A changelog is a sequence of releases, each starting with a second-level
// Markdown header naming its semantic version and release date, followed by
// the lines describing its changes:
//
//	## v0.1.6 (Sun, 02 Jan 2000 03:04:06 UTC)
//	- Change 1.
//	- Change 2.
//
// Use [ReadChangelog] to prepend releases without parsing the existing file,
// or [ReadReleases] to parse the releases for inspection or editing.
// [Changelog.AddRelease] sorts the structured [Changelog.Releases] slice
// youngest to oldest by [Release.Compare]; unparsed body bytes left by
// [ReadChangelog] are written as-is and are not reordered.
//
// Import path:
//
//	import "github.com/ctx42/gmtask/pkg/lib/gmclog"
package gmclog

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Package-level sentinel errors.
var (
	// ErrInvRelHeader is returned for a malformed release header.
	ErrInvRelHeader = errors.New("invalid release header")

	// ErrInvRelDate is returned for a malformed release date.
	ErrInvRelDate = errors.New("invalid release date")

	// ErrInvRelVersion is returned for a malformed release version.
	ErrInvRelVersion = errors.New("invalid release version")
)

// CreateFile creates an empty file if it doesn't exist.
func CreateFile(pth string) error {
	_, err := os.Stat(pth)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat file: %w", err)
	}
	fil, err := os.Create(pth) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if err = fil.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	return nil
}

// Changelog represents the changelog file.
type Changelog struct {
	pth      string     // Absolute path to the changelog file.
	Releases []*Release // Releases to write to the changelog.
	preamble []byte     // Lines preceding the first release header.
	contents []byte     // The changelog file as it is now.
}

// ReadChangelog reads the changelog file pointed by pth. In contrast to
// [ReadReleases], the contents of the changelog are not examined in any way,
// and the added releases are simply added to the top of the file.
//
// It will not return an error if the read changelog is in an unsupported
// format.
func ReadChangelog(pth string) (*Changelog, error) {
	var err error
	if pth, err = filepath.Abs(pth); err != nil {
		return nil, fmt.Errorf("resolve changelog path: %w", err)
	}
	cl := &Changelog{pth: pth}
	if cl.contents, err = os.ReadFile(pth); err != nil { //nolint:gosec
		return nil, fmt.Errorf("read changelog: %w", err)
	}
	return cl, nil
}

// ReadReleases reads the changelog file pointed by pth (joined with elems when
// provided) and parses releases. It is used when you wish to examine releases
// or edit them. It will return an error if the format of the changelog is
// incompatible.
func ReadReleases(pth string, elems ...string) (*Changelog, error) {
	pth = filepath.Join(append([]string{pth}, elems...)...)
	cl, err := ReadChangelog(pth)
	if err != nil {
		return nil, err
	}

	curr := -1
	var preamble []string
	scn := bufio.NewScanner(bytes.NewReader(cl.contents))
	for scn.Scan() {
		lin := scn.Text()
		if releaseHeaderRx.MatchString(lin) {
			var rel *Release
			if rel, err = ReleaseFromHeader(lin, WithNoFormatting); err != nil {
				return nil, fmt.Errorf("parse release header: %w", err)
			}
			cl.Releases = append(cl.Releases, rel)
			curr = len(cl.Releases) - 1
			continue
		}
		if curr >= 0 {
			cl.Releases[curr].Changes = append(cl.Releases[curr].Changes, lin)
			continue
		}
		// A line before the first release header, such as a "# Changelog"
		// title. It belongs to no release; keep it so Save re-emits it ahead
		// of the releases instead of dropping it.
		preamble = append(preamble, lin)
	}
	if err = scn.Err(); err != nil {
		return nil, fmt.Errorf("scan changelog: %w", err)
	}
	for _, rel := range cl.Releases {
		rel.finalize()
	}
	// The parsed releases and preamble fully represent the file; drop the raw
	// contents so Save reserializes from them instead of appending the file to
	// itself.
	if len(preamble) > 0 {
		cl.preamble = []byte(strings.Join(preamble, "\n") + "\n")
	}
	cl.contents = nil
	return cl, nil
}

// AddRelease adds the release(s) to the changelog. Before exiting, it sorts
// releases from youngest to oldest according to semantic version rules.
func (clg *Changelog) AddRelease(rel ...*Release) {
	clg.Releases = append(clg.Releases, rel...)
	sort.Stable(sort.Reverse(ReleaseSlice(clg.Releases)))
}

// Save saves the changelog, overwriting the original file with the releases.
// It writes to a temporary file in the same directory and renames it into
// place so a crash or partial write cannot truncate an existing changelog.
func (clg *Changelog) Save() error {
	buf := &bytes.Buffer{}
	buf.Write(clg.preamble)
	for _, rel := range clg.Releases {
		buf.WriteString(rel.String())
	}
	buf.Write(clg.contents)

	dir := filepath.Dir(clg.pth)
	tmp, err := os.CreateTemp(dir, ".changelog-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp changelog file: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write changelog: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp changelog file: %w", err)
	}
	if err = os.Rename(tmpName, clg.pth); err != nil {
		return fmt.Errorf("replace changelog file: %w", err)
	}
	ok = true
	return nil
}
