// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmgo"
)

// WithSetupDockerRepo is option for [NewSetup] setting Docker private
// repository link.
func WithSetupDockerRepo(repo string) func(*Setup) {
	return func(setup *Setup) { setup.repo = repo }
}

// WithSetupGitOrigin is option for [NewSetup] setting Git remote repository.
func WithSetupGitOrigin(origin string) func(*Setup) {
	return func(setup *Setup) { setup.origin = origin }
}

// WithSetupGoModule is option for [NewSetup] setting Go module.
func WithSetupGoModule(module string) func(*Setup) {
	return func(setup *Setup) { setup.module = module }
}

// Setup is used to set up new project.
type Setup struct {
	root   string // Absolute path to the project's root directory.
	origin string // Git repository origin.
	module string // Go module name.
	name   string // Project name.
	repo   string // Docker private repository.
}

// NewSetup returns new instance of Setup for project in root directory.
//
// Example:
//
//	Setup("/path/to/dir")
//	Setup("")
//
// The empty project root directory means current working directory.
func NewSetup(root string, opts ...func(*Setup)) (*Setup, error) {
	sup := &Setup{
		root: root,
	}
	for _, opt := range opts {
		opt(sup)
	}

	var err error
	if sup.root == "" {
		if sup.root, err = os.Getwd(); err != nil {
			return nil, err
		}
	}

	switch {
	case sup.origin != "":
		module := GoModuleName(sup.origin)
		if sup.module != "" && module != GoModuleName(sup.module) {
			return nil, errors.New("go module name does not match git origin")
		}
		sup.module = module
		sup.name = ProjectName(sup.origin)

	case sup.module != "":
		sup.name = ProjectName(sup.module)

	default:
		sup.module = GoModuleName(sup.root)
		sup.name = ProjectName(sup.root)
	}
	return sup, nil
}

// vars returns the values exposed to file node content templates.
func (sup *Setup) vars() tmplVars {
	return tmplVars{
		ProjectName: sup.name,
		Module:      sup.module,
		Package:     GoPkgName(sup.name),
		Origin:      sup.origin,
		Repo:        sup.repo,
	}
}

// Setup creates the directories and files declared in the project's gomake.yaml
// "structure" block, initializes the Go module and git repository, adds an
// initial commit and tags it v0.0.0. It fails before touching the filesystem
// when no structure is configured. The empty string used for dir means the
// current working directory.
func (sup *Setup) Setup(ctx context.Context, rng *ring.Ring) error {
	str, err := loadStructure(rng)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(rng.Stdout(), "setting up project in: %s\n", sup.root)

	// Setup always initializes git and a Go module, so both optional features
	// are enabled for the scaffold.
	err = str.materialize(
		rng.Stdout(),
		sup.root,
		sup.vars(),
		featureGit,
		featureGolang,
	)
	if err != nil {
		return err
	}

	if err = sup.addImgSrc(rng); err != nil {
		return err
	}

	if !gomake.FileExists(filepath.Join(sup.root, "go.mod")) {
		if err := gmgo.InitModule(ctx, rng, sup.root, sup.module); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(rng.Stdout(), "module %q initialized\n", sup.module)
	}

	if err = gitaid.IsRepo(ctx, sup.root); err != nil {
		if !errors.Is(err, gitaid.ErrNotRepo) {
			return err
		}
		if err = sup.initScmRepo(ctx, rng); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprint(rng.Stdout(), "done\n")
	return nil
}

// addImgSrc appends the image source variable to the project configuration file
// when a Docker repository is configured; it is a no-op when none is set.
func (sup *Setup) addImgSrc(rng *ring.Ring) error {
	if sup.repo == "" {
		return nil
	}
	pth := filepath.Join(sup.root, CfgPath)
	fil, err := os.OpenFile(pth, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o666) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()

	line := fmt.Sprintf("%s=%s\n", xdef.EnvImgSrc, sup.repo)
	if _, err = fil.WriteString(line); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "added %s to: %s\n", xdef.EnvImgSrc, pth)
	return nil
}

// initScmRepo initializes git repository in given directory, creates initial
// commit and the first tag v0.0.0. The empty string used for root directory
// means current working directory.
func (sup *Setup) initScmRepo(ctx context.Context, rng *ring.Ring) error {
	err := gitaid.Init(ctx, sup.root)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "git: repository initialized\n")

	if err = gitaid.AddAll(ctx, sup.root); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "git: all files added\n")

	if sup.origin != "" {
		if err = gitaid.AddRemote(ctx, sup.root, sup.origin); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(rng.Stdout(), "git: remote origin added\n")
	}

	if err = gitaid.Commit(ctx, sup.root, "Initial commit."); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "git: initial commit made\n")

	if err = gitaid.Tag(ctx, sup.root, "v0.0.0", "initial tag\n"); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "git: repository tagged with v0.0.0\n")
	return nil
}
