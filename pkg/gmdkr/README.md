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
injects build metadata (build date, commit, repo, CI id) as `--build-arg`s. It
supports both single-image projects and multi-stage projects that build several
named target images at once. The exported types (`DockerCmd`, `Config`,
`Build`, `DkrImages`) are also usable as a plain library.

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
gomake :docker:image:build -l         # also tag it :latest
gomake :docker:image:build -r         # rebuild with --no-cache
gomake :docker:image:build -d         # print the docker build, run nothing
```

For a project name `app` with no private registry this builds `dki-app:<rev>`,
where `<rev>` comes from `git describe`. When the config lists targets
(`C42_BLD_TARGETS`), one image is built per target
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
gomake :docker:image:push -d          # print the docker push commands only
```

### Inspecting the image metadata

`gmdkr` exposes the names, tags, and references it derives so other tooling can
consume them.

```shell
gomake :docker:image:reference        # e.g. my.nexus.dev/repo/dki-app:v1.2.3
gomake :docker:image:env              # C42_DKI_* variables, one KEY=value per line
gomake :docker:image:env -e           # same, prefixed with "export "
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
C42_BLD_TARGETS=api,worker,migrate   # Dockerfile stages to build (optional)
```

| Key               | Meaning                                                        | Required |
|-------------------|----------------------------------------------------------------|----------|
| `C42_REG_HOST`    | Private registry host.                                         | push     |
| `C42_REG_REPO`    | Private repository the image is built from and pushed to.      | push     |
| `C42_BLD_TARGETS` | Comma-separated `Dockerfile` stage names to build as images.   | no       |

`C42_REG_HOST` and `C42_REG_REPO` together mark the remote as configured;
`:push` and `:login` need both. `C42_BLD_TARGETS` switches a project from a
single image to one image per listed stage — each stage must exist in the
`Dockerfile`.

### Image naming

The image name is derived from the project name with a `dki-` prefix (added
unless the name already contains `dki-`), optionally qualified by the private
repository and the target stage, and tagged with the SCM revision:

```text
<repo>/dki-<project>-<target>:<tag>
└──────────────┬───────────────┘ └┬┘
        image name               git describe (override with -t)
```

Examples: `dki-app:v1.2.3`, `my.nexus.dev/repo/dki-app:v1.2.3`,
`my.nexus.dev/repo/dki-app-api:v1.2.3`. With `-l` the image is additionally
tagged `:latest`.

### Build arguments

Every build passes the project's metadata to the `Dockerfile` as
`--build-arg`s, so a stage can `ARG` and consume them:

| Build arg        | Value                                            |
|------------------|--------------------------------------------------|
| `C42_BUILD_DATE` | RFC-3339 build date.                             |
| `C42_SCM_REV`    | SCM revision (`git describe`).                   |
| `C42_SCM_HASH`   | Commit hash.                                     |
| `C42_SCM_REPO`   | Remote repository URL.                           |
| `C42_CCID`       | CI/CD job id, or `unknown`.                      |
| `C42_REG_HOST`   | Registry host, when configured.                  |
| `C42_REG_REPO`   | Private repository, when configured.             |
| `SSH_AUTH_SOCK`  | SSH agent socket, when set in the environment.   |

Any additional keys present in `configs/project.conf` are forwarded as
`--build-arg`s as well.

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

| Flag        | Short | Targets                         | Meaning                                |
|-------------|-------|---------------------------------|----------------------------------------|
| `--targets` | `-T`  | build, push, run, sh, reference | Comma-separated targets to act on.     |
| `--name`    | `-n`  | build, push, run, sh            | Override the derived image name.       |
| `--tag`     | `-t`  | build, push, run, sh            | Override the derived image tag.        |
| `--latest`  | `-l`  | build, run                      | Also tag the image `:latest`.          |
| `--rebuild` | `-r`  | build, run                      | Force a rebuild (build: `--no-cache`). |
| `--cmd`     | `-c`  | sh                              | Command to run inside the container.   |
| `--export`  | `-e`  | env                             | Prefix each line with `export`.        |
| `--dry-run` | `-d`  | build, push, run, sh            | Print the `docker` command only.       |
| `--help`    | `-h`  | all                             | Show the target's help.                |
