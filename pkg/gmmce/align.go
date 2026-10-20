// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmmce

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"
)

// ErrUnaligned is returned by :doc:align --check when a Markdown file holds a
// table whose columns are not aligned.
var ErrUnaligned = errors.New("markdown tables not aligned")

// Align aligns the columns of the Markdown tables in the files named as
// arguments: every cell is padded to the width of the widest cell of its
// column, counted in characters, and the dashes of the separator row fill
// each column flush to the pipes, keeping its alignment colons. A table is a
// run of lines starting and ending with "|", outside code blocks, whose
// second line is a separator row; its indentation is kept. Cells split at
// pipes not escaped with a backslash. A file whose tables change is
// rewritten and reported "Aligned <file>". With --check, no file is written:
// each file to change is reported "Unaligned <file>", and the target returns
// an error wrapping [ErrUnaligned].
//
// Example usage:
//
//	gomake :doc:align README.md docs/guide.md
//	gomake :doc:align --check README.md
func (Doc) Align(_ context.Context, rng *ring.Ring) error {
	tgtName := ":doc:align"
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.SetOutput(rng.Stderr())
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s: [flags] file...\n", tgtName)
		_, _ = fmt.Fprint(rng.Stderr(), head+fs.HelpOptions())
	}
	fs.BoolSL("help", "h", false, "show help")
	fs.BoolSL("check", "c", false, "report unaligned files, write none")
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}
	if fs.NArg() == 0 {
		return errors.New("no Markdown file given")
	}

	var unaligned []string
	for _, pth := range fs.Args() {
		data, err := os.ReadFile(pth) //nolint:gosec
		if err != nil {
			return fmt.Errorf("read markdown file: %w", err)
		}
		aligned := alignTables(string(data))
		if aligned == string(data) {
			continue
		}
		if fs.GetBool("check") {
			unaligned = append(unaligned, pth)
			_, _ = fmt.Fprintf(rng.Stdout(), "Unaligned %s\n", pth)
			continue
		}
		if err = writeFile(pth, []byte(aligned)); err != nil {
			return fmt.Errorf("write markdown file: %w", err)
		}
		_, _ = fmt.Fprintf(rng.Stdout(), "Aligned %s\n", pth)
	}
	if len(unaligned) > 0 {
		list := strings.Join(unaligned, ", ")
		return fmt.Errorf("%w: %s", ErrUnaligned, list)
	}
	return nil
}

// alignTables returns the Markdown document doc with the columns of its
// tables aligned, as [Doc.Align] describes.
func alignTables(doc string) string {
	lines := strings.Split(doc, "\n")
	var fence string
	for idx := 0; idx < len(lines); idx++ {
		line := lines[idx]
		if fence != "" {
			if closesFence(line, fence) {
				fence = ""
			}
			continue
		}
		if fence = openFence(line); fence != "" {
			continue
		}
		end := idx
		for end < len(lines) && isTableRow(lines[end]) {
			end++
		}
		if end-idx >= 2 && isSeparator(lines[idx+1]) {
			copy(lines[idx:end], alignTable(lines[idx:end]))
			idx = end - 1
		}
	}
	return strings.Join(lines, "\n")
}

// isTableRow reports whether line, its indentation and a carriage return
// left out, starts and ends with a pipe.
func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) >= 2 &&
		strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

// isSeparator reports whether line is the separator row of a table: every
// cell holds at least one dash, and nothing but dashes, colons, and spaces.
func isSeparator(line string) bool {
	for _, cell := range splitRow(line) {
		cell = strings.TrimSpace(cell)
		if !strings.Contains(cell, "-") || strings.Trim(cell, "-:") != "" {
			return false
		}
	}
	return true
}

// alignTable returns the rows of a table, the second its separator row, with
// their columns aligned and the indentation and line ending of the first row.
func alignTable(rows []string) []string {
	indent := rows[0][:len(rows[0])-len(strings.TrimLeft(rows[0], " \t"))]
	eol := ""
	if strings.HasSuffix(rows[0], "\r") {
		eol = "\r"
	}
	cells := make([][]string, len(rows))
	var widths []int
	for i, row := range rows {
		cells[i] = splitRow(row)
		for col, cell := range cells[i] {
			cell = strings.TrimSpace(cell)
			cells[i][col] = cell
			// A column is one character wide at least, so its separator
			// holds a dash between its colons.
			if col == len(widths) {
				widths = append(widths, 1)
			}
			if i != 1 {
				widths[col] = max(widths[col], utf8.RuneCountInString(cell))
			}
		}
	}
	out := make([]string, len(rows))
	for i := range rows {
		var buf strings.Builder
		buf.WriteString(indent + "|")
		for col, width := range widths {
			var cell string
			if col < len(cells[i]) {
				cell = cells[i][col]
			}
			if i == 1 {
				buf.WriteString(separatorCell(cell, width+2) + "|")
				continue
			}
			pad := width - utf8.RuneCountInString(cell)
			buf.WriteString(" " + cell + strings.Repeat(" ", pad) + " |")
		}
		out[i] = buf.String() + eol
	}
	return out
}

// separatorCell returns a separator cell of width dashes keeping the
// alignment colons of cell: a leading one, a trailing one, or both.
func separatorCell(cell string, width int) string {
	left, right := strings.HasPrefix(cell, ":"), strings.HasSuffix(cell, ":")
	dashes := []byte(strings.Repeat("-", width))
	if left {
		dashes[0] = ':'
	}
	if right {
		dashes[width-1] = ':'
	}
	return string(dashes)
}

// splitRow returns the cells of the table row line: the text between its
// outer pipes, split at every pipe not escaped with a backslash.
func splitRow(line string) []string {
	row := strings.TrimSpace(line)
	row = row[1 : len(row)-1]
	var cells []string
	var cell strings.Builder
	for i := 0; i < len(row); i++ {
		switch {
		case row[i] == '\\' && i+1 < len(row):
			cell.WriteString(row[i : i+2])
			i++

		case row[i] == '|':
			cells = append(cells, cell.String())
			cell.Reset()

		default:
			cell.WriteByte(row[i])
		}
	}
	return append(cells, cell.String())
}
