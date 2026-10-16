// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package prjcfg reads a project's layered configuration: the configuration
// file of the module the project's [EnvBldDkfModule] key names, then the
// project's own one, then the environment. It is shared by the packages that
// read the configuration, which cannot import each other.
package prjcfg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ctx42/dotenv/pkg/dotenv"
)

// Package-level errors.
var (
	// ErrNoConfig is returned when the project has no configuration file.
	ErrNoConfig = errors.New("no project configuration file")

	// ErrDkfModule is returned when the module [EnvBldDkfModule] names cannot
	// be resolved, for example because the project's go.mod does not require
	// it.
	ErrDkfModule = errors.New("cannot resolve the Dockerfile module")
)

// EnvBldDkfModule is the project configuration key naming the Go module whose
// root holds the Dockerfile the project builds its images from, together with
// the build context and the base project configuration.
const EnvBldDkfModule = "C42_BLD_DKF_MODULE"

// Configuration file name and relative path.
const (
	File = "project.conf"

	// Path is the path of the configuration file relative to the project
	// root.
	Path = "configs" + string(os.PathSeparator) + File //nolint:gocritic
)

// Config is a project configuration with its layers applied.
type Config struct {
	// Values maps the declared keys to their values.
	Values map[string]string

	// Overrides lists, sorted, the keys the environment overrode.
	Overrides []string

	// Module is the Go module path the [EnvBldDkfModule] key names, or empty
	// when the key is not set.
	Module string

	// ModuleDir is the directory of the Module version the project's go.mod
	// requires, or empty when Module is.
	ModuleDir string
}

// Read returns the configuration of the project in root. The layers apply in
// order, a later one replacing the values of the keys an earlier one declares:
// the configuration file of the module the project's [EnvBldDkfModule] key
// names, when set and the module has one; the project's configuration file,
// which may add keys of its own; and env. A variable in env named like a
// configuration key overrides that key's value; variables no configuration
// file declares are ignored. Variable references are expanded after all
// layers apply; see [dotenv.Layers].
//
// The module is resolved with the go command, run in root with env. It
// returns an error wrapping [ErrNoConfig] when the project has no
// configuration file, and one wrapping [ErrDkfModule] when the module cannot
// be resolved.
func Read(ctx context.Context, env []string, root string) (*Config, error) {
	pth := filepath.Join(root, Path)
	var prj dotenv.Layers
	if err := readLayer(&prj, pth); err != nil {
		return nil, err
	}
	cfg, keys := prj.Expand(env)
	mod := cfg[EnvBldDkfModule]
	if mod == "" {
		return &Config{Values: cfg, Overrides: keys}, nil
	}

	dir, err := moduleDir(ctx, env, root, mod)
	if err != nil {
		return nil, err
	}
	var lay dotenv.Layers
	err = readLayer(&lay, filepath.Join(dir, Path))
	if err != nil && !errors.Is(err, ErrNoConfig) {
		return nil, err
	}
	if err = readLayer(&lay, pth); err != nil {
		return nil, err
	}
	cfg, keys = lay.Expand(env)
	pcf := &Config{Values: cfg, Overrides: keys, Module: mod, ModuleDir: dir}
	return pcf, nil
}

// moduleDir returns the directory of the version of the module at pth that
// the module in root requires, downloading it to the module cache when needed.
// The go command runs with env. Errors wrap [ErrDkfModule] and name the
// [EnvBldDkfModule] key.
func moduleDir(
	ctx context.Context,
	env []string,
	root string,
	pth string,
) (string, error) {

	goCmd := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = env
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			format := "%w: %s=%s: %s: %w"
			key := EnvBldDkfModule
			return "", fmt.Errorf(format, ErrDkfModule, key, pth, msg, err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	// Download first: it fails for a module go.mod does not require. A module
	// replaced by a local directory needs no download, and only "go list"
	// reports its directory.
	if _, err := goCmd("mod", "download", pth); err != nil {
		return "", err
	}
	return goCmd("list", "-m", "-f", "{{.Dir}}", pth)
}

// readLayer reads the configuration file at pth as the next layer of lay. It
// returns an error wrapping [ErrNoConfig] when the file does not exist.
func readLayer(lay *dotenv.Layers, pth string) error {
	fil, err := os.Open(pth) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %w", ErrNoConfig, err)
		}
		return fmt.Errorf("%s: %w", pth, err)
	}
	defer func() { _ = fil.Close() }()

	if err = lay.Read(fil); err != nil {
		return fmt.Errorf("%s: %w", pth, err)
	}
	return nil
}
