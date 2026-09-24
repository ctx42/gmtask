# gmbump

The [gomake](https://github.com/ctx42/gomake) `:bump` target — tag the next
release, write its changelog, and push, in one command.

<!-- TOC -->
* [gmbump](#gmbump)
  * [Overview](#overview)
  * [Features](#features)
  * [Prerequisites](#prerequisites)
  * [Installation](#installation)
    * [As a built-in gomake target](#as-a-built-in-gomake-target)
    * [As a per-project target](#as-a-per-project-target)
  * [Usage](#usage)
    * [Use as a library](#use-as-a-library)
<!-- TOC -->

## Overview

`gmbump` releases a Go project in a single step. It reads the repository's
latest semantic-version tag, proposes the next version, and — once you confirm
it — records the commits since that tag as a new `CHANGELOG.md` entry, writes
the version to a `VER` file, commits the two files, tags the commit, and pushes
the tag to origin.

It is a gomake target: compile it into the gomake binary once and run
`gomake :bump` in any repository, with no per-project `makefile.go`. The
exported `Bump` and `BumpTarget` functions are also usable as a plain library.

## Features

- **Automatic version proposal** — the release the repository is already
  heading towards, read off the Conventional Commits since the last tag.
- **Interactive confirmation** — accept the proposal or type your own version.
- **Changelog generation** — a dated release from commits, ready to edit.
- **Clean-tree guard** — refuses to run when the working tree is dirty.
- **Branch guard** — refuses a detached HEAD, and asks before releasing from
  a branch other than `master` or `main`.
- **Skips non-semver tags** — a `nightly` or a date stamp is never mistaken
  for a release, and the one passed over is named.
- **One version everywhere** — the tag it cuts is the release `gitaid.Derive`
  stamps into development builds, so the two can never disagree.
- **Go-module aware** — prints a `go get module@version` upgrade hint.

## Prerequisites

- [gomake](https://github.com/ctx42/gomake) — the binary that hosts the target.
- `git` — used to read tags and to commit, tag, and push the release.

## Installation

### As a built-in gomake target

Add `gmbump` to the `targets.yaml` at your gomake source root:

```yaml
imports:
  - import: github.com/ctx42/gmtask/pkg/gmbump
```

Then rebuild the gomake binary so the target is compiled in:

```shell
go run github.com/ctx42/gomake/cmd/install@latest --targets=./targets.yaml
```

The `:bump` target is now available in every project the binary is used from.

### As a per-project target

Pull `gmbump` into one project without rebuilding the binary. Add it to the
project's module, then import it in `makefile.go` with a `//gomake:import`
comment (the blank identifier is required):

```shell
go get github.com/ctx42/gmtask/pkg/gmbump
```

```go
//go:build gomake

package main

import (
	_ "github.com/ctx42/gmtask/pkg/gmbump" //gomake:import
)
```

`:bump` is now available in that project only. Add a namespace prefix — e.g.
`//gomake:import release` — to expose it as `:release:bump` instead.

## Usage

Run from the repository root:

```shell
gomake :bump        # propose the release the commits imply
gomake :bump -p     # force a patch bump instead
gomake :bump -h     # show help
```

A run:

1. Verifies the working tree is clean; aborts otherwise.
2. Reads the current branch; aborts on a detached HEAD.
3. Prints the current tag, and stops here when there is nothing to release.
4. On a branch other than `master` or `main`, asks to confirm the release —
   only `y` or `yes` goes on, anything else aborts.
5. Prompts for the next version, pre-filled with the proposal — press ENTER to
   accept it or type your own.
6. Prepends the release to `CHANGELOG.md` and pauses so you can edit it; press
   ENTER to continue.
7. Writes `VER`, commits `CHANGELOG.md` and `VER`, tags the commit, and pushes
   the tag to origin (a missing remote is not an error).

A `v0.1.0` repository with a `feat:` commit since the tag looks like:

```text
Current tag: v0.1.0
Enter a version number [v0.2.0]:
Now you may edit CHANGELOG.md. Then press ENTER to continue.
Continuing.
No remote configured; skip push.
Done.
```

From a topic branch the run asks first:

```text
Current tag: v0.1.0
Release from branch "feature/x"? [y/N]: y
Enter a version number [v0.2.0]:
```

Answering anything else stops the release before it writes a thing:

```text
bump aborted: branch "feature/x" is not a default branch (master, main)
```

When the repository is a Go module, the run ends with an upgrade hint:

```text
Use
	go get example.com/module@v0.2.0
to update upstreams.
```

### Use as a library

Add the module to your project:

```shell
go get github.com/ctx42/gmtask/pkg/gmbump
```

Both entry points take the gomake `*ring.Ring` for I/O and environment access.
`Bump` operates on the current working directory; `BumpTarget` takes an explicit
repository root.

```go
import (
	"context"
	"os"

	"github.com/ctx42/gmtask/pkg/gmbump"
	"github.com/ctx42/ring/pkg/ring"
)

rng := ring.New(ring.WithEnv(os.Environ()))

// Release the current working directory.
err := gmbump.Bump(context.Background(), rng)

// Or release a specific repository.
err = gmbump.BumpTarget(context.Background(), rng, "/path/to/repo")
```
