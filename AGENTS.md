# AGENTS.md

Guidance for AI agents working in this repository. Keep it current when the
build, layout, or conventions change.

## What this is

`gmtask` (module `github.com/ctx42/gmtask`, Go 1.26) is a collection of reusable
[gomake](https://github.com/ctx42/gomake) targets and supporting libraries for
Go projects. The target packages compile into the gomake binary; the libraries
are plain importable packages.

## Layout

| Path              | Role                                              |
| ----------------- | ------------------------------------------------- |
| `pkg/gmgo`        | gomake `:go:*` targets + library helpers.         |
| `pkg/gmdkr`       | gomake `:docker:*` targets + image helpers.       |
| `pkg/gmbump`      | gomake `:bump` release target.                    |
| `pkg/gmprj`       | gomake `:project:*` setup/info targets + helpers. |
| `pkg/gmmce`       | gomake `:doc:mce` Markdown example injection.     |
| `pkg/lib/gmclog`  | Library: read/edit/write Markdown changelogs.     |
| `internal/gmtest` | Test helper for temporary Go projects.            |

## Build, test, lint

- Test (canonical, matches CI): `go test -race ./...`.
- Lint: `golangci-lint run` — pinned to `v2.12.2`; shared config is fetched from
  `git@github.com:ctx42/xdev.git`.
- CI (`.github/workflows/test.yml`) installs golangci-lint `v2.12.2` and runs
  `go test -race ./...` on push and PR.
- Dogfooding: if the packages are compiled into your gomake binary, run
  `gomake :go:check`, `gomake :go:test`, `gomake :docker:image:build`, etc.

## gomake target model (how targets are defined)

- A top-level exported `func Name(ctx context.Context, rng *ring.Ring) error`
  becomes a root target `:name` (e.g. `Bump` → `:bump`).
- A namespace is `type Ns struct{} //gomake:ns_root` with target methods on it;
  the target name is the type + method path (e.g. `Go.Vet` → `:go:vet`,
  `Lint.Config` → `:go:lint:config`). The `//gomake:ns_root` tag must have no
  space after `//`.
- `// gomake:hidden` in a target method's godoc keeps it out of public docs and
  indexes (e.g. incomplete `:docker:image:run-proj`). Unlike the other tags,
  gomake's parser requires the space after `//`.
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
  `testkit`, `xdef`, `xflag`) plus `Masterminds/semver/v3`. `xdef` owns the
  shared `C42_*` names, `gitaid` owns git and version construction - see
  [Versioning](#versioning). Editing either is a separate module and a
  separate release: ask before doing it.

## Versioning

One derivation, in one place. `gitaid` is the authoritative source for how a
version is built and what its parts mean; nothing here assembles one.

- **Never construct a version.** Call `gmgo.ProjectVersion`, which wraps
  `gitaid.Derive`. `gmprj.Info.Version`, `gmgo.buildValues` and `gmbump` all
  go through it, which is what keeps the version a binary reports, the one
  its image carries, and the one `:bump` offers to tag identical.
- **Never bump or parse a version by hand.** `Masterminds/semver/v3` does the
  arithmetic; a regex that ignored a tag's pre-release once turned
  `v1.0.0-rc.1` plus a patch into `v1.0.1`, skipping the release the rc named.
- **`dev` and `dirty` are different words.** `dev` marks a development build -
  anything that is not a release - and `dirty` marks the working tree. Never
  spell either as the other. A tree is dirty when `git status --porcelain`
  reports anything at all, untracked files included.
- The bump level is project policy, so `gitaid` takes it as an argument.
  `gmgo.InferBump` reads it off the Conventional Commits, and
  `gmgo.EnvBldBump` (`C42_BLD_BUMP`) overrides it. That name stays in this
  module.
- `C42_SCM_STATE` is deliberately not a Docker build argument: the `.dirty`
  identifier in the version already carries the tree state, and two sources
  for one fact drift.
- Only a release may move the `latest` image tag - `gmdkr` gates it on
  `Version.Release` and on the tag carrying no pre-release, not on the `-l`
  flag alone. A `v1.0.0-rc.1` checkout is a release to `gitaid` but not to
  `latest`.

## Documentation / README conventions

- `.editorconfig` governs: `*.md` wraps at 80 columns, `*.go` uses tabs, YAML
  uses 2-space indent. Count characters, not bytes — an em-dash is 3 bytes but
  one column.
- Use `craft:readme-smith` for README work.
- **Root vs member:** root carries badges, a `## License` section, and a package
  index (Targets table + Libraries table + relative links down to each member).
  Members carry no badges, no License, and never link up to the root; they may
  link sideways to a sibling only when they use it.
- **Root header order:** badges (one per line) → `#` H1 → tagline → TOC.
  **Member header order:** `#` H1 → tagline → TOC.
- **Target names** always include the leading colon (`` `:go:vet` ``, not
  `` `go:vet` ``) in Features, Usage, and the root Targets table.
- **Root Targets table** lists every public target. Omit methods marked
  `// gomake:hidden`. Keep the table in sync when adding a target.
- **Features:** one feature per line, ≤80 columns. Multi-target packages lead
  each bullet with a command token (`` `:go:vet` ``); single-command packages
  lead with a capability phrase — one style shared across the repo's members.
- Markdown tables must have aligned columns (pad every cell; align the `|`).
  Prefer total line width ≤80 when descriptions allow.
- Code fences declare a language; keep fence lines ≤ ~100 chars (prefer ≤80);
  break long `go get` / shell lines with `\`.
- Table of contents is a tool-generated `<!-- TOC -->` block (H1 + nested
  H2/H3), the same in every README.
- Do not add goreportcard.com badges — the service no longer works.

## Commits

- Conventional Commits with an imperative, lowercase summary (`docs:`, `feat:`,
  `fix:`, `refactor:`, `test:`, …); optional kernel-style body wrapped at 72
  columns explaining *why*. Use the `craft:cm` skill.
- Never add a `Co-Authored-By` / AI-attribution trailer.
