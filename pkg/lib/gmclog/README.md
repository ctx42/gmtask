# gmclog

Read, edit, and write Markdown changelog files from Go — semantic-version aware,
with releases kept in order.

<!-- TOC -->
* [gmclog](#gmclog)
  * [Overview](#overview)
  * [Features](#features)
  * [Prerequisites](#prerequisites)
  * [Installation](#installation)
  * [Usage](#usage)
    * [Build and render a release](#build-and-render-a-release)
    * [Prepend a release to a changelog file](#prepend-a-release-to-a-changelog-file)
    * [Parse existing releases](#parse-existing-releases)
    * [Parse a single header line](#parse-a-single-header-line)
    * [Preserve change lines verbatim](#preserve-change-lines-verbatim)
<!-- TOC -->

## Overview

`gmclog` treats a changelog as a sequence of releases. Each release is a
second-level Markdown header naming a semantic version and an RFC 1123 date,
followed by the lines describing its changes:

```markdown
## v0.1.6 (Sun, 02 Jan 2000 03:04:06 UTC)
- Change 1.
- Change 2.
```

There are two ways in. Use `ReadChangelog` to prepend new releases to the top of
a file without parsing what is already there — the cheap path for release
tooling. Use `ReadReleases` to parse existing releases into structured values
for inspection or editing. `AddRelease` sorts the structured `Releases` slice
youngest to oldest by semantic version; unparsed body text kept by
`ReadChangelog` is written unchanged.

## Features

- **Two read modes** — `ReadChangelog` prepends without parsing; `ReadReleases`
  parses existing releases for inspection or editing.
- **Ordered on add** — `AddRelease` sorts structured releases youngest to
  oldest by semantic version; unparsed body text is not reordered.
- **Automatic formatting** — change lines are prefixed with `- ` and get a
  trailing period; disable with the `WithNoFormatting` option.
- **Flexible construction** — build a `Release` from a version string, a
  `*semver.Version`, or an existing header line.

## Prerequisites

Go 1.26 or newer.

## Installation

```shell
go get github.com/ctx42/gmtask/pkg/lib/gmclog
```

## Usage

### Build and render a release

```go
date := time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC)
rel, err := gmclog.NewRelease("v0.1.2", date)
if err != nil {
    log.Fatal(err)
}
rel.AddChange("Add feature", "Fix bug")

fmt.Print(rel.String())
// ## v0.1.2 (Sun, 02 Jan 2000 03:04:06 UTC)
// - Add feature.
// - Fix bug.
```

### Prepend a release to a changelog file

`ReadChangelog` requires the file to exist; `CreateFile` makes an empty one if
it doesn't. `AddRelease` accepts one or more releases and re-sorts, and `Save`
writes the new releases above the existing contents.

```go
if err := gmclog.CreateFile("CHANGELOG.md"); err != nil {
    log.Fatal(err)
}
cl, err := gmclog.ReadChangelog("CHANGELOG.md")
if err != nil {
    log.Fatal(err)
}
cl.AddRelease(rel)
if err := cl.Save(); err != nil {
    log.Fatal(err)
}
```

### Parse existing releases

```go
cl, err := gmclog.ReadReleases("CHANGELOG.md")
if err != nil {
    log.Fatal(err)
}
for _, rel := range cl.Releases {
    fmt.Println(rel.Version.Original(), "-", len(rel.Changes), "changes")
}
```

### Parse a single header line

`ReleaseFromHeader` turns a `## version (date)` line back into a `Release`,
returning `ErrInvRelHeader`, `ErrInvRelDate`, or `ErrInvRelVersion` when the
line is malformed.

```go
hdr := "## v0.1.6 (Sun, 02 Jan 2000 03:04:06 UTC)"
rel, err := gmclog.ReleaseFromHeader(hdr)
if err != nil {
    log.Fatal(err)
}
```

### Preserve change lines verbatim

Pass `WithNoFormatting` to a constructor to keep change lines exactly as given —
no `- ` prefix, no trailing period. `ReadReleases` uses this internally so
parsed content round-trips unchanged.

```go
rel, err := gmclog.NewRelease("v0.1.2", date, gmclog.WithNoFormatting)
```
