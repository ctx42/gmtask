// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmbump provides the gomake ":bump" target for releasing Go projects.
//
// The target proposes the release the repository is already heading towards -
// the same one [gitaid.Derive] stamps into development builds, so the tag cut
// here can never name a different release than those builds pointed at. The
// bump comes from the Conventional Commits since the latest version tag, or
// from -p, -m or -M to force a patch, minor or major bump; on a 0.x version a
// major bump advances the minor. You confirm or override the proposal, or skip
// the question with -s and a version, and it then prepends a CHANGELOG.md
// entry built from the commits since that tag, writes the version to a VER
// file, then commits, tags, and pushes to origin.
//
// A version must be a full MAJOR.MINOR.PATCH with an optional pre-release and
// "v" prefix; partial versions and leading zeros are refused, and so is build
// metadata in a Go module. It must be newer than the current tag and must not
// name a tag yet, locally or on origin; one lower than the proposal is warned
// about. A refused version typed at the prompt is asked for again. An origin
// that cannot be queried stops the release unless -f is given.
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
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"

	"github.com/ctx42/gmtask/pkg/gmgo"
	"github.com/ctx42/gmtask/pkg/lib/gmclog"
)

// Branches a release may be cut from without an approval.
const (
	branchMaster = "master"
	branchMain   = "main"
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

	// ErrBadVersion is returned when a version is not a full semantic
	// version, or carries build metadata in a Go module.
	ErrBadVersion = errors.New("invalid version")

	// ErrTagExists is returned when the version to release already names a
	// tag, locally or on origin.
	ErrTagExists = errors.New("tag already exists")
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
// It returns [ErrBumpFlags] when more than one bump level is forced, or one is
// forced together with --set, [ErrBadVersion] for an invalid --set version,
// [ErrNotClean] for a dirty working tree, [gitaid.ErrDetached] for a detached
// HEAD, and [ErrNotDefBranch] when a release from a branch other than "master"
// or "main" is not approved. A --set version not newer than the current tag
// returns [ErrNotNewer], and one already a tag returns [ErrTagExists]. An
// origin that cannot be queried returns an error wrapping [gitaid.ErrRemote]
// unless --force is given.
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
	fs.StringSL("set", "s", "", "release the given version without asking")
	fs.BoolSL("force", "f", false, "release when origin cannot be checked")
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

	// Go modules cannot fetch a version carrying build metadata.
	gomod := gomake.FileExists(filepath.Join(repo, "go.mod"))

	// A version given up front is checked before anything else, so a typo
	// fails at once whatever the repository state.
	set, err := setVersion(fs, bump, gomod)
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

	// An error means the repository has no tag at all, which leaves nothing
	// to report; it only suppresses the notice below.
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
	if branch != branchMaster && branch != branchMain {
		format := "Release from branch %q? [y/N]: "
		_, _ = fmt.Fprintf(rng.Stdout(), format, branch)
		if err = approveBranch(rdr, branch); err != nil {
			return err
		}
	}

	// Ask for the remote rather than read it off a failed push: git words a
	// missing "origin" differently depending on how the push names it.
	origin, err := gitaid.ProjectOrigin(ctx, repo)
	if err != nil {
		return fmt.Errorf("check repository origin: %w", err)
	}

	// Every version is checked before any file is written, so a refused
	// release leaves the working tree clean.
	grd := guard{
		repo:   repo,
		origin: origin,
		cur:    curStr,
		gomod:  gomod,
		force:  fs.GetBool("force"),
	}
	if next, err = grd.choose(ctx, rng.Stdout(), rdr, set, next); err != nil {
		return err
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
	format := "" +
		"\nUse\n" +
		"\tgo get %s@%s\n" +
		"to update upstreams.\n"
	_, _ = fmt.Fprintf(rng.Stdout(), format, mod, next.Original())

	return nil
}

// guard decides whether a version may be released from a repository.
type guard struct {
	repo   string // Repository root; empty for the working directory.
	origin string // Origin URL; empty when no origin is configured.
	cur    string // Current version tag; empty when there is none.
	gomod  bool   // Whether the repository is a Go module.
	force  bool   // Whether an origin that cannot be queried is ignored.
}

// choose returns the version to release: set when it is not nil, else the
// one [guard.ask] reads, proposing proposal. Either one must pass
// [guard.check]. A version whose core is lower than proposal is released with
// a warning on out.
func (grd guard) choose(
	ctx context.Context,
	out io.Writer,
	rdr *bufio.Reader,
	set *semver.Version,
	proposal *semver.Version,
) (*semver.Version, error) {

	ver := set
	var err error
	if ver != nil {
		_, _ = fmt.Fprintf(out, "Version: %s\n", ver.Original())
		err = grd.check(ctx, ver)
	} else {
		ver, err = grd.ask(ctx, out, rdr, proposal)
	}
	if err != nil {
		return nil, err
	}

	// Only the cores are compared, so a pre-release of the proposed release
	// does not count as lower.
	core := semver.New(ver.Major(), ver.Minor(), ver.Patch(), "", "")
	if core.LessThan(proposal) {
		format := "Warning: %s is lower than the proposed %s.\n"
		_, _ = fmt.Fprintf(out, format, ver.Original(), proposal.Original())
	}
	return ver, nil
}

// ask prompts on out for the version to release, proposing def, and reads
// answers from rdr until one passes [guard.check]; an empty answer takes def.
// A version that is invalid, not newer, or already a tag is reported on out
// and asked for again. Any other failure, and running out of input, ends it.
func (grd guard) ask(
	ctx context.Context,
	out io.Writer,
	rdr *bufio.Reader,
	def *semver.Version,
) (*semver.Version, error) {

	for {
		format := "Enter a version number [%s]: "
		_, _ = fmt.Fprintf(out, format, def.Original())
		txt, err := readLine(rdr)
		if err != nil {
			return nil, fmt.Errorf("read version input: %w", err)
		}
		// Only the line ending goes: a version padded with spaces is not
		// one, and is refused like any other.
		ver := def
		if txt = strings.TrimRight(txt, "\r\n"); txt != "" {
			ver, err = parseVersion(txt, grd.gomod)
		}
		if err == nil {
			if err = grd.check(ctx, ver); err == nil {
				return ver, nil
			}
		}
		if !errors.Is(err, ErrBadVersion) &&
			!errors.Is(err, ErrNotNewer) &&
			!errors.Is(err, ErrTagExists) {

			return nil, err
		}
		_, _ = fmt.Fprintf(out, "Rejected: %s\n", err)
	}
}

// check returns nil when ver may be released: it is newer than the current
// tag and names no tag yet, locally or on origin. It returns [ErrNotNewer],
// [ErrTagExists], or - unless forced - an error wrapping [gitaid.ErrRemote]
// when origin cannot be queried.
func (grd guard) check(ctx context.Context, ver *semver.Version) error {
	if cur, err := semver.NewVersion(grd.cur); err == nil &&
		!ver.GreaterThan(cur) {

		format := "%w: %s is not newer than %s"
		return fmt.Errorf(format, ErrNotNewer, ver.Original(), grd.cur)
	}

	// The current tag is only the closest one reachable from HEAD; the tag
	// may still exist on another branch.
	tag := ver.Original()
	has, err := gitaid.HasTag(ctx, grd.repo, tag)
	if err != nil {
		return fmt.Errorf("check tags: %w", err)
	}
	if has {
		return fmt.Errorf("%w: %s", ErrTagExists, tag)
	}

	// A tag only someone else has pushed is invisible locally.
	if grd.origin == "" {
		return nil
	}
	has, err = gitaid.HasRemoteTag(ctx, grd.repo, "origin", tag)
	if err != nil {
		if grd.force && errors.Is(err, gitaid.ErrRemote) {
			return nil
		}
		return fmt.Errorf("check origin tags (--force skips): %w", err)
	}
	if has {
		return fmt.Errorf("%w on origin: %s", ErrTagExists, tag)
	}
	return nil
}

// parseVersion parses txt as a full semantic version with an optional "v"
// prefix and returns it in the canonical "v"-prefixed form Go modules require.
// Partial versions and leading zeros are refused, and so is build metadata
// when gomod is true, because Go cannot fetch a module version carrying it.
// Every refusal wraps [ErrBadVersion].
func parseVersion(txt string, gomod bool) (*semver.Version, error) {
	ver, err := semver.StrictNewVersion(strings.TrimPrefix(txt, "v"))
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrBadVersion, txt, err)
	}
	if gomod && ver.Metadata() != "" {
		format := "%w: %q: build metadata in a Go module"
		return nil, fmt.Errorf(format, ErrBadVersion, txt)
	}
	return semver.MustParse("v" + ver.String()), nil
}

// writeChangelog prepends a release of the given version, dated now and built
// from changes, to the CHANGELOG.md of the repository rooted at repo. The file
// is created when it does not exist yet.
func writeChangelog(repo string, next *semver.Version, changes []string) error {
	rel := gmclog.NewSemVerRelease(next, time.Now())
	rel.AddChange(changes...)

	pth := filepath.Join(repo, "CHANGELOG.md")
	if err := gmclog.CreateFile(pth); err != nil {
		return fmt.Errorf("create changelog: %w", err)
	}
	clg, err := gmclog.ReadChangelog(pth)
	if err != nil {
		return fmt.Errorf("read changelog: %w", err)
	}
	clg.AddRelease(rel)
	if err = clg.Save(); err != nil {
		return fmt.Errorf("save changelog: %w", err)
	}
	return nil
}

// collectChanges returns the changes to release since tag, and the tag they
// are measured from. That tag is the empty string when the repository carries
// no version tag yet, and every commit is then a change.
func collectChanges(
	ctx context.Context,
	repo string,
	tag string,
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

// approveBranch reads the answer to the question whether to release from
// branch and returns nil when it is approved: only "y" or "yes", in any case,
// goes on, and any other answer yields an error wrapping [ErrNotDefBranch].
func approveBranch(rdr *bufio.Reader, branch string) error {
	txt, err := readLine(rdr)
	if err != nil {
		return fmt.Errorf("read approval input: %w", err)
	}
	if ans := strings.ToLower(strings.TrimSpace(txt)); ans == "y" ||
		ans == "yes" {

		return nil
	}
	format := "bump aborted: branch %q is %w (master, main)"
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

// setVersion returns the version the --set flag gives, parsed by
// [parseVersion], or nil when the flag is not given. It returns
// [ErrBumpFlags] when bump, a forced bump level, is not empty as well.
func setVersion(
	fs *xflag.FlagSet,
	bump string,
	gomod bool,
) (*semver.Version, error) {

	if !fs.WasSet("set") {
		return nil, nil
	}
	if bump != "" {
		return nil, fmt.Errorf("%w: --set and --%s", ErrBumpFlags, bump)
	}
	return parseVersion(fs.GetString("set"), gomod)
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
