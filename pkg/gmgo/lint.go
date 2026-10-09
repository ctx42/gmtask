// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
)

// Lint collects Go linting targets.
type Lint Go

// Default fetches the shared config when needed, ensures golangci-lint is
// installed at the required version, and lints the current working directory
// and its subdirectories with that config. It takes the arguments of
// [Lint.Config]. Lint output is written to the ring streams.
//
// Example usage:
//
//	gomake :go:lint
func (tgt Lint) Default(ctx context.Context, rng *ring.Ring) error {
	// Resolve the config first: it validates the configuration and the
	// arguments before anything is installed, and returns on --help.
	cfgPth, help, err := tgt.config(ctx, rng)
	if err != nil || help {
		return err
	}
	cfg, err := gomake.TargetConfig(rng)
	if err != nil {
		return err
	}
	cfgVer, err := gomake.GetCfgDefault(cfg, "version", "")
	if err != nil {
		return err
	}
	req, err := requiredLintVer(cfgVer)
	if err != nil {
		return err
	}

	ver, err := tgt.checkVersion(ctx, rng, "")
	// Install when the binary is missing/unusable or older than required,
	// then re-check so a failed install surfaces before linting.
	if err != nil || ver.LessThan(req) {
		if err = tgt.Install(ctx, rng); err != nil {
			return err
		}
		if ver, err = tgt.checkVersion(ctx, rng, ""); err != nil {
			return err
		}
		if ver.LessThan(req) {
			format := "golangci-lint %s at %s is older than %s after " +
				"installing; another binary may shadow the installed one"
			return fmt.Errorf(format, ver, lintBin(rng), req)
		}
	}
	return tgt.lint(ctx, rng, "", cfgPth)
}

// requiredLintVer returns the lowest golangci-lint version [Lint.Default]
// accepts: the configured version cfgVer when it is a semantic version, else
// expLintVer. A configured version older than expLintVer is an error.
func requiredLintVer(cfgVer string) (*semver.Version, error) {
	ver, err := semver.NewVersion(cfgVer)
	if err != nil {
		// Not configured, "latest", or another module query such as a branch
		// name: there is no version to compare against.
		return expLintVer, nil
	}
	if ver.LessThan(expLintVer) {
		format := "configured golangci-lint %s is older than the required %s"
		return nil, fmt.Errorf(format, cfgVer, expLintVer.Original())
	}
	return ver, nil
}

// lintBin returns the path of the golangci-lint binary: the first one found on
// the ring's PATH, else in the directory "go install" puts it - $GOBIN when
// set, otherwise $GOPATH/bin ($HOME/go/bin by default). It returns the bare
// name when none is found, so running it fails with a "not found" error.
func lintBin(rng *ring.Ring) string {
	const name = "golangci-lint"
	dirs := filepath.SplitList(rng.EnvGet("PATH"))
	if bin := rng.EnvGet("GOBIN"); bin != "" {
		dirs = append(dirs, bin)
	} else {
		gopath := rng.EnvGet("GOPATH")
		if home := rng.EnvGet("HOME"); gopath == "" && home != "" {
			gopath = filepath.Join(home, "go")
		}
		for _, dir := range filepath.SplitList(gopath) {
			dirs = append(dirs, filepath.Join(dir, "bin"))
		}
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		pth := filepath.Join(dir, name)
		if inf, err := os.Stat(pth); err == nil && inf.Mode()&0o111 != 0 {
			return pth
		}
	}
	return name
}

// checkVersion returns the version of the golangci-lint binary lintBin finds
// or an error if:
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
	cmd := exec.CommandContext(ctx, lintBin(rng), "version")
	cmd.Env = rng.EnvAll()
	cmd.Stdout, cmd.Stderr = sout, eout
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return extractGolangCiVersion(sout.String())
}

// lint lints Go code in the given directory and its subdirectories using the
// configuration file at cfgPth or, when cfgPth is empty, the one located in
// "${dir}/tmp/.golangci.yml" or "${dir}/.golangci.yml" (if it exists). The
// empty string used for dir means current working directory.
func (Lint) lint(
	ctx context.Context,
	rng *ring.Ring,
	dir string,
	cfgPth string,
) error {

	args := []string{"run"}
	// Resolve the config under dir (not process CWD) so FileExists matches
	// cmd.Dir when dir is non-empty and not the process working directory.
	if cfgPth == "" {
		cfgPth = filepath.Join(dir, "tmp", ".golangci.yml")
		if !gomake.FileExists(cfgPth) {
			cfgPth = filepath.Join(dir, ".golangci.yml")
		}
	}
	if gomake.FileExists(cfgPth) {
		args = append(args, "-c", cfgPth)
	}
	args = append(args, "./...")

	cmd := exec.CommandContext(ctx, lintBin(rng), args...)
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
	cmd.Stdout, cmd.Stderr = rng.Stdout(), rng.Stderr()
	return cmd.Run()
}

// Config downloads the shared ".golangci.yml" configuration file to the current
// working directory or to "tmp" if it exists. The destination directory is
// customizable with target arguments. If the execution context has no deadline
// set, it will be set to 60 seconds. You may force the config file download by
// setting the [GoLintConfigForceEnvKey] environment variable to a true value
// such as "1" or "true"; a value that is not a boolean is an error. The source
// repository defaults to goDevRepo but may be overridden by the target's
// "repo" configuration, or, taking precedence over both, the
// [GoLintConfigRepoEnvKey] environment variable. The config file name defaults
// to ".golangci.yml" but may be overridden by the target's "file"
// configuration, applied to both the fetched and the written file.
//
// The configuration is fetched with a shallow clone of the source repository
// (see gitGetFile) rather than "git archive --remote", which hosts such as
// GitHub reject.
//
// Example usage:
//
//	gomake :go:lint:config
func (tgt Lint) Config(ctx context.Context, rng *ring.Ring) error {
	_, _, err := tgt.config(ctx, rng)
	return err
}

// config does the work of [Lint.Config] and returns the path of the config
// file in place. help is true when --help was requested, in which case the
// usage was written and nothing else was done.
func (Lint) config(
	ctx context.Context,
	rng *ring.Ring,
) (cfgPth string, help bool, err error) {

	cfg, err := gomake.TargetConfig(rng)
	if err != nil {
		return "", false, err
	}
	cfgFile, err := gomake.GetCfgDefault(cfg, "file", ".golangci.yml")
	if err != nil {
		return "", false, err
	}
	repo, err := gomake.GetCfgDefault(cfg, "repo", goDevRepo)
	if err != nil {
		return "", false, err
	}
	var force bool
	if env := rng.EnvGet(GoLintConfigForceEnvKey); env != "" {
		if force, err = strconv.ParseBool(env); err != nil {
			key := GoLintConfigForceEnvKey
			return "", false, fmt.Errorf("invalid %s: %q: %w", key, env, err)
		}
	}

	tgtName := ":go:lint:config"
	dirHelp := "" +
		"directory to put lint config to " +
		"(default: \".\" or \"tmp\" if exists)"
	out, rng, usage, err := parseDirTarget(rng, tgtName, dirHelp)
	if err != nil {
		return "", false, err
	}
	if usage != "" {
		_, _ = fmt.Fprint(rng.Stderr(), usage)
		return "", true, nil
	}

	if out == "" && gomake.DirExists("tmp") {
		out = "tmp"
	}

	dst := filepath.Join(out, cfgFile)
	if !force {
		if _, err := os.Stat(dst); err == nil {
			format := "#gomake INFO# lint config: using %s\n"
			_, _ = fmt.Fprintf(rng.Stderr(), format, dst)
			return dst, false, nil
		}
	}
	if env := rng.EnvGet(GoLintConfigRepoEnvKey); env != "" {
		repo = env
	}
	format := "#gomake INFO# lint config: downloading from %s to %s\n"
	_, _ = fmt.Fprintf(rng.Stderr(), format, repo, dst)
	if err = gitGetFile(ctx, rng, repo, "", cfgFile, dst); err != nil {
		return "", false, err
	}
	return dst, false, nil
}
