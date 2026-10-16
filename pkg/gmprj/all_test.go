// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/jsonkit"
	"github.com/ctx42/testkit/pkg/oskit"
)

// setStructure injects a minimal project structure config block into the ring,
// so [Setup.Setup] can load it. The go-test-all.run.xml content templates the
// project name, and configs/project.conf is declared so addRegRepo can append.
func setStructure(t tester.T, rng *ring.Ring) {
	t.Helper()
	block := map[string]any{
		"structure": map[string]any{
			"dev": map[string]any{
				"type": "dir",
				"idea": map[string]any{
					"type": "dir",
					"go-test-all.run.xml": map[string]any{
						"type":    "file",
						"content": `<module name="{{.ProjectName}}" />`,
					},
				},
			},
			"configs": map[string]any{
				"type":         "dir",
				"project.conf": map[string]any{"type": "file", "content": ""},
			},
		},
	}
	rng.MetaSet(gomake.ConfigMetaKey, jsonkit.To(t, block))
}

// toEnv parses key values in "info" format and returns them as slice of strings
// in the same format as [os.Environ] does.
func toEnv(info string) []string {
	envS := regexp.MustCompile(` +`).ReplaceAllString(info, "=")
	if envS == "" {
		return nil
	}
	env := strings.Split(envS, "\n")
	clean := env[:0]
	for i := range env {
		if env[i] != "" {
			clean = append(clean, env[i])
		}
	}
	return clean
}

func Test_toEnv(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		// --- Given ---
		info := "" +
			"key0     val0\n" +
			"key1     val1\n" +
			"key2     val2\n"

		// --- When ---
		have := toEnv(info)

		// --- Then ---
		want := []string{
			"key0=val0",
			"key1=val1",
			"key2=val2",
		}
		assert.Equal(t, want, have)
	})

	t.Run("empty", func(t *testing.T) {
		// --- When ---
		have := toEnv("")

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("empty lines", func(t *testing.T) {
		// --- Given ---
		info := "" +
			"key0     val0\n" +
			"\n" +
			"key2     val2\n"

		// --- When ---
		have := toEnv(info)

		// --- Then ---
		want := []string{
			"key0=val0",
			"key2=val2",
		}
		assert.Equal(t, want, have)
	})
}

// dkfModPath is the module path of the Dockerfile module newDkfModule creates.
const dkfModPath = "example.com/dkf"

// newDkfModule creates a Dockerfile module named dkfModPath in a temporary
// directory and rewrites the go.mod of the project in root to require it,
// replaced by that directory, so resolving it needs no network. A non-empty
// cfg becomes the module's configs/project.conf. It returns the module
// directory.
func newDkfModule(t tester.T, root, cfg string) string {
	t.Helper()
	dir := t.TempDir()
	oskit.Write(t, "module "+dkfModPath+"\n\ngo 1.26\n", dir, "go.mod")
	if cfg != "" {
		oskit.Write(t, cfg, oskit.MkdirAll(t, dir, "configs"), CfgFile)
	}
	mod := "" +
		"module example.com/comp/project\n\n" +
		"go 1.26\n\n" +
		"require " + dkfModPath + " v0.0.0\n\n" +
		"replace " + dkfModPath + " => " + dir + "\n"
	oskit.Write(t, mod, root, "go.mod")
	return dir
}
