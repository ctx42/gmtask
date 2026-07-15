// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
)

// Lint collects Go linting targets.
type Lint Go

// Default lints Go code in current working directory and subdirectories.
// Every time it runs it creates a linting log file in "${dir}/tmp" directory.
// The name of the file depends on the context the linting is done - see
// [LintLogPth] for details.
//
// Example usage:
//
//	gomake :go:lint
func (tgt Lint) Default(ctx context.Context, rng *ring.Ring) error {
	ver, err := tgt.checkVersion(ctx, rng, "")
	// Install when the binary is missing/unusable or older than required,
	// then re-check so a failed install surfaces before linting.
	if err != nil || ver.Compare(expLintVer) < 0 {
		if err = tgt.Install(ctx, rng); err != nil {
			return err
		}
		if _, err = tgt.checkVersion(ctx, rng, ""); err != nil {
			return err
		}
	}
	if err = tgt.Config(ctx, rng); err != nil {
		return err
	}
	return tgt.lint(ctx, rng, "")
}

// checkVersion returns the current golangci-lint version or error if:
//
//   - "golangci-lint" is not found or cannot be run for some reason
//   - response from "golangci-lint version" cannot be parsed
//
// The empty string used for repo directory means current working directory.
func (Lint) checkVersion(
	ctx context.Context,
	rng *ring.Ring,
	repo string,
) (*semver.Version, error) {

	sout, eout := &bytes.Buffer{}, rng.Stderr()
	cmd := exec.CommandContext(ctx, "golangci-lint", "version")
	cmd.Env = rng.EnvAll()
	cmd.Stdout, cmd.Stderr = sout, eout
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return extractGolangCiVersion(sout.String())
}

// lint lints Go code in given directory and subdirectories using a
// configuration file located in "${dir}/tmp/.golangci.yml" (if it exists). The
// empty string used for dir means current working directory.
func (Lint) lint(ctx context.Context, rng *ring.Ring, dir string) error {
	args := []string{"run"}
	cfgPth := filepath.Join("tmp", ".golangci.yml")
	if !gomake.FileExists(cfgPth) {
		cfgPth = ".golangci.yml"
	}
	if gomake.FileExists(cfgPth) {
		args = append(args, "-c", cfgPth)
	}
	args = append(args, "./...")

	cmd := exec.CommandContext(ctx, "golangci-lint", args...)
	cmd.Env = append(rng.EnvAll(), "LOG_LEVEL=error")
	cmd.Stdout, cmd.Stderr = rng.Stdout(), rng.Stderr()
	cmd.Dir = dir
	return cmd.Run()
}

// Install installs the golangci-lint binary. It installs the latest release
// unless the target's "version" configuration pins a specific version (used
// verbatim as the "go install" module query, e.g. "v2.13.0").
//
// Example usage:
//
//	gomake :go:lint:install
func (Lint) Install(ctx context.Context, rng *ring.Ring) error {
	cfg, err := gomake.TargetConfig(rng)
	if err != nil {
		return err
	}
	ver, err := gomake.GetCfgDefault(cfg, "version", "")
	if err != nil {
		return err
	}
	if ver == "" {
		ver = "latest"
	}
	pkg := "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + ver
	cmd := exec.CommandContext(ctx, "go", "install", pkg)
	cmd.Env = append(rng.EnvAll(), "CGO_ENABLED=0")
	return cmd.Run()
}

// Config downloads the shared ".golangci.yml" configuration file to current
// working directory or to "tmp" if it exists. The destination directory is
// customizable with target arguments. If the execution context has no deadline
// set, it will be set to 60 seconds. You may force the config file download by
// setting the [GoLintConfigForceEnvKey] environment variable to 1. The source
// repository defaults to [goDevRepo] but may be overridden by setting the
// [GoLintConfigRepoEnvKey] environment variable. The config file name defaults
// to ".golangci.yml" but may be overridden by the target's "file"
// configuration, applied to both the fetched and the written file.
//
// The configuration is fetched with a shallow clone of the source repository
// (see [gitGetFile]) rather than "git archive --remote", which hosts such as
// GitHub reject.
//
// Example usage:
//
//	gomake :go:lint:config
func (Lint) Config(ctx context.Context, rng *ring.Ring) error {
	cfg, err := gomake.TargetConfig(rng)
	if err != nil {
		return err
	}
	cfgFile, err := gomake.GetCfgDefault(cfg, "file", ".golangci.yml")
	if err != nil {
		return err
	}

	tgtName := ":go:lint:config"
	dirHelp := "" +
		"directory to put lint config to " +
		"(default: \".\" or \"tmp\" if exists)"
	out, rng, help, err := parseDirTarget(rng, tgtName, dirHelp)
	if err != nil {
		return err
	}
	if help {
		return nil
	}

	if out == "" && gomake.DirExists("tmp") {
		out = "tmp"
	}

	dst := filepath.Join(out, cfgFile)
	if rng.EnvGet(GoLintConfigForceEnvKey) == "" {
		if _, err := os.Stat(dst); err == nil {
			return nil
		}
	}
	repo := goDevRepo
	if env := rng.EnvGet(GoLintConfigRepoEnvKey); env != "" {
		repo = env
	}
	return gitGetFile(ctx, repo, "master", cfgFile, dst)
}
