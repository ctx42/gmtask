// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"

	"github.com/ctx42/dotenv/pkg/dotenv"
	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmgo"
)

// Info represents project configuration and environment.
type Info struct {
	// Absolute path to the project root directory. It is not reported as
	// an environment variable: nothing outside this module consumes it,
	// and the paths the images use are the C42_CTR_* ones, which name
	// locations inside the image rather than on the host.
	Root string

	// Parsed content of the project's configuration file. [GetInfo] replaces
	// the value of every key also set in the environment with the environment
	// one; see Overrides.
	Config map[string]string

	// Sorted keys of Config whose values the environment overrode, or nil.
	Overrides []string

	// All other project information values.
	Other map[string]string

	// By default set to the current date in UTC, but may be overwritten by the
	// [xdef.EnvBldDate] environment variable, which must be in RFC3339 format.
	BuildDate time.Time

	// Version derived from the git working tree. It is the zero value when
	// the project is not a git repository or the repository is empty.
	Version gitaid.Version

	// True when the project has a Dockerfile in its root directory.
	HasDockerfile bool

	// LDFlags are the "-X" linker flags injecting the build metadata into the
	// project's root package, under the xdef.Var* variable names. They are
	// empty when the project root holds no go.mod. Outside a git repository
	// the revision and hash are empty and the state is [ScmNo].
	LDFlags string
}

// NewInfo returns an [Info] seeded from env. BuildDate defaults to the current
// UTC time, overridden by [xdef.EnvBldDate] (RFC3339) when it is set to a
// valid date; an invalid one is ignored here and rejected by [GetInfo].
func NewInfo(env []string) *Info {
	inf := &Info{
		Config:    make(map[string]string),
		Other:     make(map[string]string),
		BuildDate: time.Now().UTC().Truncate(time.Millisecond),
	}
	if ev, ok := ring.EnvLookup(env, xdef.EnvBldDate); ok {
		if et, err := time.Parse(time.RFC3339Nano, ev); err == nil {
			inf.BuildDate = et
		}
	}
	if ev, ok := ring.EnvLookup(env, EnvSSHAuthSock); ok {
		inf.Set(EnvSSHAuthSock, ev)
	}
	return inf
}

// GetInfo retrieves information about a project at the root path. The root may
// be set to the "." value to indicate the current working directory. A
// variable in env named like a configuration file key overrides that key's
// value; variables the file does not declare are ignored. It returns an error
// when [xdef.EnvBldDate] is set to a value that is not an RFC3339 date.
func GetInfo(ctx context.Context, env []string, root string) (*Info, error) {
	var err error
	if root, err = filepath.Abs(root); err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}

	if ev, ok := ring.EnvLookup(env, xdef.EnvBldDate); ok {
		if _, err = time.Parse(time.RFC3339Nano, ev); err != nil {
			format := "invalid %s: %q: %w"
			return nil, fmt.Errorf(format, xdef.EnvBldDate, ev, err)
		}
	}

	inf := NewInfo(env)
	inf.Root = root
	inf.Set(xdef.EnvScmState, ScmNo)
	inf.Set(xdef.EnvBldDate, inf.BuildDateFmt())

	inf.Set(xdef.EnvPrjName, ProjectName(root))
	spec, err := getGoSpec(ctx, env, root)
	if err != nil {
		return nil, err
	}
	if err = inf.setConfig(filepath.Join(root, CfgPath)); err != nil {
		return nil, err
	}
	inf.Overrides = dotenv.Override(inf.Config, env)
	if err = inf.setScm(ctx, root, ring.New(ring.WithEnv(env))); err != nil {
		return nil, err
	}
	inf.setLDFlags(spec)
	inf.HasDockerfile = gomake.FileExists(filepath.Join(root, "Dockerfile"))
	return inf, nil
}

// BuildDateFmt returns the build date rendered the way [gmgo.BldDateFmt]
// renders it.
func (inf *Info) BuildDateFmt() string {
	return gmgo.BldDateFmt(inf.BuildDate)
}

// CfgGet retrieves the value of the configuration variable by name. It returns
// the value, which will be empty if the variable is not present. To distinguish
// between an empty value and an unset value, use [Info.CfgLookup].
func (inf *Info) CfgGet(name string) string {
	return inf.Config[name]
}

// CfgLookup retrieves the value of the configuration variable by name key. If
// the variable is present in the configuration the value (which may be empty)
// is returned and the boolean is true. Otherwise, the returned value will be
// empty and the boolean will be false.
func (inf *Info) CfgLookup(name string) (string, bool) {
	val, ok := inf.Config[name]
	return val, ok
}

// Set adds a custom environment variable to the [Info] instance.
func (inf *Info) Set(name, value string) {
	if inf.Other == nil {
		inf.Other = make(map[string]string)
	}
	inf.Other[name] = value
}

// Get retrieves the value of the environment variable by name (field or extra).
// It returns the value, which will be empty if the variable is not present. To
// distinguish between an empty value and an unset value, use [Info.Lookup].
func (inf *Info) Get(name string) string {
	val, _ := inf.Lookup(name)
	return val
}

// Lookup retrieves the value of the environment variable by name (field or
// extra). If the variable is present the value (which may be empty) is returned
// and the boolean is true. Otherwise, the returned value will be empty and the
// boolean will be false.
func (inf *Info) Lookup(name string) (string, bool) {
	// Other overrides Config, as in [Info.Env].
	if val, ok := inf.Other[name]; ok {
		return val, true
	}
	val, ok := inf.Config[name]
	return val, ok
}

// Custom returns custom environment variables in alphabetical order.
func (inf *Info) Custom() []string {
	return toAlphabeticalEnv(inf.Other)
}

// Env returns environment variables representing [Info] fields.
func (inf *Info) Env() []string {
	all := make(map[string]string, len(inf.Config)+len(inf.Other))
	maps.Copy(all, inf.Config)
	maps.Copy(all, inf.Other)
	return toAlphabeticalEnv(all)
}

// String returns a string representing [Info] fields in human-readable form.
func (inf *Info) String() string {
	buf := &bytes.Buffer{}
	_ = gomake.PrettyPrintEnv(inf.Env(), buf)
	return buf.String()
}

// setConfig loads the project configuration from the given path.
func (inf *Info) setConfig(pth string) error {
	cfg, err := getConfig(pth)
	if err != nil {
		return err
	}
	inf.Config = cfg
	return nil
}

// setScm sets values related to source control management. The rng carries
// the environment [gmgo.ProjectVersion] reads the bump override from.
func (inf *Info) setScm(
	ctx context.Context,
	root string,
	rng *ring.Ring,
) error {

	empty, err := gitaid.IsEmpty(ctx, root)
	if err != nil {
		if errors.Is(err, gitaid.ErrNotRepo) {
			return nil
		}
		return fmt.Errorf("check repository: %w", err)
	}
	if empty {
		return nil
	}

	value, err := gitaid.ProjectOrigin(ctx, root)
	if err != nil {
		return fmt.Errorf("read origin: %w", err)
	}
	if value != "" {
		inf.Set(xdef.EnvScmRepo, value)
	}

	value, err = gitaid.LatestHash(ctx, root)
	if err != nil && !errors.Is(err, gitaid.ErrEmptyRepo) {
		return fmt.Errorf("read latest hash: %w", err)
	}
	inf.Set(xdef.EnvScmHash, value)

	if inf.Version, err = gmgo.ProjectVersion(ctx, rng, root, ""); err != nil {
		if !errors.Is(err, gitaid.ErrEmptyRepo) {
			return fmt.Errorf("derive version: %w", err)
		}
	}
	inf.Set(xdef.EnvScmRev, inf.Version.Rev)

	value, err = gitaid.WorkTreeStatus(ctx, root)
	if err != nil {
		return fmt.Errorf("read work tree status: %w", err)
	}
	inf.Set(xdef.EnvScmState, value)
	return nil
}

// setLDFlags sets the "-X" linker flags injecting build date and SCM related
// values into the version package at the import spec. It injects into the
// canonical build-metadata variable names (the xdef.Var* constants) that
// gomake's :go:build target reads, so a project scaffolded here links the same
// way gomake builds it. An empty spec leaves LDFlags unset.
func (inf *Info) setLDFlags(spec string) {
	if spec == "" {
		return
	}
	vars := []gmgo.LDVar{
		{Name: xdef.VarBldDate, Value: inf.BuildDateFmt()},
		{Name: xdef.VarScmRev, Value: inf.Get(xdef.EnvScmRev)},
		{Name: xdef.VarScmHash, Value: inf.Get(xdef.EnvScmHash)},
		{Name: xdef.VarScmState, Value: inf.Get(xdef.EnvScmState)},
	}
	inf.LDFlags = gmgo.LDFlags(spec, vars)
}

// getGoSpec returns the Go import spec for the Go project in the root
// directory. It returns an empty string and no error when root holds no go.mod
// file, even when an ancestor directory does, and an error on any filesystem
// error.
func getGoSpec(ctx context.Context, env []string, root string) (string, error) {
	if !gomake.FileExists(filepath.Join(root, "go.mod")) {
		return "", nil
	}
	rng := ring.New(ring.WithEnv(env))
	spec, err := gmgo.ImpPath(ctx, rng, root)
	if err != nil {
		if !errors.Is(err, gomake.ErrNoGoMod) {
			return "", err
		}
	}
	return spec, nil
}

// getConfig reads the project configuration at the given path.
func getConfig(pth string) (map[string]string, error) {
	fil, err := os.Open(pth) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", ErrNoConfig, err)
		}
		return nil, fmt.Errorf("%s: %w", pth, err)
	}
	defer func() { _ = fil.Close() }()

	cfg := make(map[string]string, 10)
	if err = dotenv.Parse(cfg, fil); err != nil {
		return nil, fmt.Errorf("%s: %w", pth, err)
	}
	return cfg, nil
}
