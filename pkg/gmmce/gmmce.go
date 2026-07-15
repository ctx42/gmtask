// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmmce provides the :doc:mce gomake target, which injects Go example
// function bodies into placeholders in Markdown files.
package gmmce

import (
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
		_, _ = fmt.Fprint(rng.Stderr(), head+xflag.HelpOptions(fs))
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
	rng = rng.SetArgs(fs.Args())
	if fs.GetBool("help") {
		fs.Usage()
		return nil
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
		return err
	}

	keys := make([]string, 0, len(examples))
	for k := range examples {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(rng.Stdout(), "Found %s\n", k)
	}

	updated := injectExamples(string(data), examples)

	_, _ = fmt.Fprintf(rng.Stdout(), "Writing %s\n", file)
	if err = os.WriteFile(absFile, []byte(updated), 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("write markdown file: %w", err)
	}
	return nil
}

// findExamples walks root recursively and collects all Go example functions
// from *_test.go files. The returned map keys are "relpath/FuncName" where
// relpath is the directory of the test file relative to mdDir.
func findExamples(
	ctx context.Context,
	root, mdDir string,
) (map[string]string, error) {

	examples := make(map[string]string)
	err := filepath.Walk(
		root,
		func(pth string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(pth, "_test.go") {
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

// parseExamples parses a Go source file and returns a map of function name to
// body text for every function whose name starts with "Example". The body text
// has the outer braces and one level of tab indentation removed.
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
		if !strings.HasPrefix(fn.Name.Name, "Example") {
			continue
		}
		// Slice by brace byte offsets so single-line bodies keep their content.
		lo := fset.Position(fn.Body.Lbrace).Offset
		hi := fset.Position(fn.Body.Rbrace).Offset
		inner := string(src[lo+1 : hi])
		bodyLines := strings.Split(inner, "\n")
		stripped := make([]string, len(bodyLines))
		for i, line := range bodyLines {
			stripped[i] = strings.TrimPrefix(line, "\t")
		}
		examples[fn.Name.Name] = strings.TrimSpace(
			strings.Join(stripped, "\n"),
		)
	}
	return examples, nil
}

// injectExamples processes Markdown content replacing code fences that follow
// gmmce markers with the corresponding example bodies from examples. If a code
// fence does not already exist after a marker, one is inserted. Markers with no
// matching entry in examples are left unchanged.
func injectExamples(content string, examples map[string]string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		line := lines[i]
		if !strings.HasPrefix(line, markerPfx) ||
			!strings.HasSuffix(line, markerSfx) {
			result = append(result, line)
			i++
			continue
		}

		key := strings.TrimPrefix(line, markerPfx)
		key = strings.TrimSuffix(key, markerSfx)
		result = append(result, line)
		i++

		body, ok := examples[key]
		if !ok {
			continue
		}

		// Skip an existing code fence if present, allowing blank lines between
		// the marker and the opening fence (common Markdown layout).
		j := i
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j < len(lines) && strings.HasPrefix(lines[j], "```") {
			i = j + 1 // skip opening fence
			for i < len(lines) && strings.TrimSpace(lines[i]) != "```" {
				i++ // skip fence body
			}
			if i < len(lines) {
				i++ // skip closing fence
			}
		}

		result = append(result, "```go", body, "```")
	}
	return strings.Join(result, "\n")
}
