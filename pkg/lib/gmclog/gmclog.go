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
// or [ReadReleases] to parse the releases for inspection or editing. Releases
// are ordered youngest to oldest by [Release.Compare] when saved.
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
	scn := bufio.NewScanner(bytes.NewReader(cl.contents))
	for scn.Scan() {
		lin := scn.Text()
		if strings.HasPrefix(lin, "## ") {
			var rel *Release
			if rel, err = ReleaseFromHeader(lin, WithNoFormatting); err != nil {
				return nil, err
			}
			cl.Releases = append(cl.Releases, rel)
			curr = len(cl.Releases) - 1
			continue
		}
		if curr >= 0 {
			cl.Releases[curr].Changes = append(cl.Releases[curr].Changes, lin)
		}
	}
	if err = scn.Err(); err != nil {
		return nil, fmt.Errorf("scan changelog: %w", err)
	}
	for _, rel := range cl.Releases {
		rel.finalize()
	}
	// The parsed releases fully represent the file; drop the raw contents so
	// Save reserializes from Releases instead of appending the file to itself.
	cl.contents = nil
	return cl, nil
}

// AddRelease adds the release(s) to the changelog. Before exiting, it sorts
// releases from youngest to oldest according to semantic version rules.
func (cl *Changelog) AddRelease(rel ...*Release) {
	cl.Releases = append(cl.Releases, rel...)
	sort.Sort(sort.Reverse(ReleaseSlice(cl.Releases)))
}

// Save saves the changelog, overwriting the original file with the releases.
func (cl *Changelog) Save() (err error) {
	fil, err := os.Create(cl.pth)
	if err != nil {
		return fmt.Errorf("create changelog file: %w", err)
	}
	defer func() {
		if cerr := fil.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close changelog file: %w", cerr)
		}
	}()

	buf := &bytes.Buffer{}
	for _, rel := range cl.Releases {
		buf.WriteString(rel.String())
	}
	buf.Write(cl.contents)
	if _, err = fil.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write changelog: %w", err)
	}
	return nil
}
