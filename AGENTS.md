# AGENTS.md

Guidance for AI agents working in this repository. Keep it current when the
build, layout, or conventions change.

## What this is

`gmtask` (module `github.com/ctx42/gmtask`, Go 1.26) is a collection of reusable
[gomake](https://github.com/ctx42/gomake) targets and supporting libraries for
Go projects. The target packages compile into the gomake binary; the libraries
are plain importable packages.

## Layout

| Path              | Role                                           |
|-------------------|------------------------------------------------|
| `pkg/gmgo`        | gomake `:go:*` targets + library helpers.      |
| `pkg/gmbump`      | gomake `:bump` release target.                 |
| `pkg/gmprj`       | Project setup/scaffolding targets and helpers. |
| `pkg/lib/gmclog`  | Library: read/edit/write Markdown changelogs.  |
| `internal/gmtest` | Test helper for temporary Go projects.         |

Note: `pkg/gmprj` exists but is not yet listed in the root `README.md` package
index (Targets/Libraries). Add it there when it stabilizes.

## Build, test, lint

- Test (canonical, matches CI): `go test -race ./...`.
- Lint: `golangci-lint run` — pinned to `v2.12.2`; shared config is fetched from
  `git@github.com:ctx42/xdev.git`.
- CI (`.github/workflows/test.yml`) installs golangci-lint `v2.12.2` and runs
  `go test -race ./...` on push and PR.
- Dogfooding: if `gmgo`/`gmbump` are compiled into your gomake binary, run
  `gomake :go:check`, `gomake :go:test`, etc.

## gomake target model (how targets are defined)

- A top-level exported `func Name(ctx context.Context, rng *ring.Ring) error`
  becomes a root target `:name` (e.g. `Bump` → `:bump`).
- A namespace is `type Ns struct{} //gomake:ns_root` with target methods on it;
  the target name is the type + method path (e.g. `Go.Vet` → `:go:vet`,
  `Lint.Config` → `:go:lint:config`). The `//gomake:` comment tag must have no
  space after `//`.
- `*ring.Ring` carries I/O (`rng.Stdout()`, `rng.Stderr()`, `rng.Args()`) and
  environment access; pass it through, do not reach for `os.Stdout`/`os.Args`.
- Read settings with `gomake.TargetConfig(rng)` + `gomake.GetCfgDefault(...)`
  from `gomake.yaml`, keyed by import path → namespace → target.

## Two ways to consume the targets (document both)

- **Built-in** — list the package in `targets.yaml` at the gomake source root
  and rebuild the binary; the target is then available in every project.
- **Per-project** — `//gomake:import` a blank-identifier import in a project's
  `makefile.go` (with `//go:build gomake`); available in that project only. The
  package must be in the project's module graph (`go get <pkg-path>`).

`go get` needs a path that resolves to a real package — this module has no root
package, so use the subpackage path (`go get github.com/ctx42/gmtask/pkg/gmgo`),
not the bare module path.

## Go code conventions

- Follow the `golang:style` skill; production (`*.go`) and test (`*_test.go`)
  files follow different rules. Use `golang:review` after finishing edits.
- Prefer gopls/LSP for Go symbol navigation over text search.
- Package godoc lives in the file named after the package (e.g. `gmclog.go`),
  not `doc.go`. Package-wide constants go in that same file in a documented
  `const (...)` block.
- For short multi-line string literals use the `"" + "line\n"` concatenation
  form, not backtick raw strings.
- `gmgo` target settings are configured through `gomake.yaml` `target-config`,
  not env vars (env only as an explicit override).
- Tests use `github.com/ctx42/testing` (`pkg/assert`, `pkg/must`) and
  `github.com/ctx42/testkit` (`oskit`, `pathkit`, `prjkit`). Structure test
  bodies with `// --- Given ---`, `// --- When ---`, `// --- Then ---` blocks.
- Dependencies are the ctx42 ecosystem (`ring`, `gitaid`, `gomake`, `testing`,
  `testkit`, `xdef`, `xflag`) plus `Masterminds/semver/v3`.

## Documentation / README conventions

- `.editorconfig` governs: `*.md` wraps at 80 columns, `*.go` uses tabs, YAML
  uses 2-space indent. Count characters, not bytes — an em-dash is 3 bytes but
  one column.
- Use `craft:readme-smith` for README work. Root vs member READMEs differ: the
  root carries badges, a `## License` section, and a package index (split into
  Targets and Libraries); member READMEs carry no badges/License and never link
  up to the root.
- Markdown tables must have aligned columns (pad every cell; align the `|`).
- Table of contents is a tool-generated `<!-- TOC -->` block (H1 + nested
  H2/H3), the same in every README.
- Features section: one feature per line, ≤80 columns. Lead each bullet with
  a command token (`` `:go:vet` ``) for a multi-target package, or a
  capability phrase for a single-command package — pick one style all the
  repo's READMEs can share.
- Do not add goreportcard.com badges — the service no longer works.

## Commits

- Conventional Commits with an imperative, lowercase summary (`docs:`, `feat:`,
  `fix:`, `refactor:`, `test:`, …); optional kernel-style body wrapped at 72
  columns explaining *why*. Use the `craft:cm` skill.
- Never add a `Co-Authored-By` / AI-attribution trailer.
