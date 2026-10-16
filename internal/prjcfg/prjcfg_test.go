// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package prjcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/dotenv/pkg/dotenv"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_Read(t *testing.T) {
	t.Run("project only", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		cfg := "C42_HOST=ghcr.io\nC42_REPO=$C42_HOST/ctx42\n"
		oskit.Write(t, cfg, oskit.MkdirAll(t, root, "configs"), File)

		// --- When ---
		have, err := Read(t.Context(), nil, root)

		// --- Then ---
		assert.NoError(t, err)
		want := &Config{
			Values: map[string]string{
				"C42_HOST": "ghcr.io",
				"C42_REPO": "ghcr.io/ctx42",
			},
		}
		assert.Equal(t, want, have)
	})

	t.Run("environment overrides declared key", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("C42_HOST", "registry.acme.io")
		rng.EnvSet("C42_OTHER", "x")

		root := t.TempDir()
		cfg := "C42_HOST=ghcr.io\nC42_REPO=$C42_HOST/ctx42\n"
		oskit.Write(t, cfg, oskit.MkdirAll(t, root, "configs"), File)

		// --- When ---
		have, err := Read(t.Context(), rng.EnvAll(), root)

		// --- Then ---
		assert.NoError(t, err)
		want := &Config{
			Values: map[string]string{
				"C42_HOST": "registry.acme.io",
				"C42_REPO": "registry.acme.io/ctx42",
			},
			Overrides: []string{"C42_HOST"},
		}
		assert.Equal(t, want, have)
	})

	t.Run("module layer", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOPROXY", "off")

		root := t.TempDir()
		mcf := "C42_VER=v1.0.0\nC42_URL=https://acme.io/$C42_VER/x\n"
		dir := newDkfModule(t, root, mcf)
		cfg := "" +
			EnvBldDkfModule + "=" + dkfModPath + "\n" +
			"C42_VER=v2.0.0\n" +
			"C42_OWN=own\n"
		oskit.Write(t, cfg, oskit.MkdirAll(t, root, "configs"), File)

		// --- When ---
		have, err := Read(t.Context(), rng.EnvAll(), root)

		// --- Then ---
		assert.NoError(t, err)
		want := &Config{
			Values: map[string]string{
				EnvBldDkfModule: dkfModPath,
				"C42_VER":       "v2.0.0",
				"C42_URL":       "https://acme.io/v2.0.0/x",
				"C42_OWN":       "own",
			},
			Module:    dkfModPath,
			ModuleDir: dir,
		}
		assert.Equal(t, want, have)
	})

	t.Run("module without configuration", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOPROXY", "off")

		root := t.TempDir()
		dir := newDkfModule(t, root, "")
		cfg := EnvBldDkfModule + "=" + dkfModPath + "\n"
		oskit.Write(t, cfg, oskit.MkdirAll(t, root, "configs"), File)

		// --- When ---
		have, err := Read(t.Context(), rng.EnvAll(), root)

		// --- Then ---
		assert.NoError(t, err)
		want := &Config{
			Values:    map[string]string{EnvBldDkfModule: dkfModPath},
			Module:    dkfModPath,
			ModuleDir: dir,
		}
		assert.Equal(t, want, have)
	})

	t.Run("error - no configuration", func(t *testing.T) {
		// --- When ---
		have, err := Read(t.Context(), nil, t.TempDir())

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
		assert.Nil(t, have)
	})

	t.Run("error - module not resolved", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOPROXY", "off")

		root := t.TempDir()
		oskit.Write(t, "module example.com/c\n\ngo 1.26\n", root, "go.mod")
		cfg := EnvBldDkfModule + "=" + dkfModPath + "\n"
		oskit.Write(t, cfg, oskit.MkdirAll(t, root, "configs"), File)

		// --- When ---
		have, err := Read(t.Context(), rng.EnvAll(), root)

		// --- Then ---
		assert.ErrorIs(t, ErrDkfModule, err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid module configuration", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOPROXY", "off")

		root := t.TempDir()
		dir := newDkfModule(t, root, "invalid\n")
		cfg := EnvBldDkfModule + "=" + dkfModPath + "\n"
		oskit.Write(t, cfg, oskit.MkdirAll(t, root, "configs"), File)

		// --- When ---
		have, err := Read(t.Context(), rng.EnvAll(), root)

		// --- Then ---
		assert.ErrorIs(t, dotenv.ErrInvLine, err)
		assert.ErrorContain(t, filepath.Join(dir, Path)+": ", err)
		assert.Nil(t, have)
	})
}

func Test_moduleDir(t *testing.T) {
	t.Run("replaced by directory", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOPROXY", "off")
		root := t.TempDir()
		dir := newDkfModule(t, root, "")

		// --- When ---
		have, err := moduleDir(t.Context(), rng.EnvAll(), root, dkfModPath)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, dir, have)
	})

	t.Run("error - not required", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOPROXY", "off")
		root := t.TempDir()
		oskit.Write(t, "module example.com/c\n\ngo 1.26\n", root, "go.mod")

		// --- When ---
		have, err := moduleDir(t.Context(), rng.EnvAll(), root, dkfModPath)

		// --- Then ---
		assert.ErrorIs(t, ErrDkfModule, err)
		want := "^cannot resolve the Dockerfile module: " +
			"C42_BLD_DKF_MODULE=example.com/dkf: " +
			"go: module example.com/dkf: not a known dependency: exit status 1$"
		assert.ErrorRegexp(t, want, err)
		assert.Equal(t, "", have)
	})

	t.Run("error - no go.mod", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()
		rng.EnvSet("GOPROXY", "off")
		rng.EnvSet("GOWORK", "off")

		root := t.TempDir()

		// --- When ---
		have, err := moduleDir(t.Context(), rng.EnvAll(), root, dkfModPath)

		// --- Then ---
		assert.ErrorIs(t, ErrDkfModule, err)
		assert.ErrorContain(t, "go.mod file not found", err)
		assert.Equal(t, "", have)
	})
}

func Test_readLayer(t *testing.T) {
	t.Run("layer read", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Write(t, "A=1\nB=$A/b\n", t.TempDir(), File)
		var lay dotenv.Layers

		// --- When ---
		err := readLayer(&lay, pth)

		// --- Then ---
		assert.NoError(t, err)
		have, _ := lay.Expand(nil)
		assert.Equal(t, map[string]string{"A": "1", "B": "1/b"}, have)
	})

	t.Run("error - no file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), File)
		var lay dotenv.Layers

		// --- When ---
		err := readLayer(&lay, pth)

		// --- Then ---
		assert.ErrorIs(t, ErrNoConfig, err)
		assert.ErrorIs(t, os.ErrNotExist, err)
	})

	t.Run("error - invalid line", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Write(t, "A=1\ninvalid\n", t.TempDir(), File)
		var lay dotenv.Layers

		// --- When ---
		err := readLayer(&lay, pth)

		// --- Then ---
		assert.ErrorIs(t, dotenv.ErrInvLine, err)
		assert.ErrorEqual(t, pth+": dotenv: invalid config line: 2", err)
	})

	t.Run("error - path is a directory", func(t *testing.T) {
		// --- Given ---
		pth := t.TempDir()
		var lay dotenv.Layers

		// --- When ---
		err := readLayer(&lay, pth)

		// --- Then ---
		assert.ErrorContain(t, pth+": ", err)
		assert.ErrorContain(t, "is a directory", err)
	})
}
