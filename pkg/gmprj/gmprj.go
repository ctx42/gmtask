// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmprj provides gomake targets and helpers for scaffolding Go projects
// and inspecting their build environment: creating the directory and file
// layout, initializing the module and git repository, and reporting project
// information as environment variables.
package gmprj

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"
)

// Package-level errors.
var (
	// ErrTooManyArgs is returned when a target gets too many arguments.
	ErrTooManyArgs = errors.New("too many arguments")

	// ErrNoConfig is returned when project is missing configuration file.
	ErrNoConfig = errors.New("no project configuration file")

	// ErrNoStructure is returned when the running target's gomake.yaml carries
	// no "structure" block to scaffold from.
	ErrNoStructure = errors.New("no project structure defined")

	// ErrModuleOriginMismatch is returned when both a Go module path and a git
	// origin are set and they do not resolve to the same module name.
	ErrModuleOriginMismatch = errors.New(
		"go module name does not match git origin",
	)

	// ErrMkdirNeedsName is returned when --mkdir is set without --origin or
	// --module (or when those values yield no project directory name).
	ErrMkdirNeedsName = errors.New("mkdir requires --origin or --module")

	// ErrDirNotEmpty is returned when the directory to scaffold already holds
	// entries and --force was not set.
	ErrDirNotEmpty = errors.New("project directory is not empty")

	// ErrNotDir is returned when the path to scaffold into exists but is not
	// a directory.
	ErrNotDir = errors.New("project path is not a directory")
)

// EnvSSHAuthSock holds the SSH agent socket path. It keeps the standard
// operating system variable name, set outside gomake, and is the only project
// environment variable gmprj declares.
const EnvSSHAuthSock = "SSH_AUTH_SOCK"

// Configuration file name and relative path.
const (
	// CfgFile is the project configuration file name.
	CfgFile = "project.conf"

	// CfgPath is the relative (to project root) path to the project's
	// configuration file.
	CfgPath = "configs" + string(os.PathSeparator) + CfgFile //nolint:gocritic
)

// Git layout of a repository [Setup.Setup] initializes.
const (
	// branchMaster holds the empty root commit and, later, the releases.
	branchMaster = "master"

	// branchDevelop holds the project files, open to any rewrite before it is
	// squash-merged into branchMaster.
	branchDevelop = "develop"

	// tagInitial tags the empty root commit on branchMaster.
	tagInitial = "v0.0.0"
)

// ScmNo is the [xdef.EnvScmState] value of a project that is not part of a
// git repository. The other two states are gitaid's [gitaid.StateClean] and
// [gitaid.StateDirty], and gitaid reports an error rather than a state for
// this one, so it is named here.
//
// It is project information only: the version [gitaid.Derive] assembles
// carries the tree state in its own identifier, so this value never reaches
// a version string and the "-" in it is free to stay.
const ScmNo = "no-scm"

// Project collects project scaffolding and information targets.
type Project struct{} //gomake:ns_root

// Env prints "env" formatted information about the project. An additional
// argument may be passed to the target in form of an environment variable name
// to display a single value.
//
// Example usage:
//
//	gomake :project:env
func (Project) Env(ctx context.Context, rng *ring.Ring) error {
	tgtName := ":project:env"
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.SetOutput(rng.Stderr())
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s:\n", tgtName)
		_, _ = fmt.Fprint(rng.Stderr(), head+fs.HelpOptions())
	}
	fs.BoolSL("help", "h", false, "show help")
	fs.BoolSL("export", "e", false, "export variables")
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fs.Args())
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}
	if len(rng.Args()) > 1 {
		return ErrTooManyArgs
	}

	root, err := Root(".")
	if err != nil {
		return err
	}
	inf, err := GetInfo(ctx, rng.EnvAll(), root)
	if err != nil {
		return err
	}

	args := rng.Args()
	switch len(args) {
	case 0:
		env := inf.Env()
		if fs.GetBool("export") {
			for i := range env {
				env[i] = "export " + env[i]
			}
		}
		_, _ = fmt.Fprint(rng.Stdout(), strings.Join(env, "\n")+"\n")

	case 1:
		_, _ = fmt.Fprint(rng.Stdout(), inf.Get(args[0]))
	}
	return nil
}

// Info prints formatted project information. Its output is a more readable
// version of the [Project.Env] target. An additional argument may be passed to
// the target in form of an environment variable name to display a single value.
//
// Example usage:
//
//	gomake :project:info
func (Project) Info(ctx context.Context, rng *ring.Ring) error {
	args := rng.Args()
	if len(args) > 1 {
		return ErrTooManyArgs
	}

	root, err := Root(".")
	if err != nil {
		return err
	}

	inf, err := GetInfo(ctx, rng.EnvAll(), root)
	if err != nil {
		return err
	}

	switch len(args) {
	case 0:
		_, _ = fmt.Fprint(rng.Stdout(), inf.String())

	case 1:
		_, _ = fmt.Fprint(rng.Stdout(), inf.Get(args[0]))
	}
	return nil
}

// Setup creates a project scaffold.
//
// It refuses to scaffold into a directory that already holds entries, dot
// entries included, and returns [ErrDirNotEmpty]. That is almost always a
// forgotten --mkdir, so --force is what says the directory was meant: it
// lifts the check, and makes --mkdir adopt the directory when it is already
// there instead of failing. A --mkdir path that exists but is not a directory
// returns [ErrNotDir].
//
// Example usage:
//
//	gomake :project:setup
func (Project) Setup(ctx context.Context, rng *ring.Ring) error {
	tgtName := ":project:setup"
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.SetOutput(rng.Stderr())
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s:\n", tgtName)
		examples := "\nExamples:\n" +
			"  # Derive the module name from the git remote.\n" +
			"  gomake :project:setup --origin git@github.com:prj/repo.git\n" +
			"\n" +
			"  # Set the Go module path explicitly.\n" +
			"  gomake :project:setup --module github.com/prj/repo\n"
		_, _ = fmt.Fprint(rng.Stderr(), head+fs.HelpOptions()+examples)
	}
	fs.BoolSL("help", "h", false, "show help")
	fs.StringSL("origin", "o", "", "remote repository path")
	fs.StringSL("module", "m", "", "go module name")
	fs.BoolSL("mkdir", "d", false, "create project directory")
	fs.BoolSL("force", "f", false, "use the directory even if it has files")
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fs.Args())
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}

	opts := []func(*Setup){
		WithSetupGitOrigin(fs.GetString("origin")),
		WithSetupGoModule(fs.GetString("module")),
		WithSetupMkdir(fs.GetBool("mkdir")),
		WithSetupForce(fs.GetBool("force")),
	}
	sup, err := NewSetup("", opts...)
	if err != nil {
		return err
	}
	if err = sup.Setup(ctx, rng); err != nil {
		return err
	}
	return nil
}
