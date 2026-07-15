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
Its targets are reusable gomake targets — how you wire them in is up to you:
compile them into your gomake binary as built-in commands available in every
project, or import them per-project from a `makefile.go`
(see [Installation](#installation)). Its libraries are standalone Go packages
you can import directly.

## Targets

| Target                   | Description                                          |
|--------------------------|------------------------------------------------------|
| `go:vet`                 | Runs `go vet ./...`.                                 |
| `go:lint`                | Runs golangci-lint (auto-installs it + config).      |
| `go:lint:install`        | Installs the latest golangci-lint.                   |
| `go:lint:config`         | Downloads the shared `.golangci.yml`.                |
| `go:test`                | Tests with race detector and coverage.               |
| `go:test-v`              | Same as `:go:test`, verbose.                         |
| `go:check`               | Runs vet, lint, and test in order.                   |
| `go:build`               | Builds with version metadata via `-ldflags`.         |
| `go:doc`                 | Serves godoc and opens the browser.                  |
| `docker:image:build`     | Builds the project image(s) from the `Dockerfile`.   |
| `docker:image:run`       | Builds if needed, then runs the image.               |
| `docker:image:sh`        | Builds if needed, then opens a shell in the image.   |
| `docker:image:push`      | Pushes the image(s) to the private registry.         |
| `docker:image:reference` | Prints the fully-qualified image reference.          |
| `docker:image:env`       | Prints the derived image variables as `KEY=value`.   |
| `docker:image:info`      | Prints the same variables in a readable form.        |
| `docker:image:clean`     | Removes dangling and stale test images.              |
| `docker:login`           | Logs in to the configured private registry.          |
| `doc:mce`                | Inject Go example bodies into Markdown docs.         |
| `bump`                   | Tags the next version, writes the changelog, pushes. |
| `project:setup`          | Scaffold a new Go project and init module + git.     |
| `project:env`            | Print project info as `KEY=value` env variables.     |
| `project:info`           | Print the same information in a readable form.       |

See the [`gmgo`](pkg/gmgo), [`gmdkr`](pkg/gmdkr), [`gmbump`](pkg/gmbump),
[`gmprj`](pkg/gmprj), and [`gmmce`](pkg/gmmce) READMEs for flags, environment
variables, the interactive flow, and library helpers.

## Libraries

| Package                            | Description                          |
|------------------------------------|--------------------------------------|
| [`pkg/lib/gmclog`](pkg/lib/gmclog) | Read/edit/write Markdown changelogs. |

## Installation

### As built-in gomake targets

Add the packages to the `targets.yaml` at your gomake source root, then
rebuild the binary so the targets are compiled in:

```yaml
imports:
  - import: github.com/ctx42/gmtask/pkg/gmgo
  - import: github.com/ctx42/gmtask/pkg/gmdkr
  - import: github.com/ctx42/gmtask/pkg/gmbump
  - import: github.com/ctx42/gmtask/pkg/gmprj
  - import: github.com/ctx42/gmtask/pkg/gmmce
```

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

The `:go:*`, `:docker:*`, `:bump`, `:project:*`, and `:doc:mce` targets are
then available in every project the binary is used from.

### As per-project targets

Pull the packages into one project without rebuilding the binary. Add them to
the project's module, then import them in `makefile.go` with `//gomake:import`
comments (the blank identifier is required):

```shell
go get github.com/ctx42/gmtask/pkg/gmgo github.com/ctx42/gmtask/pkg/gmdkr github.com/ctx42/gmtask/pkg/gmbump github.com/ctx42/gmtask/pkg/gmprj github.com/ctx42/gmtask/pkg/gmmce
```

```go
//go:build gomake

package main

import (
	_ "github.com/ctx42/gmtask/pkg/gmgo"   //gomake:import
	_ "github.com/ctx42/gmtask/pkg/gmdkr"  //gomake:import
	_ "github.com/ctx42/gmtask/pkg/gmbump" //gomake:import
	_ "github.com/ctx42/gmtask/pkg/gmprj"  //gomake:import
	_ "github.com/ctx42/gmtask/pkg/gmmce"  //gomake:import
)
```

The `:go:*`, `:docker:*`, `:bump`, `:project:*`, and `:doc:mce` targets are
then available in that project only.

## License

Released under the MIT License — see [LICENSE.md](LICENSE.md).
