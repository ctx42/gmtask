# gmprj

[gomake](https://github.com/ctx42/gomake) targets for scaffolding a new Go
project and inspecting its build environment.

<!-- TOC -->
* [gmprj](#gmprj)
  * [Overview](#overview)
  * [Targets](#targets)
  * [Configuration](#configuration)
    * [gomake.yaml](#gomakeyaml)
    * [The structure block](#the-structure-block)
    * [Node fields](#node-fields)
    * [Template variables](#template-variables)
<!-- TOC -->

## Overview

`gmprj` provides the `:project:*` gomake targets. `:project:setup` creates a
new project's directory and file layout, initializes the Go module, and sets up
a git repository: an empty initial commit on `master` tagged `v0.0.0`, and the
project files committed on a `develop` branch made from it. The layout is not
hardcoded: it is read from a `structure` block you author in your `gomake.yaml`,
so each team controls exactly what a fresh project looks like.

Because `master` holds only the empty root, every commit on `develop` - the
scaffold commit included - can be reworded, split, or squashed with
`git rebase -i master`, then squash-merged into `master` once it is ready:

```shell
git rebase -i master
git switch master
git merge --squash develop
git commit
```

A directory that already is a git repository is left as it is: no branch,
commit, or tag is made.

## Targets

- **`:project:setup`** — scaffold a project from the `structure` block, init the
  module, and set up git with `master` and `develop`. Refuses a target
  directory that already holds entries, `.git` included, so a forgotten
  `-d/--mkdir` cannot scatter the scaffold over an existing project.
  `-f/--force` says the directory was meant: it lifts that check and lets
  `-d/--mkdir` adopt a directory that already exists. Flags: `-o/--origin`,
  `-m/--module`, `-d/--mkdir`, `-f/--force`.

  The target only parses the flags — `Setup` owns the decisions, so the same
  guards apply to `NewSetup(root, WithSetupMkdir(…), WithSetupForce(…))` used
  as a library. Every check runs before the first write, so a rejected run
  leaves no directory behind.
- **`:project:env`** — print project information as `KEY=value` environment
  lines.
- **`:project:info`** — print the same information in a readable form.

## Configuration

### gomake.yaml

`:project:setup` reads a `structure` block from the `targets:` tree, keyed by
this package's import path. gomake delivers the block to the target; setup then
creates every declared directory and file. There is no built-in default — with
no `structure` block, setup reports an error and creates nothing.

### The structure block

The block is a nested tree. A node is a `file` or a `dir`. A directory nests its
children under keys other than the reserved attribute keys (`type`, `content`,
`mode`, `feature`). A file's body is its `content`, rendered as a
[`text/template`](https://pkg.go.dev/text/template). Existing files are never
overwritten, so re-running setup is safe.

```yaml
version: 1
targets:
  github.com/ctx42/gmtask/pkg/gmprj:
    structure:
      build:                        # empty directory
        type: dir
      cmd:
        type: dir
      configs:
        type: dir
        project.conf:               # empty file inside configs/
          type: file
          content: ""
      dev:
        type: dir
        idea:
          type: dir
          go-test-all.run.xml:      # content templated with the project name
            type: file
            content: |
              <component name="ProjectRunConfigurationManager">
                <configuration name="go test all" type="GoTestRunConfiguration">
                  <module name="{{.ProjectName}}" />
                </configuration>
              </component>
      scripts:
        type: dir
        build.sh:                   # explicit octal permission
          type: file
          mode: "0755"
          content: |
            #!/bin/sh
      README.md:
        type: file
        content: ""
      .gitignore:                   # created only with the git feature
        type: file
        feature: git
        content: |
          tmp/
          dist/
```

A complete, copy-pasteable example is in
[`doc/gomake.yaml`](../../doc/gomake.yaml).

### Node fields

| Field     | Applies to | Meaning                                    | Default                   |
|-----------|------------|--------------------------------------------|---------------------------|
| `type`    | all        | `file` or `dir` (required)                 | —                         |
| `content` | file       | file body, rendered as a `text/template`   | empty                     |
| `mode`    | all        | octal permission string, e.g. `"0755"`     | dirs `0777`, files `0600` |
| `feature` | all        | gating feature: `base`, `git`, or `golang` | `base` (always made)      |

### Template variables

A file's `content` is rendered with these variables:

| Variable       | Value                   |
|----------------|-------------------------|
| `.ProjectName` | project name            |
| `.Module`      | Go module path          |
| `.Package`     | root Go package name    |
| `.Origin`      | git remote origin       |
| `.Repo`        | Docker image repository |
