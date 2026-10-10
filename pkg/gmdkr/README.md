# gmdkr

The [gomake](https://github.com/ctx42/gomake) `:docker:*` targets — build, run,
shell into, publish, and inspect a project's Docker images from one binary.

<!-- TOC -->
* [gmdkr](#gmdkr)
  * [Overview](#overview)
  * [Features](#features)
  * [Prerequisites](#prerequisites)
  * [Installation](#installation)
    * [As built-in gomake targets](#as-built-in-gomake-targets)
    * [As per-project targets](#as-per-project-targets)
  * [Usage](#usage)
    * [Building images](#building-images)
    * [Running and shelling in](#running-and-shelling-in)
    * [Pushing to a private registry](#pushing-to-a-private-registry)
    * [Inspecting the image metadata](#inspecting-the-image-metadata)
    * [Cleaning up](#cleaning-up)
    * [Use as a library](#use-as-a-library)
  * [Configuration](#configuration)
    * [Project configuration](#project-configuration)
    * [Image naming](#image-naming)
    * [Build arguments](#build-arguments)
    * [Environment variables](#environment-variables)
  * [Flags](#flags)
<!-- TOC -->

## Overview

`gmdkr` packages a project's Docker workflow as gomake targets. Instead of
copying `docker build` invocations and image-naming conventions into each repo,
you compile `gmdkr` into the gomake binary once and run `gomake :docker:image:build`,
`gomake :docker:image:push`, `gomake :docker:image:run`, and friends in any
project — no per-project `makefile.go` required.

Given a `Dockerfile` and a project configuration file, `gmdkr` derives the image
name from the project name, the tag from the SCM revision (`git describe`), and
injects build metadata (build date, commit, repo, state) as `--build-arg`s. It
supports both single-image projects and multi-stage projects that build several
named target images at once. The exported types (`DockerCmd`, `Config`,
`Build`, `ImageInfos`) are also usable as a plain library.

## Features

- **`:docker:image:build`** — build one image, or every configured target image.
- **`:docker:image:run`** — build if needed, then `docker run --rm` the image.
- **`:docker:image:sh`** — build if needed, then open a shell in the image.
- **`:docker:image:push`** — push the image(s) to the private registry.
- **`:docker:image:reference`** — print the fully-qualified image reference.
- **`:docker:image:env`** — print the derived image variables in `env` format.
- **`:docker:image:info`** — the same variables, human-readable.
- **`:docker:image:clean`** — remove dangling and stale test images.
- **`:docker:login`** — log in to the configured private registry.
- **BuildKit + SSH** — builds with BuildKit and forwards `SSH_AUTH_SOCK` as
  `--ssh default` when set.
- **Multi-target** — one command builds/pushes every stage listed in the config.

## Prerequisites

- [gomake](https://github.com/ctx42/gomake) — the binary that hosts the targets.
- `docker` with BuildKit — used for every build, run, and push.
- `git` — used to derive the image tag and SCM build metadata.
- A `Dockerfile` in the project root.
- A project configuration file at `configs/project.conf` (see
  [Configuration](#configuration)).

## Installation

### As built-in gomake targets

Add `gmdkr` to the `targets.yaml` at your gomake source root:

```yaml
imports:
  - import: github.com/ctx42/gmtask/pkg/gmdkr  # :docker:* targets.
```

Then rebuild the gomake binary so the targets are compiled in:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

The `:docker:*` targets are now available in every project the binary is used
from.

### As per-project targets

Pull `gmdkr` into one project without rebuilding the binary. Add it to the
project's module, then import it in `makefile.go` with a `//gomake:import`
comment (the blank identifier is required):

```shell
go get github.com/ctx42/gmtask/pkg/gmdkr
```

```go
//go:build gomake

package main

import (
	_ "github.com/ctx42/gmtask/pkg/gmdkr" //gomake:import
)
```

The `:docker:*` targets are now available in that project only.

## Usage

Run any target from the project root. Every build, run, and push target accepts
`--dry-run` (`-d`) to print the `docker` command without executing it.

### Building images

```shell
gomake :docker:image:build            # build the project image
gomake :docker:image:build -l         # also tag a release :latest
gomake :docker:image:build -r         # rebuild with --no-cache
gomake :docker:image:build -d         # print the docker build, run nothing
```

In CI, build and push in one step and keep the layer cache outside the
builder. `--cache-from` and `--cache-to` take any `docker build` cache
specification and pass it on as given, except that `{image}` is replaced with
the image name without the registry (`dki-app-api`), so the targets of a
multi-target build keep separate caches. Exporting a cache needs a buildx
builder that supports it, and `--push` needs the private registry configured
(see [Pushing](#pushing-to-a-private-registry)):

```shell
gomake :docker:image:build -l --push \
    --cache-from 'type=registry,ref=my.nexus.dev/cache:{image}' \
    --cache-to 'type=registry,ref=my.nexus.dev/cache:{image},mode=max'
```

For a project name `app` with no private registry this builds `dki-app:<rev>`,
where `<rev>` comes from `git describe`. When the config lists targets
(`C42_BLD_IMG_TARGETS`), one image is built per target
(`dki-app-api`, `dki-app-worker`, …); restrict the set with `-T`:

```shell
gomake :docker:image:build -T api,worker
```

Override the derived name and tag when needed:

```shell
gomake :docker:image:build -n custom-name -t v9.9.9
```

### Running and shelling in

`:run` and `:sh` build the image first if it is not already present (use `-r` to
force a rebuild), then run it with the project mounted read-only at
`/ctx42/project`.

```shell
gomake :docker:image:run              # run the single image
gomake :docker:image:run -T api       # run one target of a multi-target project
gomake :docker:image:run -- arg1 arg2 # pass arguments to the container
gomake :docker:image:sh               # open a shell (default: /bin/sh --login)
gomake :docker:image:sh -c /bin/bash  # choose the shell/command
```

A multi-target project has no single default image, so `:run` and `:sh` require
`-T` to pick exactly one target.

### Pushing to a private registry

Pushing requires both `C42_REG_HOST` and `C42_REG_REPO` in the project config
(see [Configuration](#configuration)); otherwise the target returns
`ErrNoPrvRepo`.

```shell
gomake :docker:login                  # authenticate to the registry
gomake :docker:image:push             # push the image(s)
gomake :docker:image:push -l          # also move a release's :latest
gomake :docker:image:push -d          # print the docker push commands only
```

### Inspecting the image metadata

`gmdkr` exposes the names, tags, and references it derives so other tooling can
consume them.

```shell
gomake :docker:image:reference        # e.g. my.nexus.dev/repo/dki-app:v1.2.3
gomake :docker:image:env              # C42_DKI_* variables, one KEY=value per line
gomake :docker:image:env -e           # same, as export KEY='value' lines
gomake :docker:image:env C42_DKI_REF  # print just one variable's value
gomake :docker:image:info             # the same variables, human-readable
```

For a multi-target project, `:reference` needs `-T` to pick one target.

### Cleaning up

```shell
gomake :docker:image:clean            # remove dangling + stale test images
```

`:clean` removes dangling images, and images whose repository reference contains
`ctx42-tst-img-` that were created more than an hour ago.

### Use as a library

Add the module to your project:

```shell
go get github.com/ctx42/gmtask/pkg/gmdkr
```

`DockerCmd` is the entry point: construct it from a set of [`Flags`](flags.go),
`Init` it against the project directory, then build, run, or inspect.

```go
import (
	"context"
	"os"

	"github.com/ctx42/gmtask/pkg/gmdkr"
	"github.com/ctx42/ring/pkg/ring"
)

rng := ring.New(ring.WithEnv(os.Environ()))

fls := gmdkr.NewFlags(":docker:image:build")
fls.ImgLatest = true

dc := gmdkr.NewDockerCmd(fls)
if err := dc.Init(context.Background(), rng.EnvAll(), "."); err != nil {
	// no Dockerfile, unknown target, missing config, ...
}

ref, _ := dc.Reference()          // "dki-app:v1.2.3"
err := dc.Build(context.Background(), rng)
```

To assemble a `docker build` command from project information without running
it, drop to `Config` and `Build`:

```go
inf, _ := gmprj.GetInfo(ctx, rng.EnvAll(), ".")
bld, _ := gmdkr.NewBuild(*gmdkr.ConfigFrom(inf, nil))
fmt.Println(bld.String()) // DOCKER_BUILDKIT=1 docker build --platform ... .
```

## Configuration

### Project configuration

`gmdkr` reads the project's configuration file at `configs/project.conf`, an
`env`-style file of `KEY=value` lines:

```ini
# configs/project.conf
C42_REG_HOST=my.nexus.dev            # registry host
C42_REG_REPO=my.nexus.dev/repo       # private repository the image lives in
C42_BLD_IMG_TARGETS=api,worker,migrate  # Dockerfile stages to build (optional)
```

| Key                   | Meaning                              | Required    |
|-----------------------|--------------------------------------|-------------|
| `C42_REG_HOST`        | Private registry host.               | push, login |
| `C42_REG_REPO`        | Private repo to build from, push to. | push        |
| `C42_BLD_IMG_TARGETS` | Comma-separated `Dockerfile` stages. | no          |

`C42_REG_HOST` and `C42_REG_REPO` together mark the remote as configured;
`:push` needs both, and `:login` logs in to `C42_REG_HOST`.
`C42_BLD_IMG_TARGETS` switches a project from a single image to one image per
listed stage — each stage must exist in the `Dockerfile`.

An environment variable named like a key in the file overrides that key's
value, an empty value included; variables the file does not declare are
ignored. Overrides apply after `$VAR` expansion, so overriding `C42_REG_HOST`
leaves a `C42_REG_REPO=$C42_REG_HOST/...` value unchanged. `:docker:login`,
`:docker:image:build`, `:docker:image:push`, `:docker:image:run`, and
`:docker:image:sh` print one `override KEY=value` line per overridden key to
standard error:

```shell
C42_REG_HOST=registry.acme.io C42_REG_REPO=registry.acme.io/platform \
    gomake :docker:image:push
```

### Image naming

The image name is derived from the project name with a `dki-` prefix (added
unless the name already contains `dki-`), optionally qualified by the private
repository and the target stage, and tagged with the derived version:

```text
<repo>/dki-<project>-<target>:<tag>
└──────────────┬───────────────┘ └┬┘
        image name               derived version (override -t)
```

The version comes from `gitaid.Derive`, which yields the bare release tag only
when `HEAD` is a clean checkout of a semver tag, and a pre-release of the next
release otherwise. A Docker tag cannot hold the `+` that opens SemVer
build metadata, so the tag is the version with `+` replaced by `_`; the
untouched version stays in `C42_SCM_REV` and the version label.

Examples: `dki-app:v1.2.3`, `my.nexus.dev/repo/dki-app:v1.2.3`,
`my.nexus.dev/repo/dki-app-api:v1.2.4-dev.3_ga2f04ae`. With `-l` the image is
additionally tagged `:latest` — but only a release is tagged that way, so a
dirty tree, a commit past the tag, or a pre-release tag such as `v1.0.0-rc.1`
never moves `latest`.

### Build arguments

Every build passes the project's metadata to the `Dockerfile` as
`--build-arg`s, so a stage can `ARG` and consume them:

| Build arg       | Value                                          |
|-----------------|------------------------------------------------|
| `C42_BLD_DATE`  | RFC-3339 build date, millisecond precision.    |
| `C42_PRJ_NAME`  | Project name.                                  |
| `C42_SCM_REV`   | Version derived by `gitaid`.                   |
| `C42_SCM_HASH`  | Commit hash.                                   |
| `C42_SCM_REPO`  | Remote repository URL.                         |
| `C42_REG_HOST`  | Registry host, when configured.                |
| `C42_REG_REPO`  | Private repository, when configured.           |
| `SSH_AUTH_SOCK` | SSH agent socket, when set in the environment. |

Any additional keys present in `configs/project.conf` are forwarded as
`--build-arg`s as well. `C42_SCM_STATE` is deliberately not passed: the
`.dirty` identifier in `C42_SCM_REV` already carries the tree state, and two
sources for one fact drift.

### Environment variables

`gmdkr` reads `SSH_AUTH_SOCK` from the environment; when set, the build runs
with `--ssh default=$SSH_AUTH_SOCK` so a stage can reach private Git remotes.

The `:env` and `:info` targets expose the variables `gmdkr` derives for the
current project:

| Variable            | Set when            | Value                                  |
|---------------------|---------------------|----------------------------------------|
| `C42_DKI_NAME`      | single image        | Image name.                            |
| `C42_DKI_TAG`       | always              | Image tag.                             |
| `C42_DKI_REF`       | single image        | Full image reference.                  |
| `C42_DKI_NAME_STEM` | multiple targets    | Image name without the target suffix.  |
| `C42_DKI_NAMES`     | multiple targets    | Comma-separated image names.           |
| `C42_DKI_REFS`      | multiple targets    | Comma-separated image references.      |

## Flags

Targets take only the flags relevant to them; all support `-h`/`--help`.

| Flag           | Short | Targets                         | Meaning                                |
|----------------|-------|---------------------------------|----------------------------------------|
| `--targets`    | `-T`  | build, push, run, sh, reference | Comma-separated targets to act on.     |
| `--name`       | `-n`  | build, push, run, sh            | Override the derived image name.       |
| `--tag`        | `-t`  | build, push, run, sh            | Override the derived image tag.        |
| `--latest`     | `-l`  | build, push                     | Also tag and push a release `:latest`. |
| `--rebuild`    | `-r`  | build, run                      | Force a rebuild (build: `--no-cache`). |
| `--push`       | `-p`  | build                           | Push while building (`--push`).        |
| `--cache-from` |       | build                           | `docker build --cache-from` spec.      |
| `--cache-to`   |       | build                           | `docker build --cache-to` spec.        |
| `--cmd`        | `-c`  | sh                              | Command to run; shell quoting applies. |
| `--export`     | `-e`  | env                             | Print `export KEY='value'` lines.      |
| `--dry-run`    | `-d`  | build, push, run, sh            | Print the `docker` command only.       |
| `--help`       | `-h`  | all                             | Show the target's help.                |

`--latest` is a request, not a guarantee: only a release — a clean checkout
sitting exactly on a semver tag that is not a pre-release — is tagged
`:latest`, so a dirty tree, a commit past the tag, or a tag such as
`v1.0.0-rc.1` never moves it. `:push -l` tags `:latest` from the image it
pushes before pushing it, so the registry's `latest` is always that version.
