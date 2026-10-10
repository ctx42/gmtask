## v0.15.1 (Sat, 10 Oct 2026 19:31:14 UTC)
- docs: shorten the opening sentence of target godoc.
- build(deps): update 7 ctx42 dependencies.

## v0.15.0 (Sat, 10 Oct 2026 12:14:44 UTC)
- feat(gmbump): add --unattended to release without input.

## v0.14.0 (Sat, 10 Oct 2026 09:01:07 UTC)
- feat(gmbump): release a --set version with no new commits.
- feat(gmprj): override config file keys from the environment.

## v0.13.0 (Fri, 09 Oct 2026 19:41:41 UTC)
- feat(gmbump)!: add --set and validate release versions.

## v0.12.0 (Fri, 09 Oct 2026 13:01:58 UTC)
- fix(gmtest): validate project names and fail on nil projects.
- fix(gmclog): keep the changelog file mode on save.
- fix(gmclog): update the target of a symlinked changelog.
- fix(gmclog): create the changelog without truncating or accepting dirs.
- fix(gmclog): treat only version-led headers as releases.
- fix(gmclog)!: reject release dates outside UTC.
- fix(gmclog): stop mangling blank and punctuated change lines.
- fix(gmclog): name the line of a malformed release header.
- fix(gmclog): keep the changelog title above new releases.
- refactor(gmclog)!: leave the release separator to Save.
- test(gmclog): cover a failed rename in Save.
- docs(gmclog): correct sort order, nil and normalization notes.
- refactor(gmclog): move CreateFile to helpers and tidy writes.
- test(gmclog): align tests with the project test style.
- fix(gmmce): track code fences when injecting examples.
- fix(gmmce): refuse a marker whose code fence never closes.
- fix(gmmce): warn about markers that name no example.
- fix(gmmce): find CRLF and indented markers.
- fix(gmmce)!: skip testdata, vendor and hidden dirs when scanning.
- fix(gmmce)!: reject positional arguments to :doc:mce.
- fix(gmmce): keep raw string lines verbatim in example bodies.
- fix(gmmce): write the Markdown file atomically and only on change.
- fix(gmmce): collect only real example functions.
- test(gmmce): cover invalid source and failed write paths.
- refactor(gmmce): wrap scan errors and tidy declarations.
- test(gmmce): align tests with the project test style.
- fix(gmbump): tag the canonical form of a typed version.
- test(gmbump): make the -p test fail when -p is ignored.
- fix(gmbump): refuse a version not newer than the current tag.
- fix(gmbump): accept a last answer without a trailing newline.
- fix(gmbump): say what a half-done release left behind.
- refactor(gmbump): keep prompts in the target and tidy declarations.
- test(gmbump): align tests with the project test style.
- fix(gmgo): resolve one module path inside a Go workspace.
- fix(gmgo): parse target flags without rewriting the caller's ring.
- fix(gmgo): lint with the config :go:lint placed, and stop on --help.
- fix(gmgo): let a -timeout argument win over the configured one.
- fix(gmgo): honor a zero test timeout and reject negative ones.
- fix(gmgo,gmprj)!: reject an unparseable C42_BLD_DATE.
- fix(gmgo): refuse builds that would lose injected metadata.
- fix(gmgo): fetch the lint config from the default branch.
- docs(gmgo): say the lint config comes from the default branch.
- fix(gmgo): clone the lint config with the ring environment.
- fix(gmgo)!: read GOMAKE_GOLINT_CONFIG_FORCE as a boolean.
- fix(gmgo): find golangci-lint where go install puts it.
- fix(gmgo): tell gmgo.ErrConfig apart from gomake.ErrConfig.
- test(gmgo): keep host environment variables out of the tests.
- test(gmgo): cover pinGoMajorMinor and untested error paths.
- fix(gmgo): drop the dangling separator from the no-go.mod error.
- test(gmgo): fail instead of panicking and drop a duplicate test.
- docs(gmgo): fix godoc grammar and stale package wording.
- refactor(gmgo): gather package-wide declarations in gmgo.go.
- test(gmgo): align tests with the project test style.
- fix(gmgo): never find golangci-lint on the process PATH.
- test(gmgo): fail fast when the first lint run in a test fails.
- fix(gmprj): derive the module path from URL remotes.
- fix(gmprj): ignore a /vN module suffix when naming the project.
- fix(gmprj,gmdkr)!: single-quote values in --export output.
- fix(gmprj): write the Docker repository as C42_REG_REPO.
- fix(gmprj): apply a directory node's mode after its children.
- fix(gmprj): honor setuid, setgid and sticky bits in node modes.
- fix(gmprj): reject structure node names that leave their parent.
- fix(gmprj): make Info.Set safe on a zero Info.
- fix(gmprj): make a relative project root absolute in NewSetup.
- fix(gmprj): derive a valid Go identifier for the package name.
- fix(gmprj)!: reject stray arguments to the project targets.
- fix(gmprj): take the Go spec only from a go.mod in the project root.
- test(gmprj): isolate the tests from the host git config and env.
- perf(gmprj): look values up without rebuilding the environment.
- fix(gmprj): name the step that failed in setup and info errors.
- docs(gmprj): correct stale godoc and tidy constant groups.
- style(gmprj): wrap lines over 80 columns.
- docs(gmprj): add GetInfo and ExportEnv examples.
- test(gmprj): drop duplicate tests and tighten weak assertions.
- test(gmprj): align tests with the project test style.
- fix(gmdkr): keep container arguments out of docker image ls.
- fix(gmdkr): keep the exec error and cancellation in docker errors.
- fix(gmdkr): let dry runs of run and sh skip the image listing.
- fix(gmdkr): accept a target named twice in --targets.
- fix(gmdkr)!: report an uninitialized DockerCmd with ErrNoBuilds.
- fix(gmdkr)!: do not move "latest" unless a release asks for it.
- fix(gmdkr)!: give :docker:image:clean flags and reject arguments.
- fix(gmdkr): split --cmd with shell quoting.
- fix(gmdkr): stream run and sh sessions and cap docker error text.
- fix(gmdkr): mount the SSH socket where run-proj mounts it in sh.
- fix(gmdkr): parse target flags without rewriting the caller's ring.
- test(gmdkr): fix leaks, dead checks and flaky timing in tests.
- refactor(gmdkr): tidy errors, declarations and godoc.
- style(gmdkr): wrap lines over 80 columns.
- docs(gmdkr): add ImgName, ImgTag and Build.Cmd examples.
- test(gmdkr): align tests with the project test style.
- docs: spell the gomake hidden tag with its required space.
- test(gmdkr): build pickTargets arguments in Given.
- fix(gmdkr)!: log in to the registry host in :docker:login.
- docs(gmgo): note that :go:test always tests the whole module.

## v0.11.0 (Thu, 08 Oct 2026 12:52:59 UTC)
- build(deps): upgrade dotenv, gomake, ring and xflag.
- feat(gmbump): add -m and -M flags to force minor and major bumps.

## v0.10.0 (Thu, 08 Oct 2026 12:23:47 UTC)
- build(deps)!: upgrade gitaid to v0.8.0.
- feat(gmprj)!: set up new repositories with master and develop.

## v0.9.0 (Mon, 28 Sep 2026 10:23:26 UTC)
- build(deps): bump gomake to 0.27.1.
- fix(gmbump): skip the push only when origin is unset.
- feat(gmdkr): push and cache release images from the build.
- chore: tidy go.sum.
- test(gmprj): give git an identity for Setup commits.
- test(gmgo): ignore go install download output.

## v0.8.0 (Thu, 24 Sep 2026 11:26:48 UTC)
- test(gmgo): stop the timeout tests racing the watchdog.
- refactor(gmprj): lift the config lookup into the loop condition.
- refactor!: derive versions through gitaid.
- feat(gmbump)!: propose the release the commits imply.
- docs: record the versioning conventions for agents.
- feat(gmbump)!: ask before bumping off master or main.
- chore: add an IDE test run configuration and project config.
- test(gmbump): pin the clean check ahead of the branch gate.
- refactor(gmbump): lift the release steps out of BumpTarget.
- feat(gmprj)!: refuse to scaffold into a non-empty directory.
- build(deps): bump gitaid to 0.6.1.
- build(deps): bump gitaid to 0.6.2.

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

