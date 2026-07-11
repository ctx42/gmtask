# gmtask

[![Go](https://github.com/ctx42/gmtask/actions/workflows/test.yml/badge.svg)](https://github.com/ctx42/gmtask/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ctx42/gmtask.svg)](https://pkg.go.dev/github.com/ctx42/gmtask)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ctx42/gmtask)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE.md)

Reusable [gomake](https://github.com/ctx42/gomake) targets and supporting
libraries for Go projects.

<!-- TOC -->
* [gmtask](#gmtask)
  * [Overview](#overview)
  * [Targets](#targets)
  * [Libraries](#libraries)
  * [Installation](#installation)
    * [As built-in gomake targets](#as-built-in-gomake-targets)
    * [As per-project targets](#as-per-project-targets)
  * [License](#license)
<!-- TOC -->

## Overview

`gmtask` is a collection of Go packages that standardize routine project work.
Its targets compile into the gomake binary as built-in commands — available in
every repo with no per-project `makefile.go` — while its libraries are
standalone Go packages you can import directly.

## Targets

The [`gmgo`](pkg/gmgo) and [`gmbump`](pkg/gmbump) packages provide gomake
targets, run as `gomake :<name>` once compiled into the binary:

| Target             | Description                                          |
|--------------------|------------------------------------------------------|
| `:go:vet`          | Runs `go vet ./...`.                                 |
| `:go:lint`         | Runs golangci-lint (auto-installs it + config).      |
| `:go:lint:install` | Installs the latest golangci-lint.                   |
| `:go:lint:config`  | Downloads the shared `.golangci.yml`.                |
| `:go:test`         | Tests with race detector and coverage.               |
| `:go:test-v`       | Same as `:go:test`, verbose.                         |
| `:go:check`        | Runs vet, lint, and test in order.                   |
| `:go:build`        | Builds with version metadata via `-ldflags`.         |
| `:go:doc`          | Serves godoc and opens the browser.                  |
| `:bump`            | Tags the next version, writes the changelog, pushes. |

See the [`gmgo`](pkg/gmgo) and [`gmbump`](pkg/gmbump) READMEs for flags,
environment variables, the interactive flow, and library helpers.

## Libraries

| Package                            | Description                          |
|------------------------------------|--------------------------------------|
| [`pkg/lib/gmclog`](pkg/lib/gmclog) | Read/edit/write Markdown changelogs. |

## Installation

### As built-in gomake targets

Add both packages to the `targets.yaml` at your gomake source root, then
rebuild the binary so the targets are compiled in:

```yaml
imports:
  - import: github.com/ctx42/gmtask/pkg/gmgo
  - import: github.com/ctx42/gmtask/pkg/gmbump
```

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

The `:go:*` and `:bump` targets are then available in every project the binary
is used from.

### As per-project targets

Pull the packages into one project without rebuilding the binary. Add them to
the project's module, then import them in `makefile.go` with `//gomake:import`
comments (the blank identifier is required):

```shell
go get github.com/ctx42/gmtask/pkg/gmgo github.com/ctx42/gmtask/pkg/gmbump
```

```go
//go:build gomake

package main

import (
	_ "github.com/ctx42/gmtask/pkg/gmgo"   //gomake:import
	_ "github.com/ctx42/gmtask/pkg/gmbump" //gomake:import
)
```

The `:go:*` and `:bump` targets are then available in that project only.

## License

Released under the MIT License — see [LICENSE.md](LICENSE.md).
