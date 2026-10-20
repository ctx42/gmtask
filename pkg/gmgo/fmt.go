// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"
)

// Limits of Go source lines when no .editorconfig sets them.
const (
	defLineLimit = 80
	defTabWidth  = 4
)

// Fmt reports the Go files under the current directory that the changes
// since the last commit touch and gofmt would rewrite, as "<file>: not
// formatted", and the lines those changes add or change that are longer than
// the line limit, as "<file>:<line>: <width> columns, limit <limit>". Staged,
// unstaged, and untracked changes count; --rev compares with another revision
// than HEAD. The limit and the tab width are those the .editorconfig files of
// the current directory and its parents set for Go files, as [goLineLimits]
// reads them, else 80 and 4. It returns an error wrapping [ErrFmt] when it
// reports anything.
//
// Example usage:
//
//	gomake :go:fmt
//	gomake :go:fmt --rev origin/master
func (Go) Fmt(ctx context.Context, rng *ring.Ring) error {
	tgtName := ":go:fmt"
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.SetOutput(rng.Stderr())
	head := fmt.Sprintf("Usage of %s\n", tgtName)
	fs.Usage = func() { _, _ = fmt.Fprint(rng.Stderr(), head+fs.HelpOptions()) }
	fs.BoolSL("help", "h", false, "show help")
	fs.StringSL("rev", "r", "HEAD", "revision to compare the changes with")
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}

	limit, tab, err := goLineLimits(".")
	if err != nil {
		return err
	}
	changed, err := gitaid.ChangedLines(ctx, "", fs.GetString("rev"), "*.go")
	if err != nil {
		return fmt.Errorf("changed lines: %w", err)
	}
	var issues int
	for _, pth := range slices.Sorted(maps.Keys(changed)) {
		lines, err := fmtIssues(pth, changed[pth], limit, tab)
		if err != nil {
			return err
		}
		for _, line := range lines {
			_, _ = fmt.Fprintln(rng.Stdout(), line)
		}
		issues += len(lines)
	}
	if issues > 0 {
		return fmt.Errorf("%w: %d issues", ErrFmt, issues)
	}
	return nil
}

// fmtIssues returns the issues [Go.Fmt] reports for the Go file at pth whose
// lines nums changed: the file when gofmt would rewrite it, and every changed
// line longer than limit columns, tabs counting up to the next multiple of
// tab. A limit of zero checks no line.
func fmtIssues(pth string, nums []int, limit, tab int) ([]string, error) {
	src, err := os.ReadFile(pth) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", pth, err)
	}
	var issues []string
	if out, err := format.Source(src); err != nil || !bytes.Equal(out, src) {
		issues = append(issues, pth+": not formatted")
	}
	if limit == 0 {
		return issues, nil
	}
	lines := strings.Split(string(src), "\n")
	for _, num := range nums {
		if num < 1 || num > len(lines) {
			continue
		}
		if width := lineWidth(lines[num-1], tab); width > limit {
			msg := "%s:%d: %d columns, limit %d"
			issues = append(issues, fmt.Sprintf(msg, pth, num, width, limit))
		}
	}
	return issues, nil
}

// lineWidth returns the number of columns line takes, a tab reaching the next
// multiple of tab and a trailing carriage return left out.
func lineWidth(line string, tab int) int {
	var width int
	for _, chr := range strings.TrimSuffix(line, "\r") {
		if chr == '\t' {
			width += tab - width%tab
			continue
		}
		width++
	}
	return width
}

// goLineLimits returns the line limit and the tab width the .editorconfig
// files of dir and its parents set for Go files: "max_line_length", where
// "off" gives a limit of zero, and "tab_width", else a numeric "indent_size".
// A closer file wins, and one with "root = true" ends the search. An unset
// limit is 80 and an unset tab width 4.
func goLineLimits(dir string) (int, int, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return 0, 0, err
	}
	props := make(map[string]string)
	for {
		pth := filepath.Join(dir, ".editorconfig")
		got, root, err := editorConfigGo(pth)
		if err != nil {
			return 0, 0, err
		}
		for key, val := range got {
			if _, ok := props[key]; !ok {
				props[key] = val
			}
		}
		parent := filepath.Dir(dir)
		if root || parent == dir {
			break
		}
		dir = parent
	}
	limit, tab := defLineLimit, defTabWidth
	if val, ok := props["max_line_length"]; ok {
		limit, _ = strconv.Atoi(val) // "off" and any other word give zero.
	}
	for _, key := range []string{"indent_size", "tab_width"} {
		if num, err := strconv.Atoi(props[key]); err == nil && num > 0 {
			tab = num
		}
	}
	return limit, tab, nil
}

// editorConfigGo returns the properties the .editorconfig file at pth sets
// for Go files, keys and values in lower case, a later section overriding an
// earlier one, and whether its preamble says "root = true". A missing file
// sets nothing.
func editorConfigGo(pth string) (map[string]string, bool, error) {
	data, err := os.ReadFile(pth) //nolint:gosec
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("editorconfig: %w", err)
	}
	props := make(map[string]string)
	var root, inSection, matches bool
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection, matches = true, matchesGo(line[1:len(line)-1])
			continue
		}
		key, val, ok := editorProp(line)
		switch {
		case !ok:

		case !inSection:
			root = root || key == "root" && val == "true"

		case matches:
			props[key] = val
		}
	}
	return props, root, nil
}

// editorProp returns the key and the value, in lower case, of the
// .editorconfig property line, trimmed; false for a comment, a blank line, or
// a line without "=".
func editorProp(line string) (string, string, bool) {
	if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
		return "", "", false
	}
	key, val, ok := strings.Cut(line, "=")
	key = strings.ToLower(strings.TrimSpace(key))
	val = strings.ToLower(strings.TrimSpace(val))
	return key, val, ok
}

// matchesGo reports whether the .editorconfig section glob matches a Go file
// in the directory of the .editorconfig file. Braces list alternatives; a
// glob naming a subdirectory never matches.
func matchesGo(glob string) bool {
	for _, alt := range expandBraces(glob) {
		alt = strings.TrimPrefix(alt, "**/")
		if strings.Contains(alt, "/") {
			continue
		}
		if ok, _ := path.Match(alt, "x.go"); ok {
			return true
		}
	}
	return false
}

// expandBraces returns the alternatives the "{a,b}" groups of glob stand for,
// in order: "*.{go,mod}" gives "*.go" and "*.mod".
func expandBraces(glob string) []string {
	start := strings.IndexByte(glob, '{')
	end := strings.IndexByte(glob[max(start, 0):], '}') + max(start, 0)
	if start < 0 || end <= start {
		return []string{glob}
	}
	var alts []string
	for alt := range strings.SplitSeq(glob[start+1:end], ",") {
		alts = append(alts, expandBraces(glob[:start]+alt+glob[end+1:])...)
	}
	return alts
}
