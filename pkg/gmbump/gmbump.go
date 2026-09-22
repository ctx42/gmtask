// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmbump provides the gomake ":bump" target for releasing Go projects.
//
// The target proposes the release the repository is already heading towards -
// the same one [gitaid.Derive] stamps into development builds, so the tag cut
// here can never name a different release than those builds pointed at. The
// bump comes from the Conventional Commits since the latest version tag, or
// from -p to force a patch. You confirm or override the proposal, and it then
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
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"

	"github.com/ctx42/gmtask/pkg/gmgo"
	"github.com/ctx42/gmtask/pkg/lib/gmclog"
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
//nolint:cyclop
func BumpTarget(ctx context.Context, rng *ring.Ring, repo string) error {
	tgtName := ":bump"
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.SetOutput(rng.Stderr())
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s:\n", tgtName)
		_, _ = fmt.Fprint(rng.Stderr(), head+xflag.HelpOptions(fs))
	}
	fs.BoolSL("help", "h", false, "show help")
	fs.BoolSL("patch", "p", false, "force a patch version bump")
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
		return fmt.Errorf("check repository state: %w", err)
	}
	if !clean {
		return gitaid.ErrNotClean
	}

	// The same derivation every other target uses, so the tag this offers to
	// cut is the release the development builds were already heading to.
	var bump string
	if fs.GetBool("patch") {
		bump = gitaid.BumpPatch
	}
	ver, err := gmgo.ProjectVersion(ctx, rng, repo, bump)
	if err != nil {
		return fmt.Errorf("resolve version: %w", err)
	}
	next, err := nextRelease(ver)
	if err != nil {
		return fmt.Errorf("resolve version: %w", err)
	}

	if closest, err := gitaid.ClosestTag(ctx, repo, ""); err == nil &&
		closest != "" && closest != ver.Tag {

		// Only version tags are considered, so say which one was passed
		// over rather than leave the proposal looking wrong.
		_, _ = fmt.Fprintf(rng.Stdout(), "Skipping tag: %q\n", closest)
	}

	curStr := ver.Tag
	changes, err := gitaid.ChangeLog(ctx, repo, curStr)
	if errors.Is(err, gitaid.ErrUnkTag) {
		// The tag git does not know is the synthetic base gitaid falls back
		// to when the repository has no version tag. Every commit is then a
		// change, and there is no current tag to report.
		curStr = ""
		changes, err = gitaid.ChangeLog(ctx, repo, "")
	}
	if err != nil {
		return fmt.Errorf("read changelog: %w", err)
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "Current tag: %s\n", curStr)

	if len(changes) == 0 {
		_, _ = fmt.Fprint(rng.Stdout(), "HEAD on tag. Nothing to do.\n")
		return nil
	}

	format := "Enter a version number [%s]: "
	_, _ = fmt.Fprintf(rng.Stdout(), format, next.Original())
	rdr := bufio.NewReader(rng.Stdin())
	txt, err := rdr.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read version input: %w", err)
	}
	if txt = strings.TrimSpace(txt); txt != "" {
		if next, err = semver.NewVersion(txt); err != nil {
			return fmt.Errorf("parse version %q: %w", txt, err)
		}
	}

	// Always use a "v" prefix. User input was already validated as a semver
	// string above when non-empty.
	nextStr := next.Original()
	if nextStr != "" && nextStr[0] != 'v' {
		next = semver.MustParse("v" + nextStr)
	}

	rel := gmclog.NewSemVerRelease(next, time.Now())
	rel.AddChange(changes...)
	pth := filepath.Join(repo, "CHANGELOG.md")
	if err = gmclog.CreateFile(pth); err != nil {
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

	msg := "Now you may edit CHANGELOG.md. Then press ENTER to continue.\n"
	_, _ = fmt.Fprint(rng.Stdout(), msg)
	if _, err = rdr.ReadString('\n'); err != nil {
		return fmt.Errorf("read continue input: %w", err)
	}
	_, _ = fmt.Fprint(rng.Stdout(), "Continuing.\n")

	pth = filepath.Join(repo, "VER")
	if err = os.WriteFile(pth, []byte(next.Original()), 0o600); err != nil {
		return fmt.Errorf("write VER file: %w", err)
	}

	if err = gitaid.Add(ctx, repo, "CHANGELOG.md", "VER"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	cm := fmt.Sprintf("Bump version to %s.", next.Original())
	if err = gitaid.Commit(ctx, repo, cm); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	tm := fmt.Sprintf("Tag version %s.", next.Original())
	if err = gitaid.Tag(ctx, repo, next.Original(), tm); err != nil {
		return fmt.Errorf("git tag: %w", err)
	}

	if err = gitaid.Push(ctx, repo); err != nil {
		if !errors.Is(err, gitaid.ErrNoRemote) {
			return fmt.Errorf("git push: %w", err)
		}
		_, _ = fmt.Fprint(rng.Stdout(), "No remote configured; skip push.\n")
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
