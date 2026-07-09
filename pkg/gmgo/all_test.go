package gmgo

import (
	"os/exec"

	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// setupConfigRepo creates a local git repository on the "master" branch
// holding the shared configuration and returns its path. The config file is
// named ".golangci.yml" unless name overrides it. Tests point [Lint.Config] at
// it through the GOMAKE_GOLINT_CONFIG_REPO environment variable so linting
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
