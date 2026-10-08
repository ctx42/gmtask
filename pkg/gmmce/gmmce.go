// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmmce provides the :doc:mce gomake target, which injects Go example
// function bodies into placeholders in Markdown files.
package gmmce

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"
)

// Injection marker delimiters. A marker is an HTML comment of the form
// "<!-- gmmce:path/Func -->" naming the example whose body follows it.
const (
	// markerPfx is the prefix opening a gmmce injection marker.
	markerPfx = "<!-- gmmce:"

	// markerSfx is the suffix closing a gmmce injection marker.
	markerSfx = " -->"
)

// errFenceNotClosed is returned when the code fence following a marker is
// never closed; replacing it would swallow the rest of the document.
var errFenceNotClosed = errors.New("code fence after marker not closed")

// Doc collects Markdown documentation targets.
type Doc struct{} //gomake:ns_root

// Mce injects Go example function bodies into placeholders in a Markdown file.
// It recursively scans the source directory for Go example functions in
// *_test.go files and replaces code fences following
// "<!-- gmmce:path/Func -->" markers in the Markdown file with the current
// example body.
//
// The path in a marker is relative to the Markdown file's directory.
func (Doc) Mce(ctx context.Context, rng *ring.Ring) error {
	tgtName := ":doc:mce"
	var dir, file string
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.SetOutput(rng.Stderr())
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s:\n", tgtName)
		_, _ = fmt.Fprint(rng.Stderr(), head+fs.HelpOptions())
	}
	fs.BoolSL("help", "h", false, "show help")
	fs.StringVar(&dir, "dir", ".", "root directory to scan for Go examples")
	fs.StringVar(
		&file,
		"file",
		"",
		"Markdown file to update (default: README.md in --dir)",
	)
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve source directory: %w", err)
	}
	if file == "" {
		file = filepath.Join(absDir, "README.md")
	}
	absFile, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("resolve markdown file: %w", err)
	}
	mdDir := filepath.Dir(absFile)

	data, err := os.ReadFile(absFile) //nolint:gosec
	if err != nil {
		return fmt.Errorf("read markdown file: %w", err)
	}

	examples, err := findExamples(ctx, absDir, mdDir)
	if err != nil {
		return fmt.Errorf("find examples: %w", err)
	}

	keys := make([]string, 0, len(examples))
	for k := range examples {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(rng.Stdout(), "Found %s\n", k)
	}

	updated, unmatched, err := injectExamples(string(data), examples)
	if err != nil {
		return fmt.Errorf("inject examples: %w", err)
	}
	for _, key := range unmatched {
		format := "#gomake WARN# no example for marker: %s\n"
		_, _ = fmt.Fprintf(rng.Stderr(), format, key)
	}

	if updated == string(data) {
		_, _ = fmt.Fprintf(rng.Stdout(), "Unchanged %s\n", file)
		return nil
	}
	if err = writeFile(absFile, []byte(updated)); err != nil {
		return fmt.Errorf("write markdown file: %w", err)
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "Wrote %s\n", file)
	return nil
}

// findExamples walks root recursively and collects all Go example functions
// from *_test.go files. The returned map keys are "relpath/FuncName" where
// relpath is the directory of the test file relative to mdDir. Like the go
// tool, the walk skips "testdata" and "vendor" directories and those whose
// names begin with "." or "_", except root itself.
func findExamples(
	ctx context.Context,
	root, mdDir string,
) (map[string]string, error) {

	examples := make(map[string]string)
	err := filepath.WalkDir(
		root,
		func(pth string, ent fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			if ent.IsDir() {
				if pth != root && skipDir(ent.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(pth, "_test.go") {
				return nil
			}
			pkgDir := filepath.Dir(pth)
			relDir, err := filepath.Rel(mdDir, pkgDir)
			if err != nil {
				return err
			}
			funcs, err := parseExamples(pth)
			if err != nil {
				return err
			}
			for name, body := range funcs {
				key := name
				if relDir != "." {
					// Markers always use slash separators (Markdown is OS-agnostic).
					key = filepath.ToSlash(relDir) + "/" + name
				}
				examples[key] = body
			}
			return nil
		},
	)
	return examples, err
}

// skipDir returns true for a directory name the go tool ignores when matching
// packages: "testdata", "vendor", and names beginning with "." or "_".
func skipDir(name string) bool {
	return name == "testdata" || name == "vendor" ||
		strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// parseExamples parses a Go source file and returns a map of function name to
// body text for every example function: one whose name starts with "Example"
// and that has no receiver, parameters or results. The body text has the
// outer braces and one level of tab indentation removed; lines that continue
// a raw string literal are kept verbatim.
func parseExamples(filename string) (map[string]string, error) {
	src, err := os.ReadFile(filename) //nolint:gosec
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	examples := make(map[string]string)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		// Example functions take no receiver, parameters or results.
		if !strings.HasPrefix(fn.Name.Name, "Example") || fn.Recv != nil ||
			fn.Type.Params.NumFields() > 0 || fn.Type.Results.NumFields() > 0 {
			continue
		}
		// Slice by brace byte offsets so single-line bodies keep their content.
		lbr := fset.Position(fn.Body.Lbrace)
		hi := fset.Position(fn.Body.Rbrace).Offset
		inner := string(src[lbr.Offset+1 : hi])
		raw := rawStringLines(fset, fn.Body)
		bodyLines := strings.Split(inner, "\n")
		stripped := make([]string, len(bodyLines))
		for i, line := range bodyLines {
			if raw[lbr.Line+i] {
				stripped[i] = line // Part of a raw string; keep it verbatim.
				continue
			}
			stripped[i] = strings.TrimPrefix(line, "\t")
		}
		examples[fn.Name.Name] = strings.TrimSpace(
			strings.Join(stripped, "\n"),
		)
	}
	return examples, nil
}

// rawStringLines returns the file line numbers inside node that continue a
// multi-line raw string literal: every line of the literal but its first.
func rawStringLines(fset *token.FileSet, node ast.Node) map[int]bool {
	lines := make(map[int]bool)
	ast.Inspect(node, func(nod ast.Node) bool {
		lit, ok := nod.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || lit.Value[0] != '`' {
			return true
		}
		first := fset.Position(lit.Pos()).Line
		last := fset.Position(lit.End()).Line
		for lin := first + 1; lin <= last; lin++ {
			lines[lin] = true
		}
		return true
	})
	return lines
}

// injectExamples processes Markdown content replacing code fences that follow
// gmmce markers with the corresponding example bodies from examples. If a code
// fence does not already exist after a marker, one is inserted. Markers with no
// matching entry in examples are left unchanged and their keys are returned in
// order; markers inside a code block are left unchanged too. A fence following
// a marker that is never closed yields an error wrapping errFenceNotClosed.
// The inserted fence is longer than any backtick run in the body, so a body
// holding a fence of its own cannot close it early.
func injectExamples(
	content string,
	examples map[string]string,
) (string, []string, error) {

	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	fence := "" // Fence of the code block being copied; empty outside one.
	var unmatched []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if fence != "" {
			if closesFence(line, fence) {
				fence = ""
			}
			result = append(result, line)
			continue
		}
		if fence = openFence(line); fence != "" {
			result = append(result, line)
			continue
		}
		result = append(result, line)
		marker := strings.TrimSpace(line)
		if !strings.HasPrefix(marker, markerPfx) ||
			!strings.HasSuffix(marker, markerSfx) {
			continue
		}

		key := strings.TrimPrefix(marker, markerPfx)
		key = strings.TrimSuffix(key, markerSfx)
		body, ok := examples[key]
		if !ok {
			unmatched = append(unmatched, key)
			continue
		}

		// Skip an existing code fence if present, allowing blank lines between
		// the marker and the opening fence (common Markdown layout).
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j < len(lines) {
			if old := openFence(lines[j]); old != "" {
				i = j + 1 // Skip the opening fence.
				for i < len(lines) && !closesFence(lines[i], old) {
					i++ // Skip the fence body.
				}
				if i == len(lines) {
					err := fmt.Errorf("%w: %s", errFenceNotClosed, key)
					return "", nil, err
				}
			}
		}

		// The fence takes the marker's indentation, so it stays inside a list
		// item, and its line ending, so a CRLF file stays CRLF.
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		eol := ""
		if strings.HasSuffix(line, "\r") {
			eol = "\r"
		}
		fen := bodyFence(body)
		result = append(result, indent+fen+"go"+eol)
		for _, bln := range strings.Split(body, "\n") {
			if bln != "" {
				bln = indent + bln
			}
			result = append(result, bln+eol)
		}
		result = append(result, indent+fen+eol)
	}
	return strings.Join(result, "\n"), unmatched, nil
}

// openFence returns the fence (a run of three or more backticks or tildes)
// opening a code block on line, or an empty string when line opens none.
func openFence(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return ""
	}
	fence := trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, trimmed[:1]))]
	if len(fence) < 3 {
		return ""
	}
	return fence
}

// closesFence returns true when line closes a code block opened with fence: it
// holds only the fence character, at least as many times as in fence.
func closesFence(line, fence string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) >= len(fence) &&
		strings.Trim(trimmed, fence[:1]) == ""
}

// bodyFence returns a backtick fence longer than any run of backticks in body
// and at least three backticks long.
func bodyFence(body string) string {
	longest, run := 0, 0
	for _, chr := range body {
		if chr != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	return strings.Repeat("`", max(3, longest+1))
}
