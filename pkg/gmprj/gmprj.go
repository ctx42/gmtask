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

// Working directory state values reported through [xdef.EnvScmState].
const (
	// ScmNo marks a project that is not part of a git repository.
	ScmNo = "no-scm"

	// ScmClean marks a git working directory with no uncommitted changes.
	ScmClean = "clean"
)

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
		_, _ = fmt.Fprint(rng.Stderr(), head+xflag.HelpOptions(fs))
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
		_, _ = fmt.Fprint(rng.Stderr(), head+xflag.HelpOptions(fs)+examples)
	}
	fs.BoolSL("help", "h", false, "show help")
	fs.StringSL("origin", "o", "", "remote repository path")
	fs.StringSL("module", "m", "", "go module name")
	fs.BoolSL("mkdir", "d", false, "create project directory")
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fs.Args())
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}

	var root string
	if fs.GetBool("mkdir") {
		src := fs.GetString("origin")
		mod := fs.GetString("module")
		if mod != "" {
			src = mod
		}
		root = ProjectName(src)
		if err := os.Mkdir(root, 0o755); err != nil { //nolint:gosec
			return err
		}
	}

	opts := []func(*Setup){
		WithSetupGitOrigin(fs.GetString("origin")),
		WithSetupGoModule(fs.GetString("module")),
	}
	sup, err := NewSetup(root, opts...)
	if err != nil {
		return err
	}
	if err = sup.Setup(ctx, rng); err != nil {
		return err
	}
	return nil
}
