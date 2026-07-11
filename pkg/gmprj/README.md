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
a git repository with an initial commit tagged `v0.0.0`. The layout is not
hardcoded: it is read from a `structure` block you author in your `gomake.yaml`,
so each team controls exactly what a fresh project looks like.

## Targets

- **`:project:setup`** — scaffold a project from the `structure` block, init the
  module, and init/commit/tag git. Flags: `-o/--origin`, `-m/--module`,
  `-d/--mkdir`.
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
