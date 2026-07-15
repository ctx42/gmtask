// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"errors"
	"strings"

	"github.com/ctx42/testing/pkg/tester"
)

// errTest is a sentinel error used to exercise error passthrough.
var errTest = errors.New("test error")

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
