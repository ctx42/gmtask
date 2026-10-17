// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// dkfModPath is the module path of the Dockerfile module newDkfModule creates.
const dkfModPath = "example.com/dkf"

// errTest is a sentinel error used to exercise error passthrough.
var errTest = errors.New("test error")

// newDkfModule creates a Dockerfile module named dkfModPath in a temporary
// directory, with a Dockerfile when dkf is true, and rewrites the go.mod of the
// project in root to require it, replaced by that directory, so resolving it
// needs no network. It returns the module directory.
func newDkfModule(t tester.T, root string, dkf bool) string {
	t.Helper()
	dir := t.TempDir()
	oskit.Write(t, "module "+dkfModPath+"\n\ngo 1.26\n", dir, "go.mod")
	if dkf {
		oskit.Write(t, "FROM scratch\n", dir, "Dockerfile")
	}
	mod := "" +
		"module example.com/comp/project\n\n" +
		"go 1.26\n\n" +
		"require " + dkfModPath + " v0.0.0\n\n" +
		"replace " + dkfModPath + " => " + dir + "\n"
	oskit.Write(t, mod, root, "go.mod")
	return dir
}

// infoToEnv parses "env"-style output (one "KEY VALUE" pair per line) into a
// map. It fails the test on a malformed line or a repeated key.
func infoToEnv(t tester.T, output string) map[string]string {
	t.Helper()
	env := make(map[string]string, 10)
	for lin := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(lin))
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			t.Errorf("expected line to have two fields, got: %q", lin)
			continue
		}
		key := fields[0]
		if _, ok := env[key]; ok {
			t.Errorf("did not expect keys to repeat, key: %q", key)
		}
		env[key] = fields[1]
	}
	return env
}

// dockerCLI runs docker with args, feeding it stdin, and returns its trimmed
// standard output. It fails the test on error.
func dockerCLI(t tester.T, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("docker %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// dockerHas reports whether the Docker resource id of kind ("container",
// "network", "volume", "image") exists.
func dockerHas(kind, id string) bool {
	return exec.Command("docker", kind, "inspect", id).Run() == nil
}

// dockerTry runs docker with args, ignoring any failure. Test cleanups use it
// for resources a test may already have removed.
func dockerTry(args ...string) {
	_ = exec.Command("docker", args...).Run()
}
