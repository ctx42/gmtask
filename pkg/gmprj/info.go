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

// Configuration file name and relative path.
const (
	// CfgFile is the project configuration file name.
	CfgFile = "project.conf"

	// CfgPath is the relative (to project root) path to the project's
	// configuration file.
	CfgPath = "configs" + string(os.PathSeparator) + CfgFile
)

// Info represents project configuration and environment.
type Info struct {
	// Parsed content of the project's configuration file.
	Config map[string]string

	// All other project information values.
	Other map[string]string

	// By default set to the current date in UTC, but may be overwritten by the
	// [xdef.EnvBuildDate] environment variable which must be in RFC3339 format.
	BuildDate time.Time

	// True when the project has a Dockerfile in its root directory.
	HasDockerfile bool

	// LDFlags used to set values in the project's "version.go" file.
	//
	// If the project is not part of a git repository "ScmRev" and "ScmHash"
	// will be empty strings and the working directory state will be reported as
	// dirty.
	LDFlags string
}

// NewInfo returns a new instance of Info.
func NewInfo(env []string) *Info {
	inf := &Info{
		Config:    make(map[string]string),
		Other:     make(map[string]string),
		BuildDate: time.Now().UTC().Truncate(time.Millisecond),
	}
	if ev, ok := ring.EnvLookup(env, xdef.EnvBuildDate); ok {
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
// be set to the "." value to indicate the current working directory.
func GetInfo(ctx context.Context, env []string, root string) (*Info, error) {
	var err error
	if root, err = filepath.Abs(root); err != nil {
		return nil, err
	}

	inf := NewInfo(env)
	inf.Set(xdef.EnvProjRootDir, root)
	inf.Set(xdef.EnvProjDistDir, filepath.Join(root, "dist"))
	inf.Set(xdef.EnvScmState, ScmNo)
	inf.Set(xdef.EnvBuildDate, inf.BuildDate.Format(time.RFC3339Nano))

	spec, err := inf.setProjectName(ctx, env, root)
	if err != nil {
		return nil, err
	}
	if err = inf.setConfig(filepath.Join(root, CfgPath)); err != nil {
		return nil, err
	}
	if err = inf.setScm(ctx, root); err != nil {
		return nil, err
	}
	inf.setCCTag(env)
	inf.setLDFlags(spec)
	inf.HasDockerfile = gomake.FileExists(filepath.Join(root, "Dockerfile"))
	return inf, nil
}

// BuildDateFmt returns the RFC3339Nano formatted build date.
func (inf *Info) BuildDateFmt() string {
	return inf.BuildDate.Format(time.RFC3339Nano)
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

// Set adds a custom environment variable to the Info instance.
func (inf *Info) Set(name, value string) {
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
	env := gomake.EnvSplit(inf.Env())
	val, exist := env[name]
	return val, exist
}

// Custom returns custom environment variables in alphabetical order.
func (inf *Info) Custom() []string {
	return toAlphabeticalEnv(inf.Other)
}

// Env returns environment variables representing Info fields.
func (inf *Info) Env() []string {
	all := make(map[string]string, len(inf.Config)+len(inf.Other))
	maps.Copy(all, inf.Config)
	maps.Copy(all, inf.Other)
	return toAlphabeticalEnv(all)
}

// String returns a string representing Info fields in human readable form.
func (inf *Info) String() string {
	buf := &bytes.Buffer{}
	_ = gomake.PrettyPrintEnv(inf.Env(), buf)
	return buf.String()
}

// setProjectName sets the project name based on the Go module name or the root
// directory name.
func (inf *Info) setProjectName(
	ctx context.Context,
	env []string,
	root string,
) (string, error) {

	inf.Set(xdef.EnvProjName, ProjectName(root))
	spec, err := getGoSpec(ctx, env, root)
	if err != nil {
		return "", err
	}
	if spec != "" {
		inf.Set(xdef.EnvProjGoImpSpec, spec)
	}
	return spec, nil
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

// setCCTag sets [xdef.EnvCCID] based on the environment.
func (inf *Info) setCCTag(env []string) {
	if val, ok := ring.EnvLookup(env, xdef.EnvCCID); ok && val != "" {
		inf.Set(xdef.EnvCCID, val)
		return
	}
	inf.Set(xdef.EnvCCID, xdef.PhUnknown)
}

// setScm sets values related to source control management.
func (inf *Info) setScm(ctx context.Context, root string) error {
	empty, err := gitaid.IsEmpty(ctx, root)
	if err != nil {
		if errors.Is(err, gitaid.ErrNotRepo) {
			return nil
		}
		return err
	}
	if empty {
		return nil
	}

	value, err := gitaid.ProjectOrigin(ctx, root)
	if err != nil {
		return err
	}
	if value != "" {
		inf.Set(xdef.EnvScmRepo, value)
	}

	value, err = gitaid.LatestHash(ctx, root)
	if err != nil && !errors.Is(err, gitaid.ErrEmptyRepo) {
		return err
	}
	inf.Set(xdef.EnvScmHash, value)

	value, err = gitaid.Describe(ctx, root)
	if err != nil && !errors.Is(err, gitaid.ErrEmptyRepo) {
		return err
	}
	inf.Set(xdef.EnvScmRev, value)

	value, err = gitaid.WorkTreeStatus(ctx, root)
	if err != nil {
		return err
	}
	inf.Set(xdef.EnvScmState, value)
	return nil
}

// setLDFlags sets the "-X" linker flags injecting build date and SCM related
// values into the version package at the import spec. It injects into the
// canonical build-metadata variable names (the xdef.Var* constants) that
// gomake's :go:build target reads, so a project scaffolded here links the same
// way gomake builds it. The import spec must not be an empty string.
func (inf *Info) setLDFlags(spec string) {
	if spec == "" {
		return
	}
	ccid := inf.Get(xdef.EnvCCID)
	if ccid == "" {
		ccid = xdef.PhUnknown
	}
	vars := []gmgo.LDVar{
		{Name: xdef.VarBuildDate, Value: inf.BuildDateFmt()},
		{Name: xdef.VarScmRev, Value: inf.Get(xdef.EnvScmRev)},
		{Name: xdef.VarScmHash, Value: inf.Get(xdef.EnvScmHash)},
		{Name: xdef.VarScmState, Value: inf.Get(xdef.EnvScmState)},
		{Name: xdef.VarCCID, Value: ccid},
	}
	inf.LDFlags = gmgo.LDFlags(spec, vars)
}

// getGoSpec returns the Go import spec for the Go project in the root directory.
// It returns an empty string and no error when root is not a Go project, and an
// error on any filesystem error.
func getGoSpec(ctx context.Context, env []string, root string) (string, error) {
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
	fil, err := os.Open(pth)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", ErrNoConfig, err)
		}
		return nil, err
	}
	defer func() { _ = fil.Close() }()

	cfg := make(map[string]string, 10)
	if err = dotenv.Parse(cfg, fil); err != nil {
		return nil, err
	}
	return cfg, nil
}
