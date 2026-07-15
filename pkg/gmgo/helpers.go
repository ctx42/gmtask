// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
)

var (
	// ErrModInit is returned when Go module initialization failed.
	ErrModInit = errors.New("go module initialization error")

	// ErrImpPath is returned when ImpPath function failed.
	ErrImpPath = errors.New("cannot determine Go module import path")
)

// ImpPath returns Go import path for the package based on working directory.
// The empty string used for dir means current working directory.
func ImpPath(ctx context.Context, rng *ring.Ring, dir string) (string, error) {
	const format = "%w: %s: %s"

	sout, eout := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "go", "list", "-m")
	cmd.Env = rng.EnvAll()
	cmd.Stdout, cmd.Stderr = sout, eout
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		if dir == "" {
			dir, _ = os.Getwd()
		}
		return "", fmt.Errorf(format, ErrImpPath, dir, eout.String())
	}
	rsp := strings.TrimSpace(sout.String())
	if rsp == "command-line-arguments" {
		if dir == "" {
			dir, _ = os.Getwd()
		}
		return "", fmt.Errorf(format, gomake.ErrNoGoMod, dir, eout.String())
	}
	return rsp, nil
}

// InitModule initializes Go module with given name in directory dir. The empty
// string used for dir means current working directory. The "go" directive in
// the resulting go.mod is pinned to the toolchain's "major.minor" version
// (e.g. "go 1.26"), not its patch version (e.g. "go 1.26.3").
func InitModule(ctx context.Context, rng *ring.Ring, dir, name string) error {
	sout, eout := io.Discard, &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "go", "mod", "init", name)
	cmd.Env = rng.EnvAll()
	cmd.Stdout, cmd.Stderr = sout, eout
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		msg := eout.String()
		if msg != "" {
			return fmt.Errorf("%w: %s: %s", ErrModInit, dir, msg)
		}
		return fmt.Errorf("%w: %s", ErrModInit, dir)
	}
	return pinGoMajorMinor(ctx, rng, dir)
}

// rxGoDirective captures the version in a go.mod "go" directive, e.g. the
// "1.26.3" in "go 1.26.3".
var rxGoDirective = regexp.MustCompile(`(?m)^go (\d+\.\d+(?:\.\d+)?)`)

// pinGoMajorMinor rewrites the "go" directive in the go.mod located in dir to
// its "major.minor" form (e.g. "go 1.26" instead of "go 1.26.3"), so the
// project is not pinned to the patch version of the toolchain that created it.
// It is a no-op when the directive is already "major.minor" or absent.
func pinGoMajorMinor(ctx context.Context, rng *ring.Ring, dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrModInit, dir, err)
	}
	m := rxGoDirective.FindSubmatch(data)
	if m == nil {
		return nil
	}
	ver, err := semver.NewVersion(string(m[1]))
	if err != nil {
		// The go directive is not a version we can parse; leave it as is
		// rather than rewriting an unexpected value.
		return nil
	}
	mm := fmt.Sprintf("%d.%d", ver.Major(), ver.Minor())
	if mm == string(m[1]) {
		return nil
	}

	eout := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "go", "mod", "edit", "-go="+mm)
	cmd.Env = rng.EnvAll()
	cmd.Stdout, cmd.Stderr = io.Discard, eout
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s: %s", ErrModInit, dir, eout.String())
	}
	return nil
}

// LDVar is a single ldflags variable assignment: the target variable Name and
// the Value to inject into it.
type LDVar struct {
	Name  string // Variable name, e.g. "scmRev".
	Value string // Value to inject, e.g. "v1.2".
}

// LDFlags returns the "-X" linker flags injecting each variable into the
// package at pkgPath, e.g.:
//
//	-X 'example.com/app/version.scmRev=v1.2'
//
// Each assignment is single-quoted so "go build" receives one argument per
// flag. It returns an empty string when vars is empty.
func LDFlags(pkgPath string, vars []LDVar) string {
	parts := make([]string, 0, len(vars))
	for _, v := range vars {
		part := fmt.Sprintf("-X '%s.%s=%s'", pkgPath, v.Name, v.Value)
		parts = append(parts, part)
	}
	return strings.Join(parts, " ")
}

// CovLogFilename returns filename used to store a coverage report. When running
// in CI/CD context the value of the [BuildIDEnvKey] environment variable is
// added to the filename. Examples:
//
//	go_test_coverage.log
//	go_test_coverage_123.log
func CovLogFilename(rng *ring.Ring) string {
	stem := "go_test_coverage"
	if val := rng.EnvGet(BuildIDEnvKey); val != "" {
		stem += "_" + val
	}
	return stem + ".log"
}

// TestLogFilename returns filename used to store a test report. When running in
// CI/CD context the value of the [BuildIDEnvKey] environment variable is added
// to the filename. Examples:
//
//	go_test_run.log
//	go_test_run_123.log
func TestLogFilename(rng *ring.Ring) string {
	stem := "go_test_run"
	if val := rng.EnvGet(BuildIDEnvKey); val != "" {
		stem += "_" + val
	}
	return stem + ".log"
}

// gitGetFile downloads a single file from a git repository and writes it to
// dst. It shallow-clones repo at the given branch (or tag) into a temporary
// directory and copies src out of the working tree, so it works with hosts
// that reject "git archive --remote" such as GitHub. The empty string used for
// branch means the repository's default branch. When ctx has no deadline set,
// one of 60 seconds is applied.
func gitGetFile(ctx context.Context, repo, branch, src, dst string) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	// Fail before touching the network when the destination directory is
	// missing; the error wraps fs.ErrNotExist and names the offending path.
	if _, err := os.Stat(filepath.Dir(dst)); err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "gmgo-lint-config-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	args := []string{"clone", "--depth", "1", "--single-branch", "--no-tags"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repo, tmp)

	eout := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.Stderr = eout
	if err = cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		msg := strings.TrimSpace(eout.String())
		return fmt.Errorf("%w: git clone %s: %s", err, repo, msg)
	}

	data, err := os.ReadFile(filepath.Join(tmp, src)) //nolint:gosec
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600) //nolint:gosec
}

// freeAddr returns a loopback host:port address that is free to bind at the
// moment of the call. It binds port 0 so the OS picks an unused port, then
// closes the listener so the caller can bind the address — leaving a small
// window in which another process could claim the port first.
func freeAddr() (string, error) {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", fmt.Errorf("reserving free port: %w", err)
	}
	addr := lis.Addr().String()
	if err = lis.Close(); err != nil {
		return "", fmt.Errorf("releasing free port: %w", err)
	}
	return addr, nil
}

// rfc3339Milli formats t in UTC, truncated to millisecond precision, using the
// RFC 3339 layout.
func rfc3339Milli(t time.Time) string {
	return t.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano)
}

// extractGolangCiVersion extracts and parses semantic version from output given
// by "golangci-lint version" command. It returns [semver.ErrInvalidSemVer]
// on error.
func extractGolangCiVersion(line string) (*semver.Version, error) {
	var ver string
	if _, sub, ok := strings.Cut(line, "version "); ok {
		if sub, _, ok = strings.Cut(sub, " built "); ok {
			ver = sub
		}
	}
	return semver.NewVersion(ver)
}
