# gmmce

The [gomake](https://github.com/ctx42/gomake) `:doc:mce` target — inject Go
example function bodies into Markdown files, so documentation code blocks stay
in sync with compilable, tested examples.

<!-- TOC -->
* [gmmce](#gmmce)
  * [Overview](#overview)
  * [Features](#features)
  * [Prerequisites](#prerequisites)
  * [Installation](#installation)
    * [As a built-in gomake target](#as-a-built-in-gomake-target)
    * [As a per-project target](#as-a-per-project-target)
  * [Usage](#usage)
    * [Markers](#markers)
    * [Use as a library](#use-as-a-library)
<!-- TOC -->

## Overview

`gmmce` keeps the Go code blocks in your Markdown docs honest. Instead of
copy-pasting a snippet into `README.md` and watching it rot, you write a real
`Example` function in a `*_test.go` file — one the `go` toolchain compiles and
`go test` verifies — and mark where its body belongs in the Markdown with an
HTML comment. Running `:doc:mce` scans the source tree for those examples and
rewrites each marked code fence with the current example body.

It is a gomake target: compile it into the gomake binary once and run
`gomake :doc:mce` in any repository, with no per-project `makefile.go`. The
exported `Doc.Mce` method is also usable as a plain library.

## Features

- **Single source of truth** — docs show the same code `go test` compiles.
- **Recursive scan** — collects `Example*` functions from every `*_test.go`
  under the scanned directory.
- **Idempotent** — re-running with unchanged examples leaves the file byte-for-
  byte identical.
- **Insert or replace** — writes a fresh code fence when a marker has none, and
  overwrites the existing one when it does.
- **Unknown markers preserved** — a marker with no matching example is left
  untouched, never emptied.

## Prerequisites

- [gomake](https://github.com/ctx42/gomake) — the binary that hosts the target.
- The Go toolchain — `gmmce` parses `*_test.go` files with `go/parser`.

## Installation

### As a built-in gomake target

Add `gmmce` to the `targets.yaml` at your gomake source root:

```yaml
imports:
  - import: github.com/ctx42/gmtask/pkg/gmmce
```

Then rebuild the gomake binary so the target is compiled in:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

The `:doc:mce` target is now available in every project the binary is used
from.

### As a per-project target

Pull `gmmce` into one project without rebuilding the binary. Add it to the
project's module, then import it in `makefile.go` with a `//gomake:import`
comment (the blank identifier is required):

```shell
go get github.com/ctx42/gmtask/pkg/gmmce
```

```go
//go:build gomake

package main

import (
	_ "github.com/ctx42/gmtask/pkg/gmmce" //gomake:import
)
```

`:doc:mce` is now available in that project only. Add a namespace prefix — e.g.
`//gomake:import docs` — to expose it as `:docs:doc:mce` instead.

## Usage

Run from the repository root:

```shell
gomake :doc:mce                       # update README.md in the current dir
gomake :doc:mce --dir ./pkg           # scan ./pkg, update ./pkg/README.md
gomake :doc:mce --file docs/API.md    # update a specific Markdown file
gomake :doc:mce -h                    # show help
```

The flags:

```text
Usage of :doc:mce:
      --dir     root directory to scan for Go examples
      --file    Markdown file to update (default: README.md in --dir)
  -h, --help    show help
```

`--dir` sets the root scanned for example functions (default `.`). `--file`
selects the Markdown file to rewrite; when omitted it defaults to `README.md`
inside `--dir`. Each matched example is reported on stdout, followed by the file
being written.

### Markers

Mark an insertion point in the Markdown with an HTML comment naming the
example, then let `gmmce` fill the code fence beneath it:

```markdown
<!-- gmmce:pkg1/Example_case1 -->
```

The key after `gmmce:` is `relpath/FuncName`, where `relpath` is the directory
holding the `*_test.go` file relative to the Markdown file's own directory, and
`FuncName` is the example function's name. Examples in the same directory as the
Markdown file drop the path and use `FuncName` alone.

Given this example function in `pkg1/examples_test.go`:

```go
func Example_case1() {
	fmt.Println("Hello world.")

	// Output: Hello world.
}
```

the marker above expands to:

````markdown
<!-- gmmce:pkg1/Example_case1 -->
```go
fmt.Println("Hello world.")

// Output: Hello world.
```
````

The outer braces and one level of tab indentation are stripped, so the fence
holds the example body exactly as it reads in the source.

### Use as a library

Add the module to your project:

```shell
go get github.com/ctx42/gmtask/pkg/gmmce
```

`Doc.Mce` takes the gomake `*ring.Ring` for I/O and argument access, reading the
same `--dir` and `--file` flags from `rng.Args()`:

```go
import (
	"context"
	"os"

	"github.com/ctx42/gmtask/pkg/gmmce"
	"github.com/ctx42/ring/pkg/ring"
)

rng := ring.New(
	ring.WithEnv(os.Environ()),
	ring.WithArgs([]string{"--dir", "./pkg"}),
)

err := gmmce.Doc{}.Mce(context.Background(), rng)
```
