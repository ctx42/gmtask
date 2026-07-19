## v0.7.0 (Sun, 19 Jul 2026 20:10:49 UTC)
- feat(gmgo): make lint config source repo configurable.

## v0.6.1 (Thu, 16 Jul 2026 19:30:31 UTC)
- build(deps): bump gomake to 0.26.0 and testing to 0.56.0.

## v0.6.0 (Wed, 15 Jul 2026 18:19:01 UTC)
- fix(gmdkr): clone ring for dangling Clean ImgLs.
- fix(gmdkr): drop docker pull from goImageLatest ref.
- fix(gmmce): skip blank lines before fence replace.
- fix(gmmce): keep single-line example body content.
- fix(gmbump): use Original in commit and tag messages.
- fix(gmprj): return non-ErrNotRepo IsRepo failures in Setup.
- fix(gmprj): honor structure feature gates on materialize.
- fix(gmgo): auto-install golangci-lint when missing.
- fix(gmgo): drop shell quotes from godoc -notes argv.
- fix(gmdkr): return deleteImage errors from Clean.
- fix(gmgo): wrap go command run errors in ImpPath and InitModule.
- fix(gmclog): write changelog via temp file rename.
- fix(gmmce): use slash separators in example marker keys.
- fix(gmprj): log remote origin only after AddRemote succeeds.
- fix(gmgo): pass ring env to godoc and pkgsite.
- fix(gmgo): wire lint install stdout and stderr to ring.
- test(gmgo): cover lint config resolved under dir.
- fix(gmprj): add ErrModuleOriginMismatch sentinel.
- fix(gmbump): note skipped push when no remote is set.
- docs(gmgo): fix Lint.Default godoc to match ring output.
- docs(gmdkr): cite --targets flag in target error godoc.
- docs(gmclog): document AddRelease sort scope accurately.
- fix(gmprj): include node path in structure create errors.
- fix(gmmce): wrap findExamples errors in Mce.
- fix(gmdkr): add Build.CmdStdin for stdin Dockerfile builds.
- docs(gmdkr): fix fromInfo godoc for empty values.
- docs(gmdkr): fix Reference godoc and ImageInfos name.
- docs(gmprj): fix NewSetup and initScmRepo godoc.
- fix(gmprj): require origin or module with --mkdir.
- fix(gmprj): make Root walk portable and use CfgPath.
- test(gmclog): pin distinctive error causes.
- refactor(gmclog): keep package sentinels in gmclog.go.
- fix(gmprj): include path in config parse errors.
- docs(gmdkr): distinguish ErrNoTarget and ErrUnkTarget.
- docs(gmgo): fix ErrModInit and ErrImpPath godoc.
- test(gmbump): use Nil for absent current version.
- test(gmbump): use Nil for remaining nil current version.
- docs(gmdkr): fix godoc english and pickTargets wording.
- docs(gmbump): clarify version prefix comment and wrap text.
- docs(gmclog): cross-ref NewSemVerRelease and ErrInvRelVersion.
- style(gmmce): move WriteFile nolint off the call line.
- test(gmtest): cover ImpSpec and prepare name in Given.
- docs: drop outdated gmprj README index note from AGENTS.
- test(gmprj): fix When section tag in Lookup subtest.
- style(gmclog): use three-letter Changelog receiver.
- test(gmgo): name stdout have and log content wantLog.
- test(gmmce): blank line between Then assertion groups.
- style: drop Build.CmdStdin and align test have/want style.
- docs: align root README with members and public targets.
- docs: refresh AGENTS layout and README conventions.
- docs: use one go get for the module in root README.

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

