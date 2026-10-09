// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"context"
	"regexp"
	"strings"

	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/ring/pkg/ring"
)

// breakRx matches a Conventional Commits subject marking a breaking change
// with "!" after the type.
var breakRx = regexp.MustCompile(`(?m)^[a-z]+(\([^)]*\))?!:`)

// breakFooterRx matches a "BREAKING CHANGE:" footer line.
var breakFooterRx = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE:`)

// featRx matches a Conventional Commits subject introducing a feature.
var featRx = regexp.MustCompile(`(?m)^feat(\([^)]*\))?:`)

// ProjectVersion returns the version of the project at repo, as
// [gitaid.Derive] builds it.
//
// The bump is the one given, or [EnvBldBump] when rng carries it, or the one
// read off the Conventional Commits since the last version tag - in that
// order, so a caller that knows the level does not pay for the scan.
//
// Deriving it here rather than at each call site is what keeps the version a
// binary reports, the one its image carries, and the one ":bump" offers to
// tag the same string.
func ProjectVersion(
	ctx context.Context,
	rng *ring.Ring,
	repo string,
	bump string,
) (gitaid.Version, error) {

	if bump == "" {
		bump = rng.EnvGet(EnvBldBump)
	}
	if bump == "" {
		var err error
		if bump, err = InferBump(ctx, repo); err != nil {
			return gitaid.Version{}, err
		}
	}
	return gitaid.Derive(ctx, repo, bump)
}

// InferBump reads the bump level off the Conventional Commits made since the
// last version tag in repo. The highest match wins, and anything it does not
// recognize falls back to [gitaid.BumpPatch] - an under-guess is merely
// imprecise where an over-guess publishes a development version outranking a
// real release.
func InferBump(ctx context.Context, repo string) (string, error) {
	tag, err := gitaid.Describe(ctx, repo, gitaid.WithMatch(gitaid.MatchSemVer))
	if err != nil {
		return "", err
	}

	// A tag git does not know is the synthetic one Describe falls back to
	// when no tag is a version, so every commit counts as made since it.
	msgs, err := gitaid.Messages(ctx, repo, tagOf(tag)+"..HEAD")
	if err != nil {
		if msgs, err = gitaid.Messages(ctx, repo, ""); err != nil {
			return "", err
		}
	}

	bump := gitaid.BumpPatch
	for _, msg := range msgs {
		subject, body, _ := strings.Cut(msg, "\n")
		switch {
		case breakRx.MatchString(subject) || breakFooterRx.MatchString(body):
			// The "!" marker is only valid in the subject and the footer only
			// in the body, so neither is scanned against the whole message.
			return gitaid.BumpMajor, nil

		case featRx.MatchString(subject):
			bump = gitaid.BumpMinor
		}
	}
	return bump, nil
}

// tagOf returns the tag part of what [gitaid.Describe] returned, following
// the recipe its documentation gives: strip the dirty marker, then split from
// the right, because a tag may itself hold "-" and even "-g".
func tagOf(desc string) string {
	desc = strings.TrimSuffix(desc, "-"+gitaid.StateDirty)
	idx := strings.LastIndex(desc, "-g")
	if idx < 0 {
		return desc
	}
	hash, rest := desc[idx+2:], desc[:idx]
	if !gitaid.IsHash(hash) {
		return desc
	}
	if idx = strings.LastIndex(rest, "-"); idx < 0 {
		return desc
	}
	if cnt := rest[idx+1:]; cnt == "" ||
		strings.TrimLeft(cnt, "0123456789") != "" {

		return desc
	}
	return rest[:idx]
}
