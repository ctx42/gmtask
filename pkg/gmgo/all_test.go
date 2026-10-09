// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/xdef/pkg/xdef"
)

// TestMain clears the environment variables the targets read, so a value the
// host exports cannot change what a test sees: rings created by ringtest start
// from the process environment.
func TestMain(m *testing.M) {
	keys := []string{
		BuildIDEnvKey,
		EnvBldBump,
		GoLintConfigForceEnvKey,
		GoLintConfigRepoEnvKey,
		GoTestTimeoutEnvKey,
		xdef.EnvBldDate,
	}
	for _, key := range keys {
		if err := os.Unsetenv(key); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

// setupConfigRepo creates a local git repository on the "master" branch
// holding the shared configuration and returns its path. The config file is
// named ".golangci.yml" unless name overrides it. Tests point [Lint.Config] at
// it through the [GoLintConfigRepoEnvKey] environment variable so linting
// configuration downloads without network access.
func setupConfigRepo(t tester.T, name ...string) string {
	t.Helper()
	file := ".golangci.yml"
	if len(name) > 0 {
		file = name[0]
	}
	repo := t.TempDir()
	cfg := oskit.ReadFile(t, "testdata/golangci.yml")
	oskit.Write(t, cfg, repo, file)

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	git("-c", "init.defaultBranch=master", "init")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test User")
	git("add", "-A")
	git("commit", "-m", "config")
	return repo
}

// pathWithoutBinary returns PATH with directories that contain name removed, so
// tests can force LookPath / exec to miss an otherwise-installed tool.
func pathWithoutBinary(name string) string {
	var keep []string
	// The process PATH, deliberately: exec resolves binaries on it.
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			continue
		}
		keep = append(keep, dir)
	}
	return strings.Join(keep, string(os.PathListSeparator))
}
