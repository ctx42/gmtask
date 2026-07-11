// Package gmbump provides the gomake ":bump" target for releasing Go projects.
//
// The target proposes the next semantic version from the repository's latest
// tag (minor bump by default, patch with -p), lets you confirm or override it,
// prepends a CHANGELOG.md entry built from the commits since that tag, writes
// the version to a VER file, then commits, tags, and pushes to origin.
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

// StartSemVer represents the first semantic version used to tag the repo.
const StartSemVer = "v0.0.0"

var _ = semver.MustParse(StartSemVer) // Ensure StartSemVer is a valid semver.

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
func BumpTarget(ctx context.Context, rng *ring.Ring, repo string) error {
	tgtName := ":bump"
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.SetOutput(rng.Stderr())
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s:\n", tgtName)
		_, _ = fmt.Fprint(rng.Stderr(), head+xflag.HelpOptions(fs))
	}
	fs.BoolSL("help", "h", false, "show help")
	fs.BoolSL("patch", "p", false, "bump patch version")
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fs.Args())
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}

	clean, err := gitaid.IsClean(ctx, repo)
	if err != nil {
		return err
	}
	if !clean {
		return gitaid.ErrNotClean
	}

	curr, next, skipped, err := getSemVer(ctx, repo, "", fs.GetBool("patch"))
	if err != nil {
		return err
	}
	for _, tag := range skipped {
		_, _ = fmt.Fprintf(rng.Stdout(), "Skipping tag: %q\n", tag)
	}
	var curStr string // String representation of the current semantic version.
	if curr != nil {
		curStr = curr.Original()
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "Current tag: %s\n", curStr)

	changes, err := gitaid.ChangeLog(ctx, repo, curStr)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		_, _ = fmt.Fprint(rng.Stdout(), "HEAD on tag. Nothing to do.\n")
		return nil
	}

	_, _ = fmt.Fprintf(
		rng.Stdout(),
		"Enter a version number [%s]: ",
		next.Original(),
	)
	rdr := bufio.NewReader(rng.Stdin())
	txt, err := rdr.ReadString('\n')
	if err != nil {
		return err
	}
	if txt = strings.TrimSpace(txt); txt != "" {
		ver, err := semver.NewVersion(txt)
		if err != nil {
			return err
		}
		next = ver
	}

	// We always use "v" prefix.
	nextStr := next.Original()
	if len(nextStr) > 0 && nextStr[0] != 'v' {
		// At this point we are sure next is valid
		// we checked that during input validation.
		next = semver.MustParse("v" + nextStr)
	}

	rel := gmclog.NewSemVerRelease(next, time.Now())
	rel.AddChange(changes...)
	pth := filepath.Join(repo, "CHANGELOG.md")
	if err = gmclog.CreateFile(pth); err != nil {
		return err
	}
	cl, err := gmclog.ReadChangelog(pth)
	if err != nil {
		return err
	}
	cl.AddRelease(rel)
	if err = cl.Save(); err != nil {
		return err
	}

	msg := "Now you may edit CHANGELOG.md. Then press ENTER to continue.\n"
	_, _ = fmt.Fprint(rng.Stdout(), msg)
	if _, err = rdr.ReadString('\n'); err != nil {
		return err
	}
	_, _ = fmt.Fprint(rng.Stdout(), "Continuing.\n")

	pth = filepath.Join(repo, "VER")
	if err = os.WriteFile(pth, []byte(next.Original()), 0o600); err != nil {
		return err
	}

	if err = gitaid.Add(ctx, repo, "CHANGELOG.md", "VER"); err != nil {
		return err
	}

	cm := fmt.Sprintf("Bump version to %s.", next)
	if err = gitaid.Commit(ctx, repo, cm); err != nil {
		return err
	}

	tm := fmt.Sprintf("Tag version %s.", next)
	if err = gitaid.Tag(ctx, repo, next.Original(), tm); err != nil {
		return err
	}

	if err = gitaid.Push(ctx, repo); err != nil {
		if !errors.Is(err, gitaid.ErrNoRemote) {
			return err
		}
	}

	_, _ = fmt.Fprint(rng.Stdout(), "Done.\n")

	// Print additional info if this is a Go module.
	mod, err := gmgo.ImpPath(ctx, rng, repo)
	if err != nil {
		if errors.Is(err, gomake.ErrNoGoMod) {
			return nil
		}
		return err
	}
	msg = "\nUse\n\tgo get %s@%s\nto update upstreams.\n"
	_, _ = fmt.Fprintf(rng.Stdout(), msg, mod, next.Original())

	return nil
}

// getSemVer proposes the next semver tag based on current repository status. It
// returns the current SemVer tag (nil if none), the proposal for the next
// SemVer, and the tags skipped because they are not valid semantic versions. In
// most cases you want to set startRev to the empty string, but to start
// searching from a specific revision set startRev.
//
// When bumpPatch is true the patch version is increased instead of the minor.
func getSemVer(
	ctx context.Context,
	repo string,
	startRev string,
	bumpPatch bool,
) (*semver.Version, *semver.Version, []string, error) {

	rev, err := gitaid.ClosestTag(ctx, repo, startRev)
	if err != nil {
		return nil, nil, nil, err
	}
	if rev == "" {
		return nil, semver.MustParse(StartSemVer), nil, nil
	}

	ver, err := semver.NewVersion(rev)
	if err != nil {
		if rev == startRev {
			// The [gitaid.ClosestTag] returned the same rev as the startRev
			// which means this is the only (not valid semver) tagged commit
			// in the repository.
			return nil, semver.MustParse(StartSemVer), nil, nil
		}
		curr, next, skipped, err := getSemVer(ctx, repo, rev, bumpPatch)
		return curr, next, append([]string{rev}, skipped...), err
	}

	var ret semver.Version
	if bumpPatch {
		ret = ver.IncPatch()
	} else {
		ret = ver.IncMinor()
	}
	return ver, &ret, nil, nil
}
