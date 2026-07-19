# gmgo

Ready-made [gomake](https://github.com/ctx42/gomake) targets for Go projects —
vet, lint, test, build, and serve docs — shared across every repo from one
binary.

<!-- TOC -->
* [gmgo](#gmgo)
  * [Overview](#overview)
  * [Features](#features)
  * [Prerequisites](#prerequisites)
  * [Installation](#installation)
    * [As built-in gomake targets](#as-built-in-gomake-targets)
    * [As per-project targets](#as-per-project-targets)
  * [Usage](#usage)
    * [Building with version metadata](#building-with-version-metadata)
    * [Test reports](#test-reports)
    * [Linting config](#linting-config)
    * [Use as a library](#use-as-a-library)
  * [Configuration](#configuration)
    * [gomake.yaml](#gomakeyaml)
    * [Environment variables](#environment-variables)
<!-- TOC -->

## Overview

`gmgo` packages the routine Go development commands as gomake targets. Instead
of copying a `Makefile` or shell scripts into each repo, you compile `gmgo` into
the gomake binary once (as [built-in targets](https://github.com/ctx42/gomake))
and run `gomake :go:test`, `gomake :go:lint`, `gomake :go:build`, and friends in
any project — no per-project `makefile.go` required.

It is meant for teams and individuals who want a single, versioned source of
truth for their Go workflow: fix a target once, `go get -u`, and every project
picks up the change. The exported helpers (import-path resolution, ldflags
assembly, log-file naming) are also usable as a plain library.

## Features

- **`:go:vet`** — runs `go vet ./...`.
- **`:go:lint`** — runs golangci-lint, auto-installing it and the config.
- **`:go:test`** — tests with race and coverage.
- **`:go:test-v`** — verbose tests with race and coverage.
- **`:go:check`** — runs `:go:vet`, `:go:lint`, and `:go:test` in order.
- **`:go:build`** — runs `go build`, injecting metadata via `-ldflags`.
- **`:go:doc`** — serves godoc and opens the current package in your browser.
- **`:go:pkgsite`** — serves pkgsite and opens the current package.
- **Library helpers** — `ImpPath`, `InitModule`, `LDFlags`, and more.

## Prerequisites

- [gomake](https://github.com/ctx42/gomake) — the binary that hosts the targets.
- `git`.
- `golangci-lint` `v2.12.2` or newer (auto-installed if missing).
- A browser opener (`xdg-open` on Linux, `open` on macOS).
- `godoc`.
- `pkgsite`.

## Installation

### As built-in gomake targets

Add `gmgo` to the `targets.yaml` at your gomake source root:

```yaml
imports:
  - import: github.com/ctx42/gmtask/pkg/gmgo
```

Then rebuild the gomake binary so the targets are compiled in:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

The `:go:*` targets are now available in every project the binary is used from.

### As per-project targets

Pull `gmgo` into one project without rebuilding the binary. Add it to the
project's module, then import it in `makefile.go` with a `//gomake:import`
comment (the blank identifier is required):

```shell
go get github.com/ctx42/gmtask/pkg/gmgo
```

```go
//go:build gomake

package main

import (
	_ "github.com/ctx42/gmtask/pkg/gmgo" //gomake:import
)
```

The `:go:*` targets are now available in that project only.

## Usage

Run any target from a project's root directory:

```shell
gomake :go:vet          # go vet ./...
gomake :go:test         # tests with race detector + coverage
gomake :go:test-v       # same, verbose
gomake :go:lint         # golangci-lint run
gomake :go:check        # vet, then lint, then test
gomake :go:build        # go build with version metadata
gomake :go:doc          # serve godoc and open the browser
gomake :go:pkgsite      # serve pkgsite and open the browser
```

### Building with version metadata

`:go:build` forwards its arguments to `go build`. When the current module is
configured it injects build metadata into a package of your choice via
`-ldflags -X`; with no matching configuration it builds normally and injects
nothing.

```shell
gomake :go:build cmd/main.go
```

Configure injection in a `gomake.yaml` (user- or project-level), keyed by the
module import path reported by `go list -m`:

```yaml
version: 1
targets:
  github.com/ctx42/gmtask/pkg/gmgo:
    go:
      build:
        modules:
          github.com/acme/app:
            package: github.com/acme/app/internal/version
            names:            # optional; omitted fields keep the defaults
              scmRev: Version
              ccid: CIJob
```

`package` (required) is the import path whose variables receive the values.
Declare a package-level `string` in it for each field, named as below:

| Field        | Default name | Value                              |
|--------------|--------------|------------------------------------|
| build date   | `buildDate`  | `OCI_IMAGE_CREATED` env, else now  |
| SCM revision | `scmRev`     | `git describe`, else `v0.0.0`      |
| SCM hash     | `scmHash`    | latest commit hash, else `0000000` |
| SCM state    | `scmState`   | `clean`/`dirty`, else `unknown`    |
| CI/CD id     | `ccid`       | `GOMAKE_CCID` env, else `unknown`  |

`names` overrides the variable name for any field; the defaults match the names
gomake injects into its own binary. A module with no entry is built without
injection.

### Test reports

`:go:test` and `:go:test-v` write two files, in `tmp/` when that directory
exists, otherwise the current directory:

- `go_test_coverage.log` — the coverage profile.
- `go_test_run.log` — a copy of the `go test` output.

In CI, when `BUILD_ID` is set, it is appended to each name (e.g.
`go_test_coverage_123.log`). Override the destination with `-dir`, and pass
extra flags straight to `go test` after `--`:

```shell
gomake :go:test -dir build/reports -- -run TestFoo
```

Set a `go test -timeout` in `gomake.yaml` or via `GOMAKE_GO_TEST_TIMEOUT` — see
[Configuration](#configuration).

### Linting config

`:go:lint` installs golangci-lint if it is missing or below `v2.12.2`, then
downloads a shared `.golangci.yml` before running. Manage those steps directly
with the sub-targets:

```shell
gomake :go:lint:install         # install the latest golangci-lint
gomake :go:lint:config          # download the shared .golangci.yml
gomake :go:lint:config -dir tmp # to a specific directory
```

Pin the golangci-lint version and the config file name in `gomake.yaml` — see
[Configuration](#configuration).

The config is fetched with a shallow clone of a git repository (the `master`
branch's `.golangci.yml`), defaulting to `git@github.com:ctx42/xdev.git`. Point
it at your own repository with `GOMAKE_GOLINT_CONFIG_REPO` — any remote `git`
can clone, SSH or HTTPS:

```shell
GOMAKE_GOLINT_CONFIG_REPO=git@github.com:acme/dev.git gomake :go:lint:config
GOMAKE_GOLINT_CONFIG_REPO=https://github.com/acme/dev.git gomake :go:lint
```

Add `GOMAKE_GOLINT_CONFIG_FORCE=1` to re-download over an existing file.

### Use as a library

Add the module to your project:

```shell
go get github.com/ctx42/gmtask/pkg/gmgo
```

The helpers are ordinary functions — useful when writing your own targets or
tooling:

```go
import (
	"github.com/ctx42/gmtask/pkg/gmgo"
	"github.com/ctx42/ring/pkg/ring"
)

// Resolve the module import path of the current directory.
rng := ring.New(ring.WithEnv(os.Environ()))
path, err := gmgo.ImpPath(ctx, rng, "")

// Format the -ldflags string for a package.
flags := gmgo.LDFlags("example.com/app/version", []gmgo.LDVar{
	{Name: "scmRev", Value: "v1.2.0"},
	{Name: "scmHash", Value: "ab12cd"},
})

// CI-aware report filenames (rng is the *ring.Ring the target receives).
cov := gmgo.CovLogFilename(rng)  // go_test_coverage.log (+ BUILD_ID in CI)
run := gmgo.TestLogFilename(rng) // go_test_run.log      (+ BUILD_ID in CI)
```

## Configuration

`gmgo` reads settings from two places: a `gomake.yaml` `targets:` block and
environment variables. A setting offered in both is overridden by the
environment variable.

### gomake.yaml

The `targets:` section is a nested tree that mirrors the target names — keyed by
import path, then namespace, then target, all lower-case and kebab-cased. A
setting placed on a namespace node is shared by that namespace's targets, so it
is written once, not repeated: `version` and `file` sit on `lint` (shared by
`:go:lint`, `:go:lint:install`, and `:go:lint:config`); `timeout` sits on `go`
(shared by `:go:test`, `:go:test-v`, and `:go:check`).

```yaml
version: 1
targets:
  github.com/ctx42/gmtask/pkg/gmgo:
    go:
      timeout: 5m           # go test -timeout; default: the go tool default.
      lint:
        version: v2.12.2    # golangci-lint version to install; default: latest.
        file: .golangci.yml # shared config file name; default: .golangci.yml.
        repo: git@github.com:acme/dev.git # config source; default: ctx42/xdev.
```

The `file` key names the config; the `repo` key sets the git repository it is
fetched from (a shallow clone of the `master` branch, any SSH or HTTPS remote).
`repo` is also settable per run with the `GOMAKE_GOLINT_CONFIG_REPO` environment
variable, which takes precedence:

```shell
GOMAKE_GOLINT_CONFIG_REPO=git@github.com:acme/dev.git gomake :go:lint
```

A target receives the block from the nearest level on its path — so `:go:build`
injection is configured under `go.build` and never sees `timeout`. See
[Building with version metadata](#building-with-version-metadata). A full,
annotated example is in [`gomake.example.yaml`](gomake.example.yaml).

`:go:check` runs its lint step with lint defaults: it is delivered the `go`
node's block (`timeout` only), so `go.lint`'s `version`, `file`, and `repo` are
not applied during a check. Run `:go:lint`, `:go:lint:install`, or
`:go:lint:config` directly to exercise lint configuration.

| Node   | Key       | Meaning                              | Default                         |
|--------|-----------|--------------------------------------|---------------------------------|
| `lint` | `version` | golangci-lint version installed      | `latest`                        |
| `lint` | `file`    | shared config file fetched + written | `.golangci.yml`                 |
| `lint` | `repo`    | git repo the config is fetched from  | `git@github.com:ctx42/xdev.git` |
| `go`   | `timeout` | `go test -timeout` value             | go tool default                 |

### Environment variables

- **`GOMAKE_GO_TEST_TIMEOUT`** (`:go:test`) — `go test -timeout` value;
  overrides the `timeout` configuration above.
- **`GOMAKE_GOLINT_CONFIG_FORCE`** (`:go:lint:config`) — re-download the config
  file even if it already exists.
- **`GOMAKE_GOLINT_CONFIG_REPO`** (`:go:lint:config`) — source repo for the
  config; overrides the `repo` configuration above (default
  `git@github.com:ctx42/xdev.git`).
- **`BUILD_ID`** (`:go:test`) — appended to the coverage and run log filenames
  in CI.
- **`GOMAKE_CCID`** (`:go:build`) — CI/CD job id injected as the `ccid` value.
- **`OCI_IMAGE_CREATED`** (`:go:build`) — RFC-3339 build date injected as
  `buildDate`; the current time is used when unset.
