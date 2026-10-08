// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmmce

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_Doc_Mce(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		dir := t.TempDir()
		copyExamples(t, dir)
		readme := oskit.Write(t, "# Doc\n\n"+
			"<!-- gmmce:pkg1/Example_case1 -->\n", dir, "README.md")

		rng := tst.Ring("--dir", dir)

		// --- When ---
		err := Doc{}.Mce(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Found pkg1/Example_case1\n", tst.Stdout())
		want := "# Doc\n\n" +
			"<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```go\n" +
			"fmt.Println(\"Hello world.\")\n" +
			"\n" +
			"// Output: Hello world.\n" +
			"```\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, readme))
	})

	t.Run("idempotent", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		dir := t.TempDir()
		copyExamples(t, dir)
		input := "# Doc\n\n" +
			"<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```go\n" +
			"fmt.Println(\"Hello world.\")\n" +
			"\n" +
			"// Output: Hello world.\n" +
			"```"
		readme := oskit.Write(t, input, dir, "README.md")

		rng := tst.Ring("--dir", dir)

		// --- When ---
		err := Doc{}.Mce(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Found pkg1/Example_case1\n", tst.Stdout())
		assert.Equal(t, input, oskit.ReadFileStr(t, readme))
	})

	t.Run("custom file flag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStdout()

		dir := t.TempDir()
		copyExamples(t, dir)
		md := oskit.Write(t, "<!-- gmmce:pkg1/Example_case1 -->\n",
			dir, "DOCS.md")

		rng := tst.Ring("--dir", dir, "--file", md)

		// --- When ---
		err := Doc{}.Mce(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "Found pkg1/Example_case1\n", tst.Stdout())
		want := "<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```go\n" +
			"fmt.Println(\"Hello world.\")\n" +
			"\n" +
			"// Output: Hello world.\n" +
			"```\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, md))
	})

	t.Run("warns about unmatched marker", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout().WetStderr()
		dir := t.TempDir()
		readme := oskit.Write(t, "<!-- gmmce:pkg1/Nope -->\n", dir, "README.md")
		rng := tst.Ring("--dir", dir)

		// --- When ---
		err := Doc{}.Mce(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "#gomake WARN# no example for marker: pkg1/Nope\n"
		assert.Equal(t, want, tst.Stderr())
		assert.Equal(t, "Writing "+readme+"\n", tst.Stdout())
		want = "<!-- gmmce:pkg1/Nope -->\n"
		assert.Equal(t, want, oskit.ReadFileStr(t, readme))
	})

	t.Run("error - readme not found", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		dir := t.TempDir()
		rng := tst.Ring("--dir", dir)

		// --- When ---
		err := Doc{}.Mce(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "README.md", err)
	})

	t.Run("show help", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		rng := tst.Ring("--help")

		// --- When ---
		err := Doc{}.Mce(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"Usage of :doc:mce:\n" +
			"      --dir     root directory to scan for Go examples\n" +
			"      --file    Markdown file to update (default: README.md in --dir)\n" +
			"  -h, --help    show help\n"
		assert.Equal(t, want, tst.Stderr())
	})

	t.Run("error - positional argument", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		dir := t.TempDir()
		readme := oskit.Write(t, "x\n", dir, "README.md")
		rng := tst.Ring("--dir", dir, "DOCS.md")

		// --- When ---
		err := Doc{}.Mce(t.Context(), rng)

		// --- Then ---
		assert.ErrorContain(t, "unexpected arguments: [DOCS.md]", err)
		assert.Equal(t, "x\n", oskit.ReadFileStr(t, readme))
	})

	t.Run("error - unknown flag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t).WetStderr()

		rng := tst.Ring("-unknown")

		// --- When ---
		err := Doc{}.Mce(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -unknown", err)
		assert.Contain(t, "flag provided but not defined: -unknown", tst.Stderr())
	})
}

func Test_findExamples(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		root := must.Value(filepath.Abs("testdata"))
		mdDir := root

		// --- When ---
		have, err := findExamples(ctx, root, mdDir)

		// --- Then ---
		assert.NoError(t, err)
		assert.HasKey(t, "pkg1/Example_case1", have)
		assert.Equal(t,
			"fmt.Println(\"Hello world.\")\n\n// Output: Hello world.",
			have["pkg1/Example_case1"],
		)
	})

	t.Run("root in mdDir", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		root := must.Value(filepath.Abs(filepath.Join("testdata", "pkg1")))
		mdDir := root

		// --- When ---
		have, err := findExamples(ctx, root, mdDir)

		// --- Then ---
		assert.NoError(t, err)
		assert.HasKey(t, "Example_case1", have)
	})

	t.Run("skips ignored directories", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		copyExamples(t, root)
		bad := "package x\n\nfunc {\n"
		for _, dir := range []string{"testdata", "vendor", ".git", "_tmp"} {
			oskit.Write(t, bad, oskit.MkdirAll(t, root, dir), "x_test.go")
		}

		// --- When ---
		have, err := findExamples(t.Context(), root, root)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{"pkg1/Example_case1"}
		assert.Equal(t, want, slices.Sorted(maps.Keys(have)))
	})

	t.Run("error - not existing root", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()

		root := filepath.Join("testdata", "not_existing")

		// --- When ---
		_, err := findExamples(ctx, root, root)

		// --- Then ---
		assert.ErrorContain(t, "not_existing", err)
	})

	t.Run("error - context cancelled", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		root := must.Value(filepath.Abs("testdata"))

		// --- When ---
		_, err := findExamples(ctx, root, root)

		// --- Then ---
		assert.ErrorIs(t, context.Canceled, err)
	})
}

func Test_parseExamples(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join("testdata", "pkg1", "examples_test.go")

		// --- When ---
		have, err := parseExamples(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.HasKey(t, "Example_case1", have)
		want := "fmt.Println(\"Hello world.\")\n\n// Output: Hello world."
		assert.Equal(t, want, have["Example_case1"])
	})

	t.Run("non-example functions are skipped", func(t *testing.T) {
		// --- Given ---
		src := "package foo\n\nfunc TestFoo(t interface{}) {}\n"
		pth := oskit.Write(t, src, t.TempDir(), "foo_test.go")

		// --- When ---
		have, err := parseExamples(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 0, have)
	})

	t.Run("single-line empty body", func(t *testing.T) {
		// --- Given ---
		src := "package foo\n\nfunc Example() {}\n"
		pth := oskit.Write(t, src, t.TempDir(), "eg_test.go")

		// --- When ---
		have, err := parseExamples(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", have["Example"])
	})

	t.Run("single-line non-empty body", func(t *testing.T) {
		// --- Given ---
		src := "package foo\n\nfunc Example() { fmt.Println(\"x\") }\n"
		pth := oskit.Write(t, src, t.TempDir(), "eg_test.go")

		// --- When ---
		have, err := parseExamples(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "fmt.Println(\"x\")", have["Example"])
	})

	t.Run("error - not existing file", func(t *testing.T) {
		// --- When ---
		_, err := parseExamples("not_existing_test.go")

		// --- Then ---
		assert.ErrorContain(t, "not_existing_test.go", err)
	})
}

func Test_injectExamples(t *testing.T) {
	examples := map[string]string{
		"pkg1/Example_case1": "fmt.Println(\"hello\")",
	}

	t.Run("insert fence when absent", func(t *testing.T) {
		// --- Given ---
		input := "text\n\n<!-- gmmce:pkg1/Example_case1 -->\n\nmore"

		// --- When ---
		have, _, err := injectExamples(input, examples)

		// --- Then ---
		assert.NoError(t, err)
		want := "text\n\n" +
			"<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```go\n" +
			"fmt.Println(\"hello\")\n" +
			"```\n\nmore"
		assert.Equal(t, want, have)
	})

	t.Run("replace existing fence", func(t *testing.T) {
		// --- Given ---
		input := "<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```go\n" +
			"old content\n" +
			"```\n" +
			"after"

		// --- When ---
		have, _, err := injectExamples(input, examples)

		// --- Then ---
		assert.NoError(t, err)
		want := "<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```go\n" +
			"fmt.Println(\"hello\")\n" +
			"```\n" +
			"after"
		assert.Equal(t, want, have)
	})

	t.Run("replace fence after blank lines", func(t *testing.T) {
		// --- Given ---
		input := "<!-- gmmce:pkg1/Example_case1 -->\n" +
			"\n" +
			"```go\n" +
			"old content\n" +
			"```\n" +
			"after"

		// --- When ---
		have, _, err := injectExamples(input, examples)

		// --- Then ---
		assert.NoError(t, err)
		want := "<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```go\n" +
			"fmt.Println(\"hello\")\n" +
			"```\n" +
			"after"
		assert.Equal(t, want, have)
	})

	t.Run("unknown marker key is left unchanged", func(t *testing.T) {
		// --- Given ---
		input := "<!-- gmmce:unknown/Func -->\n```go\nold\n```"

		// --- When ---
		have, unmatched, err := injectExamples(input, examples)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, input, have)
		assert.Equal(t, []string{"unknown/Func"}, unmatched)
	})

	t.Run("body containing fence", func(t *testing.T) {
		// --- Given ---
		exs := map[string]string{"k": "a\n```\nb"}
		input := "" +
			"<!-- gmmce:k -->\n" +
			"````go\n" +
			"a\n" +
			"```\n" +
			"b\n" +
			"````\n" +
			"after"

		// --- When ---
		have, _, err := injectExamples(input, exs)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, input, have)
	})

	t.Run("marker inside fence", func(t *testing.T) {
		// --- Given ---
		input := "" +
			"```markdown\n" +
			"<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```\n" +
			"\n" +
			"text\n" +
			"```go\n" +
			"code\n" +
			"```\n"

		// --- When ---
		have, _, err := injectExamples(input, examples)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, input, have)
	})

	t.Run("error - fence not closed", func(t *testing.T) {
		// --- Given ---
		input := "" +
			"<!-- gmmce:pkg1/Example_case1 -->\n" +
			"```go\n" +
			"old\n" +
			"# Next section\n"

		// --- When ---
		have, _, err := injectExamples(input, examples)

		// --- Then ---
		assert.ErrorIs(t, errFenceNotClosed, err)
		assert.ErrorContain(t, "pkg1/Example_case1", err)
		assert.Equal(t, "", have)
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		// --- Given ---
		input := "<!-- gmmce:pkg1/Example_case1 -->\r\n\r\nmore"

		// --- When ---
		have, _, err := injectExamples(input, examples)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"<!-- gmmce:pkg1/Example_case1 -->\r\n" +
			"```go\r\n" +
			"fmt.Println(\"hello\")\r\n" +
			"```\r\n" +
			"\r\n" +
			"more"
		assert.Equal(t, want, have)
	})

	t.Run("indented marker", func(t *testing.T) {
		// --- Given ---
		input := "- item\n  <!-- gmmce:pkg1/Example_case1 -->\n"

		// --- When ---
		have, _, err := injectExamples(input, examples)

		// --- Then ---
		assert.NoError(t, err)
		want := "" +
			"- item\n" +
			"  <!-- gmmce:pkg1/Example_case1 -->\n" +
			"  ```go\n" +
			"  fmt.Println(\"hello\")\n" +
			"  ```\n"
		assert.Equal(t, want, have)
	})

	t.Run("no markers unchanged", func(t *testing.T) {
		// --- Given ---
		input := "# Title\n\nSome text.\n"

		// --- When ---
		have, _, err := injectExamples(input, examples)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, input, have)
	})

	t.Run("multiple markers", func(t *testing.T) {
		// --- Given ---
		exs := map[string]string{
			"pkg/ExampleA": "bodyA",
			"pkg/ExampleB": "bodyB",
		}
		input := "<!-- gmmce:pkg/ExampleA -->\n" +
			"<!-- gmmce:pkg/ExampleB -->\n"

		// --- When ---
		have, _, err := injectExamples(input, exs)

		// --- Then ---
		assert.NoError(t, err)
		want := "<!-- gmmce:pkg/ExampleA -->\n" +
			"```go\n" +
			"bodyA\n" +
			"```\n" +
			"<!-- gmmce:pkg/ExampleB -->\n" +
			"```go\n" +
			"bodyB\n" +
			"```\n"
		assert.Equal(t, want, have)
	})
}
