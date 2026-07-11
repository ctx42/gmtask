# gmbump

The [gomake](https://github.com/ctx42/gomake) `:bump` target — tag the next
release, write its changelog, and push, in one command.

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

- **Automatic version proposal** — bumps the minor of the latest tag by
  default, or the patch with `-p`; starts at `v0.0.0` for an untagged repo.
- **Interactive confirmation** — accept the proposed version or type your own;
  a missing `v` prefix is added for you.
- **Changelog generation** — prepends a dated release built from the commit
  subjects since the last tag, then pauses so you can edit it.
- **Clean-tree guard** — refuses to run when the working tree has uncommitted
  or untracked changes.
- **Skips non-semver tags** — walks back past tags that are not valid semantic
  versions.
- **Go-module aware** — prints the `go get module@version` hint after releasing
  a module.

## Prerequisites

- Go 1.26 or newer.
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

### As a library

```shell
go get github.com/ctx42/gmtask/pkg/gmbump
```

## Usage

Run from the repository root:

```shell
gomake :bump        # bump the minor version
gomake :bump -p     # bump the patch version instead
gomake :bump -h     # show help
```

A run:

1. Verifies the working tree is clean; aborts otherwise.
2. Prints the current tag and prompts for the next version, pre-filled with the
   proposal — press ENTER to accept it or type your own.
3. Prepends the release to `CHANGELOG.md` and pauses so you can edit it; press
   ENTER to continue.
4. Writes `VER`, commits `CHANGELOG.md` and `VER`, tags the commit, and pushes
   the tag to origin (a missing remote is not an error).

A minor bump of a `v0.1.0` repository looks like:

```text
Current tag: v0.1.0
Enter a version number [v0.2.0]:
Now you may edit CHANGELOG.md. Then press ENTER to continue.
Continuing.
Done.
```

When the repository is a Go module, the run ends with an upgrade hint:

```text
Use
	go get example.com/module@v0.2.0
to update upstreams.
```

### As a library

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
