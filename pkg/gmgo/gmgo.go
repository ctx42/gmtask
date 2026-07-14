// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmgo provides gomake targets and helpers for Go projects: vetting,
// linting, testing, building, and serving documentation.
package gmgo

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"
	"github.com/ctx42/xflag/pkg/xflag"
)

// ErrConfig is returned when a module with an entry in the target's "modules"
// configuration omits its package.
var ErrConfig = errors.New("invalid target configuration")

// expLintVer is the minimum expected version of golangci-lint.
var expLintVer = semver.MustParse("v2.12.2")

// goDevRepo represents the git repository hosting the shared golangci-lint
// configuration.
const goDevRepo = "git@github.com:ctx42/xdev.git"

// Environment variable keys recognized by gmgo targets.
const (
	// GoTestTimeoutEnvKey is the environment variable overriding the
	// per-invocation `go test` timeout. When set, its value is parsed as a Go
	// duration (e.g. "60s", "5m") and takes precedence over the configured
	// timeout. Unset leaves the go tool default in effect.
	GoTestTimeoutEnvKey = "GOMAKE_GO_TEST_TIMEOUT"

	// GoLintConfigForceEnvKey is the environment variable that, when set to any
	// non-empty value, forces a fresh download of the shared golangci-lint
	// config even if a local .golangci.yml already exists.
	GoLintConfigForceEnvKey = "GOMAKE_GOLINT_CONFIG_FORCE"

	// GoLintConfigRepoEnvKey is the environment variable overriding the git
	// repository the shared golangci-lint config is fetched from. Unset
	// defaults to [goDevRepo].
	GoLintConfigRepoEnvKey = "GOMAKE_GOLINT_CONFIG_REPO"

	// BuildIDEnvKey is the environment variable carrying the CI/CD build
	// identifier. When set, its value is appended to generated log filenames
	// (see [CovLogFilename], [TestLogFilename]) to keep per-build reports
	// distinct.
	BuildIDEnvKey = "BUILD_ID"
)

// Go collects Go related targets.
type Go struct{} //gomake:ns_root

// Vet vets Go code in current directory and its subdirectories.
//
// Example usage:
//
//	gomake :go:vet
func (Go) Vet(ctx context.Context, rng *ring.Ring) error {
	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Stdout, cmd.Stderr = rng.Stdout(), rng.Stderr()
	cmd.Env = rng.EnvAll()
	return cmd.Run()
}

// Check runs :go:vet, :go:lint, :go:test in order.
//
// Example usage:
//
//	gomake :go:check
func (tgt Go) Check(ctx context.Context, rng *ring.Ring) error {
	if err := tgt.Vet(ctx, rng); err != nil {
		return err
	}
	// gomake delivers :go:check the "go" node block ({timeout}). The lint step
	// reads only its own "version" and "file" keys and ignores the sibling
	// "timeout", so the whole block passes through unchanged to Test, which
	// honours the configured timeout.
	if err := (Lint{}).Default(ctx, rng); err != nil {
		return err
	}
	return tgt.Test(ctx, rng)
}

// TestV runs all Go tests in current working directory and its subdirectories
// in verbose mode. See documentation of [Go.test] method for more details.
//
// Example usage:
//
//	gomake :go:test-v
func (tgt Go) TestV(ctx context.Context, rng *ring.Ring) error {
	return tgt.test(ctx, rng, true)
}

// Test runs all Go tests in current working directory and its subdirectories.
// See documentation of [Go.test] method for more details.
//
// Example usage:
//
//	gomake :go:test
func (tgt Go) Test(ctx context.Context, rng *ring.Ring) error {
	return tgt.test(ctx, rng, false)
}

// test runs all Go tests with coverage report and race detector in current
// working directory and its subdirectories.
//
// By default, race detector is turned on and two report files are created:
//   - go_test_coverage.log with coverage log
//   - go_test_run.log with copy of what "go test" printed out
//
// If the program is run in CI/CD context the files will have BUILD_ID as
// part of their name - see [CovLogFilename] and [TestLogFilename] for details.
//
// The files are put in current working directory, or "tmp" subdirectory if it
// exists. The destination directory is customizable with target arguments.
//
// When verbose is set to true an additional flag "-v" is passed to "go test".
//
// The additional arguments may be passed to the "go test" with "-- arg0 arg1"
// construct.
//
//nolint:cyclop
func (Go) test(ctx context.Context, rng *ring.Ring, verbose bool) error {
	tgtName := ":go:test"
	if verbose {
		tgtName += "-v"
	}

	dirHelp := `directory to put reports to (default: "." or "tmp" if exists)`
	out, rng, help, err := parseDirTarget(rng, tgtName, dirHelp)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	args := rng.Args()

	if out == "" && gomake.DirExists("tmp") {
		out = "tmp"
	}

	covPth := filepath.Join(out, CovLogFilename(rng))
	repPth := filepath.Join(out, TestLogFilename(rng))

	cmdArgs := []string{"test"}
	if verbose {
		cmdArgs = append(cmdArgs, "-v")
	}
	cmdArgs = append(
		cmdArgs,
		"-race",
		"-coverprofile="+covPth,
		"-count=1",
	)
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}

	cfg, err := gomake.TargetConfig(rng)
	if err != nil {
		return err
	}
	timeout, err := gomake.GetCfgDefault[time.Duration](cfg, "timeout", 0)
	if err != nil {
		return err
	}
	if env := rng.EnvGet(GoTestTimeoutEnvKey); env != "" {
		if timeout, err = time.ParseDuration(env); err != nil {
			return fmt.Errorf("gmgo: invalid %s: %w", GoTestTimeoutEnvKey, err)
		}
	}
	if timeout > 0 {
		cmdArgs = append(cmdArgs, "-timeout="+timeout.String())
	}
	cmdArgs = append(cmdArgs, "./...")

	// Create the report only after config and timeout resolution so an
	// invalid config fails before a stray empty report is written to disk.
	rep, err := os.Create(repPth) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = rep.Close() }()
	mw := io.MultiWriter(rng.Stdout(), rep)

	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Stdout, cmd.Stderr = mw, rng.Stderr()
	cmd.Env = rng.EnvAll()
	return cmd.Run()
}

// parseDirTarget parses the --dir and --help flags shared by the targets that
// write output to a directory, using dirHelp as the --dir usage description. It
// returns the resolved --dir value and the ring with the remaining positional
// arguments applied. help is true when --help was requested, in which case the
// usage was already written and the caller should return without further work.
func parseDirTarget(
	rng *ring.Ring,
	tgtName, dirHelp string,
) (out string, _ *ring.Ring, help bool, err error) {

	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.BoolSL("help", "h", false, "show help")
	fs.StringVar(&out, "dir", "", dirHelp)
	fs.SetOutput(rng.Stderr())
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s\n", tgtName)
		_, _ = fmt.Fprint(rng.Stderr(), head+xflag.HelpOptions(fs))
	}
	if err = fs.Parse(rng.Args()); err != nil {
		return "", rng, false, err
	}
	rng = rng.SetArgs(fs.Args())
	if fs.GetBool("help") {
		fs.Usage()
		return "", rng, true, nil
	}
	return out, rng, false, nil
}

// Doc starts the godoc documentation engine service and opens it in default
// browser for the package in the current working directory. The server binds
// an OS-assigned free port, so concurrent Doc runs do not collide.
//
// Example usage:
//
//	gomake :go:doc
func (Go) Doc(ctx context.Context, rng *ring.Ring) error {
	// godoc serves package documentation under the "/pkg/" path prefix.
	return serveDocServer(ctx, rng, serveDoc, "/pkg/")
}

// Pkgsite starts the pkgsite documentation server and opens it in default
// browser for the package in the current working directory. The server binds
// an OS-assigned free port, so concurrent Pkgsite runs do not collide.
//
// Example usage:
//
//	gomake :go:pkgsite
func (Go) Pkgsite(ctx context.Context, rng *ring.Ring) error {
	// pkgsite serves package documentation directly under the import path.
	return serveDocServer(ctx, rng, servePkgsite, "/")
}

// serveDocServer reserves a free loopback port, starts a documentation server
// on it with serve, waits for the server to accept connections, and opens the
// default browser at the current package's import path joined to the server
// address under pkgPrefix. It returns serve's error, or a module-resolution
// error when the working directory is not a Go module.
func serveDocServer(
	ctx context.Context,
	rng *ring.Ring,
	serve func(ctx context.Context, rng *ring.Ring, addr string) error,
	pkgPrefix string,
) error {

	ctx, cxl := context.WithCancel(ctx)
	defer cxl()

	imPath, err := ImpPath(ctx, rng, "")
	if err != nil {
		return err
	}

	addr, err := freeAddr()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var srvErr error
	go func() {
		defer wg.Done()
		defer cxl()
		srvErr = serve(ctx, rng, addr)
	}()

	go func() {
		defer wg.Done()
		base := "http://" + addr
		if !waitForServer(ctx, base+"/") {
			return
		}
		name, args := browserCmd(runtime.GOOS, base+pkgPrefix+imPath)
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdout, cmd.Stderr = rng.Stdout(), rng.Stderr()
		// Opening the browser is best-effort: a background goroutine cannot
		// propagate an error and the server runs regardless, so ignore it.
		_ = cmd.Run()
	}()

	wg.Wait()
	return srvErr
}

// serveDoc runs the godoc documentation server bound to addr, streaming its
// output to rng, until ctx is cancelled. It returns an error only when godoc
// exits before ctx is done; a shutdown triggered by the caller cancelling ctx
// is not an error.
func serveDoc(ctx context.Context, rng *ring.Ring, addr string) error {
	args := []string{
		"-http=" + addr,
		"-play",
		"-index",
		"-notes=\"BUG|TODO|FIX\"",
	}
	cmd := exec.CommandContext(ctx, "godoc", args...)
	cmd.Stdout, cmd.Stderr = rng.Stdout(), rng.Stderr()
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// servePkgsite runs the pkgsite documentation server bound to addr, serving the
// module in the current working directory and streaming its output to rng,
// until ctx is cancelled. It returns an error only when pkgsite exits before
// ctx is done; a shutdown triggered by the caller cancelling ctx is not an
// error.
func servePkgsite(ctx context.Context, rng *ring.Ring, addr string) error {
	cmd := exec.CommandContext(ctx, "pkgsite", "-http="+addr)
	cmd.Stdout, cmd.Stderr = rng.Stdout(), rng.Stderr()
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// browserCmd returns the command name and arguments that open url in the
// default web browser on the given GOOS: "open" on macOS, "rundll32" on
// Windows, and "xdg-open" elsewhere (Linux and other Unix systems).
func browserCmd(goos, url string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}

	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}

	default:
		return "xdg-open", []string{url}
	}
}

// waitForServer polls url with HTTP HEAD requests until one succeeds,
// returning true once the server responds. It returns false when ctx is
// cancelled first, letting the caller abandon a server that never comes up.
func waitForServer(ctx context.Context, url string) bool {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()

	cli := &http.Client{Timeout: time.Second}
	for {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodHead,
			url,
			http.NoBody,
		)
		if err != nil {
			return false
		}
		resp, err := cli.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			return true
		}

		select {
		case <-ctx.Done():
			return false

		case <-tick.C:
		}
	}
}

// buildVarNames lists the canonical ldflags variable names go:build injects,
// in the order they are emitted. Each is defined in xdef so the names never
// drift from the ones gomake injects into its own binary.
var buildVarNames = []string{
	xdef.VarBuildDate,
	xdef.VarScmRev,
	xdef.VarScmHash,
	xdef.VarScmState,
	xdef.VarCCID,
}

// Build runs "go build", injecting build metadata via "-ldflags -X" when the
// current module has an entry in the target's "modules" configuration. Without
// a matching entry it builds normally and injects nothing. Extra arguments are
// forwarded to "go build".
//
// Example usage:
//
//	gomake :go:build cmd/main.go
func (Go) Build(ctx context.Context, rng *ring.Ring) error {
	cfg, err := gomake.TargetConfig(rng)
	if err != nil {
		return err
	}

	module, err := ImpPath(ctx, rng, "")
	if err != nil {
		return err
	}

	args := []string{"build"}
	base := "modules.'" + module + "'"
	if cfg.Has(base) {
		pkg, err := gomake.GetCfg[string](cfg, base+".package")
		if err != nil {
			return fmt.Errorf("%w: module %q: %w", ErrConfig, module, err)
		}
		vals := buildValues(ctx, rng)
		vars := make([]LDVar, 0, len(buildVarNames))
		for _, field := range buildVarNames {
			name, err := gomake.GetCfgDefault(cfg, base+".names."+field, field)
			if err != nil {
				return err
			}
			vars = append(vars, LDVar{Name: name, Value: vals[field]})
		}
		args = append(args, "-ldflags="+LDFlags(pkg, vars))
	}
	args = append(args, rng.Args()...)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = rng.EnvAll()
	cmd.Stdout = rng.Stdout()
	cmd.Stderr = rng.Stderr()

	format := "#gomake INFO# go %s\n"
	_, _ = fmt.Fprintf(cmd.Stderr, format, strings.Join(args, " "))
	return cmd.Run()
}

// buildValues collects the build-metadata values to inject, keyed by canonical
// field name. A missing source falls back to the matching xdef placeholder (or,
// for the build date, the current time), so a build outside a git work tree or
// without CI metadata still succeeds.
func buildValues(ctx context.Context, rng *ring.Ring) map[string]string {
	rev, revErr := gitaid.Describe(ctx, "")
	hash, hashErr := gitaid.LatestHash(ctx, "")
	state, stateErr := gitaid.WorkTreeStatus(ctx, "")

	buildDate := rfc3339Milli(time.Now())
	if val, ok := rng.EnvLookup(xdef.EnvImgCreated); ok && val != "" {
		if tim, err := time.Parse(time.RFC3339Nano, val); err == nil {
			buildDate = rfc3339Milli(tim)
		}
	}

	ccid := xdef.PhUnknown
	if val := rng.EnvGet(gomake.CCIDEnvKey); val != "" {
		ccid = val
	}

	return map[string]string{
		xdef.VarBuildDate: buildDate,
		xdef.VarScmRev:    gitOr(rev, revErr, xdef.PhRev),
		xdef.VarScmHash:   gitOr(hash, hashErr, xdef.PhHash),
		xdef.VarScmState:  gitOr(state, stateErr, xdef.PhUnknown),
		xdef.VarCCID:      ccid,
	}
}

// gitOr returns s when err is nil and s is non-empty, otherwise the placeholder
// ph.
func gitOr(s string, err error, ph string) string {
	if err != nil || s == "" {
		return ph
	}
	return s
}
