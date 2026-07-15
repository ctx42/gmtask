## v0.5.0 (Wed, 15 Jul 2026 08:11:36 UTC)
- refactor(gmtest): route NewProject through NewNamedProject.
- docs: clarify target installation modes in README.
- feat(gmdkr): add :docker:* targets.
- refactor(gmprj): group errors and constants; genericize test fixtures.
- feat(gmmce): thread context through Mce and add runnable example.
- feat(gmgo): pin go.mod directive to major.minor and pass env to commands.
- refactor(gmbump): wrap errors with context and best-effort upgrade hint.
- fix(gmclog): keep changelog preamble and harden release parsing.
- build(deps): bump gomake, testkit, and xdef.

## v0.4.0 (Sun, 12 Jul 2026 21:15:40 UTC)
- feat(gmmce): add :doc:mce example injection target.

## v0.3.0 (Sun, 12 Jul 2026 20:38:48 UTC)
- feat(gmprj): add project scaffolding and env targets.

## v0.2.0 (Sat, 11 Jul 2026 11:36:44 UTC)
- feat(gmbump): add version bumping and changelog generation tool.
- refactor(gmbump): return skipped tags from getSemVer.
- test(gmgo): raise per-function coverage.
- test(gmclog): cover changelog error paths.
- style(gmbump): segment multi-line upstream-update message.
- docs: add TOCs and tidy package READMEs.
- docs: expand root README installation and packages.
- docs(gmgo): list :go:test and :go:test-v separately.

## v0.1.0 (Fri, 10 Jul 2026 20:32:57 UTC)
- Initial commit.
- feat: The initial public release of gmtask.
- feat(gmgo): add :go:pkgsite documentation target.
- ci: install golangci-lint before running tests.

