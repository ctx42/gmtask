# gmtask

[![Go](https://github.com/ctx42/gmtask/actions/workflows/test.yml/badge.svg)](https://github.com/ctx42/gmtask/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ctx42/gmtask.svg)](https://pkg.go.dev/github.com/ctx42/gmtask)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ctx42/gmtask)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE.md)

Reusable [gomake](https://github.com/ctx42/gomake) targets and supporting
libraries for Go projects.

## Overview

`gmtask` is a collection of Go packages that standardize routine project work.
Its targets compile into the gomake binary as built-in commands — available in
every repo with no per-project `makefile.go` — while its libraries are
standalone Go packages you can import directly.

## Packages

| Package                            | Description                          |
|------------------------------------|--------------------------------------|
| [`pkg/gmgo`](pkg/gmgo)             | gomake targets for the Go workflow.  |
| [`pkg/gmbump`](pkg/gmbump)         | The `:bump` release target.          |
| [`pkg/lib/gmclog`](pkg/lib/gmclog) | Read/edit/write Markdown changelogs. |

## Gomake targets

The [`gmgo`](pkg/gmgo) package provides these targets, run as
`gomake :go:<name>` once it is compiled into the binary:

| Target             | Description                                     |
|--------------------|-------------------------------------------------|
| `:go:vet`          | Runs `go vet ./...`.                            |
| `:go:lint`         | Runs golangci-lint (auto-installs it + config). |
| `:go:lint:install` | Installs the latest golangci-lint.              |
| `:go:lint:config`  | Downloads the shared `.golangci.yml`.           |
| `:go:test`         | Tests with race detector and coverage.          |
| `:go:test-v`       | Same as `:go:test`, verbose.                    |
| `:go:check`        | Runs vet, lint, and test in order.              |
| `:go:build`        | Builds with version metadata via `-ldflags`.    |
| `:go:doc`          | Serves godoc and opens the browser.             |

See the [`gmgo` README](pkg/gmgo) for flags, environment variables, and the
library helpers.

The [`gmbump`](pkg/gmbump) package provides the release target:

| Target  | Description                                          |
|---------|------------------------------------------------------|
| `:bump` | Tags the next version, writes the changelog, pushes. |

See the [`gmbump` README](pkg/gmbump) for the interactive flow and flags.

## Installation

Add `gmgo` to the `targets.yaml` at your gomake source root, then rebuild the
binary so the targets are compiled in:

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

## License

Released under the MIT License — see [LICENSE.md](LICENSE.md).
