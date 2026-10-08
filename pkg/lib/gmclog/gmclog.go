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
// Use [ReadChangelog] to add releases above the existing ones without parsing
// them, or [ReadReleases] to parse the releases for inspection or editing.
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

// CreateFile creates an empty file at pth when none exists; an existing file
// is left untouched. Creating and checking happen in one open call, so a file
// created concurrently by another process is never truncated. It returns an
// error when pth is a directory.
func CreateFile(pth string) error {
	fil, err := os.OpenFile(pth, os.O_RDONLY|os.O_CREATE, 0o666) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	inf, err := fil.Stat()
	_ = fil.Close()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if inf.IsDir() {
		return fmt.Errorf("create file: %s: is a directory", pth)
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

// ReadChangelog reads the changelog file pointed to by pth. In contrast to
// [ReadReleases], the releases in the changelog are not parsed:
// [Changelog.Save] writes the added releases above the existing ones, below a
// preamble such as a "# Changelog" title. The preamble is everything before
// the first release header or, in a file with none, a leading "# " title line
// and the blank lines after it.
//
// It will not return an error if the read changelog is in an unsupported
// format.
func ReadChangelog(pth string) (*Changelog, error) {
	clg, err := readChangelog(pth)
	if err != nil {
		return nil, err
	}
	clg.preamble, clg.contents = splitPreamble(clg.contents)
	return clg, nil
}

// readChangelog reads the changelog file pointed to by pth without examining
// its contents.
func readChangelog(pth string) (*Changelog, error) {
	var err error
	if pth, err = filepath.Abs(pth); err != nil {
		return nil, fmt.Errorf("resolve changelog path: %w", err)
	}
	// Save replaces the file by renaming, which would replace a symbolic link
	// rather than update its target; resolve the link up front.
	if res, rerr := filepath.EvalSymlinks(pth); rerr == nil {
		pth = res
	}
	clg := &Changelog{pth: pth}
	if clg.contents, err = os.ReadFile(pth); err != nil { //nolint:gosec
		return nil, fmt.Errorf("read changelog: %w", err)
	}
	return clg, nil
}

// splitPreamble splits data into the preamble preceding the releases and the
// rest. See [ReadChangelog] for what the preamble is.
func splitPreamble(data []byte) (preamble, rest []byte) {
	if loc := releaseLineRx.FindIndex(data); loc != nil {
		return data[:loc[0]], data[loc[0]:]
	}
	if !bytes.HasPrefix(data, []byte("# ")) {
		return nil, data
	}
	end := bytes.IndexByte(data, '\n') + 1
	if end == 0 {
		return data, nil
	}
	for end < len(data) {
		nxt := bytes.IndexByte(data[end:], '\n') + 1
		if nxt == 0 || len(bytes.TrimSpace(data[end:end+nxt])) > 0 {
			break
		}
		end += nxt
	}
	return data[:end], data[end:]
}

// ReadReleases reads the changelog file pointed by pth (joined with elems when
// provided) and parses releases. It is used when you wish to examine releases
// or edit them. It will return an error if the format of the changelog is
// incompatible.
func ReadReleases(pth string, elems ...string) (*Changelog, error) {
	pth = filepath.Join(append([]string{pth}, elems...)...)
	cl, err := readChangelog(pth)
	if err != nil {
		return nil, err
	}

	curr, num := -1, 0
	var preamble []string
	scn := bufio.NewScanner(bytes.NewReader(cl.contents))
	for scn.Scan() {
		num++
		lin := scn.Text()
		if releaseLineRx.MatchString(lin) {
			var rel *Release
			if rel, err = ReleaseFromHeader(lin, WithNoFormatting); err != nil {
				format := "parse release header on line %d: %w"
				return nil, fmt.Errorf(format, num, err)
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
// The saved file keeps the mode of the file it replaces; a new file gets 0644.
func (clg *Changelog) Save() error {
	buf := &bytes.Buffer{}
	buf.Write(clg.preamble)
	for _, rel := range clg.Releases {
		buf.WriteString(rel.String())
		buf.WriteString("\n") // A blank line ends each release.
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
	// The temporary file is created with mode 0600; give it the mode of the
	// file it replaces, or 0644 for a new changelog.
	mode := os.FileMode(0o644)
	if inf, serr := os.Stat(clg.pth); serr == nil {
		mode = inf.Mode().Perm()
	}
	if err = tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set changelog mode: %w", err)
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
