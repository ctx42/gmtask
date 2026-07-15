// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmmce_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gmtask/pkg/gmmce"
)

func ExampleDoc_Mce() {
	// Set up a throwaway project: a package holding a Go example function and
	// a Markdown file whose gmmce marker names that example.
	dir, err := os.MkdirTemp("", "gmmce-example")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	pkg := filepath.Join(dir, "greet")
	if err = os.MkdirAll(pkg, 0o750); err != nil {
		fmt.Println(err)
		return
	}
	example := "package greet\n\n" +
		"import \"fmt\"\n\n" +
		"func ExampleHello() {\n" +
		"\tfmt.Println(\"Hello world.\")\n" +
		"}\n"
	if err = os.WriteFile(
		filepath.Join(pkg, "greet_test.go"),
		[]byte(example),
		0o600,
	); err != nil {
		fmt.Println(err)
		return
	}

	readme := filepath.Join(dir, "README.md")
	marker := "<!-- gmmce:greet/ExampleHello -->\n"
	if err = os.WriteFile(readme, []byte(marker), 0o600); err != nil {
		fmt.Println(err)
		return
	}

	// Run the target. Progress output is routed to io.Discard to keep the
	// example output stable.
	rng := ring.New(ring.WithArgs([]string{"--dir", dir, "--file", readme}))
	rng.SetStdout(io.Discard)
	if err = (gmmce.Doc{}).Mce(context.Background(), rng); err != nil {
		fmt.Println(err)
		return
	}

	out, err := os.ReadFile(readme)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(string(out))
	// Output:
	// <!-- gmmce:greet/ExampleHello -->
	// ```go
	// fmt.Println("Hello world.")
	// ```
}
