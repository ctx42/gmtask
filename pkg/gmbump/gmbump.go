// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmbump provides the gomake ":bump" target for releasing Go projects.
//
// The target proposes the release the repository is already heading towards -
// the same one [gitaid.Derive] stamps into development builds, so the tag cut
// here can never name a different release than those builds pointed at. The
// bump comes from the Conventional Commits since the latest version tag, or
// from -p, -m or -M to force a patch, minor or major bump; on a 0.x version a
// major bump advances the minor. You confirm or override the proposal, and it
// then prepends a CHANGELOG.md entry built from the commits since that tag,
// writes the version to a VER file, then commits, tags, and pushes to origin.
//
// A release needs a clean working tree and a branch to cut from. The target
// refuses a detached HEAD outright, and on a branch other than "master" or
// "main" it asks to confirm before releasing - after establishing there is
// anything to release, so a no-op bump never asks.
//
// Import path:
//
//	import "github.com/ctx42/gmtask/pkg/gmbump"
package gmbump

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"

	"github.com/ctx42/gmtask/pkg/gmgo"
	"github.com/ctx42/gmtask/pkg/lib/gmclog"
)

// Branches a release may be cut from without an approval.
const (
	// branchMaster is the primary default branch name.
	branchMaster = "master"

	// branchMain is the alternative default branch name.
	branchMain = "main"
)

// Sentinel errors.
var (
	// ErrNotClean is returned when the working tree has uncommitted changes or
	// untracked files.
	ErrNotClean = errors.New("working directory not clean")

	// ErrNotDefBranch is returned when a release from a branch other than
	// "master" or "main" was not approved.
	ErrNotDefBranch = errors.New("not a default branch")

	// ErrBumpFlags is returned when more than one of the flags forcing a bump
	// level is set.
	ErrBumpFlags = errors.New("conflicting bump flags")

	// ErrNotNewer is returned when the version to release is not newer than
	// the current version tag.
	ErrNotNewer = errors.New("version not newer than the current tag")
)

// Bump runs the ":bump" target against the current working directory. It is the
// entry point registered with gomake; see [BumpTarget] for the behavior.
func Bump(ctx context.Context, rng *ring.Ring) error {
	return BumpTarget(ctx, rng, "")
}

// BumpTarget bumps the version of the repository rooted at repo: it tags the
// next semantic version, prepends a CHANGELOG.md entry built from the commits
// since the previous tag, writes the version to a VER file, then commits, tags,
// and pushes to origin. The empty string for repo means the current working
// directory.
//
// It returns [ErrBumpFlags] when more than one bump level is forced,
// [ErrNotClean] for a dirty working tree, [gitaid.ErrDetached] for a detached
// HEAD, [ErrNotDefBranch] when a release from a branch other than "master"
// or "main" is not approved, and [ErrNotNewer] when the entered version is not
// newer than the current version tag.
//
//nolint:cyclop
func BumpTarget(ctx context.Context, rng *ring.Ring, repo string) error {
	tgtName := ":bump"
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.SetOutput(rng.Stderr())
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s:\n", tgtName)
		_, _ = fmt.Fprint(rng.Stderr(), head+fs.HelpOptions())
	}
	fs.BoolSL("help", "h", false, "show help")
	fs.BoolSL(gitaid.BumpPatch, "p", false, "force a patch version bump")
	fs.BoolSL(gitaid.BumpMinor, "m", false, "force a minor version bump")
	fs.BoolSL(gitaid.BumpMajor, "M", false, "force a major version bump")
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fs.Args())
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}
	bump, err := forcedBump(fs)
	if err != nil {
		return err
	}

	clean, err := gitaid.IsClean(ctx, repo)
	if err != nil {
		return fmt.Errorf("check repository state: %w", err)
	}
	if !clean {
		return ErrNotClean
	}

	// A detached HEAD has no branch to release from: the commit this would
	// make is reachable only through the tag, and git refuses to push it.
	branch, err := gitaid.Branch(ctx, repo)
	if errors.Is(err, gitaid.ErrDetached) {
		return err
	}
	if err != nil {
		return fmt.Errorf("check repository branch: %w", err)
	}

	// The same derivation every other target uses, so the tag this offers to
	// cut is the release the development builds were already heading to.
	ver, err := gmgo.ProjectVersion(ctx, rng, repo, bump)
	if err != nil {
		return fmt.Errorf("resolve version: %w", err)
	}
	next, err := nextRelease(ver)
	if err != nil {
		return fmt.Errorf("resolve version: %w", err)
	}

	closest, err := gitaid.ClosestTag(ctx, repo, "")
	if err == nil && closest != "" && closest != ver.Tag {
		// Only version tags are considered, so say which one was passed
		// over rather than leave the proposal looking wrong.
		_, _ = fmt.Fprintf(rng.Stdout(), "Skipping tag: %q\n", closest)
	}

	changes, curStr, err := collectChanges(ctx, repo, ver.Tag)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "Current tag: %s\n", curStr)

	if len(changes) == 0 {
		_, _ = fmt.Fprint(rng.Stdout(), "HEAD on tag. Nothing to do.\n")
		return nil
	}

	rdr := bufio.NewReader(rng.Stdin())
	if err = approveBranch(rng, rdr, branch); err != nil {
		return err
	}

	format := "Enter a version number [%s]: "
	_, _ = fmt.Fprintf(rng.Stdout(), format, next.Original())
	txt, err := readLine(rdr)
	if err != nil {
		return fmt.Errorf("read version input: %w", err)
	}
	if txt = strings.TrimSpace(txt); txt != "" {
		if next, err = semver.NewVersion(txt); err != nil {
			return fmt.Errorf("parse version %q: %w", txt, err)
		}
	}

	// Tag the canonical form: a "v" prefix and all three version numbers, as
	// Go modules require. Short input such as "1.2" parses as "1.2.0".
	next = semver.MustParse("v" + next.String())

	// Refuse to tag backwards or reuse the current tag before any file is
	// written, so a refused release leaves the working tree clean.
	if cur, cerr := semver.NewVersion(curStr); cerr == nil &&
		!next.GreaterThan(cur) {

		format := "%w: %s is not newer than %s"
		return fmt.Errorf(format, ErrNotNewer, next.Original(), curStr)
	}

	if err = writeChangelog(repo, next, changes); err != nil {
		return err
	}

	msg := "Now you may edit CHANGELOG.md. Then press ENTER to continue.\n"
	_, _ = fmt.Fprint(rng.Stdout(), msg)
	// From here on a failure leaves the release half done; every error says
	// what state the repository was left in.
	const unfinished = "release not finished; " +
		"CHANGELOG.md and VER may be modified: %w"
	if _, err = readLine(rdr); err != nil {
		err = fmt.Errorf("read continue input: %w", err)
		return fmt.Errorf(unfinished, err)
	}
	_, _ = fmt.Fprint(rng.Stdout(), "Continuing.\n")

	pth := filepath.Join(repo, "VER")
	if err = os.WriteFile(pth, []byte(next.Original()), 0o600); err != nil {
		return fmt.Errorf(unfinished, fmt.Errorf("write VER file: %w", err))
	}

	if err = gitaid.Add(ctx, repo, "CHANGELOG.md", "VER"); err != nil {
		return fmt.Errorf(unfinished, fmt.Errorf("git add: %w", err))
	}

	cm := fmt.Sprintf("Bump version to %s.", next.Original())
	if err = gitaid.Commit(ctx, repo, cm); err != nil {
		return fmt.Errorf(unfinished, fmt.Errorf("git commit: %w", err))
	}

	tm := fmt.Sprintf("Tag version %s.", next.Original())
	if err = gitaid.Tag(ctx, repo, next.Original(), tm); err != nil {
		format := "release %s committed but not tagged: git tag: %w"
		return fmt.Errorf(format, next.Original(), err)
	}

	// Ask for the remote rather than read it off a failed push: git words a
	// missing "origin" differently depending on how the push names it.
	origin, err := gitaid.ProjectOrigin(ctx, repo)
	if err != nil {
		return fmt.Errorf("check repository origin: %w", err)
	}
	if origin == "" {
		_, _ = fmt.Fprint(rng.Stdout(), "No remote configured; skip push.\n")
	} else if err = gitaid.Push(ctx, repo); err != nil {
		format := "release %s committed and tagged locally; " +
			"push it manually: git push: %w"
		return fmt.Errorf(format, next.Original(), err)
	}

	_, _ = fmt.Fprint(rng.Stdout(), "Done.\n")

	// Print additional info if this is a Go module. The release is already
	// committed and tagged (and pushed when a remote exists), so the upgrade
	// hint is best-effort: any failure to resolve the module path is ignored
	// rather than failing a completed release.
	mod, err := gmgo.ImpPath(ctx, rng, repo)
	if err != nil {
		return nil
	}
	msg = "" +
		"\nUse\n" +
		"\tgo get %s@%s\n" +
		"to update upstreams.\n"
	_, _ = fmt.Fprintf(rng.Stdout(), msg, mod, next.Original())

	return nil
}

// writeChangelog prepends a release of the given version, dated now and built
// from changes, to the CHANGELOG.md of the repository rooted at repo. The file
// is created when it does not exist yet.
func writeChangelog(
	repo string,
	next *semver.Version,
	changes []string,
) error {

	rel := gmclog.NewSemVerRelease(next, time.Now())
	rel.AddChange(changes...)

	pth := filepath.Join(repo, "CHANGELOG.md")
	if err := gmclog.CreateFile(pth); err != nil {
		return fmt.Errorf("create changelog: %w", err)
	}
	cl, err := gmclog.ReadChangelog(pth)
	if err != nil {
		return fmt.Errorf("read changelog: %w", err)
	}
	cl.AddRelease(rel)
	if err = cl.Save(); err != nil {
		return fmt.Errorf("save changelog: %w", err)
	}
	return nil
}

// collectChanges returns the changes to release since tag, and the tag they
// are measured from. That tag is the empty string when the repository carries
// no version tag yet, and every commit is then a change.
func collectChanges(
	ctx context.Context,
	repo, tag string,
) ([]string, string, error) {

	changes, err := gitaid.ChangeLog(ctx, repo, tag)
	if errors.Is(err, gitaid.ErrUnkTag) {
		// The tag git does not know is the synthetic base gitaid falls back
		// to when the repository has no version tag.
		tag = ""
		changes, err = gitaid.ChangeLog(ctx, repo, "")
	}
	if err != nil {
		return nil, "", fmt.Errorf("read changelog: %w", err)
	}
	return changes, tag, nil
}

// approveBranch asks to confirm a release cut from branch and returns nil when
// it is approved. A default branch is approved without asking; anywhere else
// only "y" or "yes", in any case, goes on, and any other answer yields an error
// wrapping [ErrNotDefBranch].
func approveBranch(rng *ring.Ring, rdr *bufio.Reader, branch string) error {
	if branch == branchMaster || branch == branchMain {
		return nil
	}

	format := "Release from branch %q? [y/N]: "
	_, _ = fmt.Fprintf(rng.Stdout(), format, branch)
	txt, err := readLine(rdr)
	if err != nil {
		return fmt.Errorf("read approval input: %w", err)
	}
	if ans := strings.ToLower(strings.TrimSpace(txt)); ans == "y" ||
		ans == "yes" {

		return nil
	}
	format = "bump aborted: branch %q is %w (master, main)"
	return fmt.Errorf(format, branch, ErrNotDefBranch)
}

// readLine reads one line of input from rdr. A last line not ended by a
// newline, as piped input often is, is returned as an answer rather than
// failing with [io.EOF]; only end of input with nothing read is an error.
func readLine(rdr *bufio.Reader) (string, error) {
	txt, err := rdr.ReadString('\n')
	if errors.Is(err, io.EOF) && txt != "" {
		return txt, nil
	}
	return txt, err
}

// nextRelease returns the release ver heads towards. It is the version core
// of the development version [gitaid.Derive] built, which is the last tag
// already advanced by the bump - so the tag cut here and the one stamped into
// a development build can never name different releases.
func nextRelease(ver gitaid.Version) (*semver.Version, error) {
	sem, err := semver.NewVersion(ver.Rev)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ver.Rev, err)
	}
	core := fmt.Sprintf("v%d.%d.%d", sem.Major(), sem.Minor(), sem.Patch())
	return semver.NewVersion(core)
}

// forcedBump returns the bump level the flags force, or the empty string when
// none does. Each level is forced by the flag of the same long name. It
// returns [ErrBumpFlags] when more than one is set.
func forcedBump(fs *xflag.FlagSet) (string, error) {
	var bump string
	for _, lvl := range []string{
		gitaid.BumpPatch,
		gitaid.BumpMinor,
		gitaid.BumpMajor,
	} {
		if !fs.GetBool(lvl) {
			continue
		}
		if bump != "" {
			return "", fmt.Errorf("%w: --%s and --%s", ErrBumpFlags, bump, lvl)
		}
		bump = lvl
	}
	return bump, nil
}
