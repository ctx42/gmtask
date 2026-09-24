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

// WithSetupMkdir is option for [NewSetup] making [Setup.Setup] create the
// project root directory instead of scaffolding into an existing one.
func WithSetupMkdir(mkdir bool) func(*Setup) {
	return func(setup *Setup) { setup.mkdir = mkdir }
}

// WithSetupForce is option for [NewSetup] allowing [Setup.Setup] to scaffold
// into a directory that already holds entries.
func WithSetupForce(force bool) func(*Setup) {
	return func(setup *Setup) { setup.force = force }
}

// checkSetupRoot reports whether pth can take a project scaffold. A path that
// does not exist is fine - it is about to be created. An existing path that is
// not a directory yields [ErrNotDir] whatever force says, since nothing can be
// scaffolded into a file; a directory that already holds entries, dot entries
// included, yields [ErrDirNotEmpty] unless force is set.
func checkSetupRoot(pth string, force bool) error {
	inf, err := os.Stat(pth)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !inf.IsDir() {
		return fmt.Errorf("%w: %s", ErrNotDir, pth)
	}
	if force {
		return nil
	}

	empty, err := dirEmpty(pth)
	if err != nil {
		return err
	}
	if !empty {
		format := "%w: %s (use --force to set up anyway)"
		return fmt.Errorf(format, ErrDirNotEmpty, pth)
	}
	return nil
}

// Setup is used to set up new project.
type Setup struct {
	root   string // Absolute path to the project's root directory.
	origin string // Git repository origin.
	module string // Go module name.
	name   string // Project name.
	repo   string // Docker private repository.
	mkdir  bool   // Create the root directory instead of using an existing one.
	force  bool   // Scaffold even into a directory that holds entries.
}

// NewSetup returns a new [Setup] for the project in root. An empty root means
// the current working directory, unless [WithSetupMkdir] is set - then the
// root is a directory named after the project, below the current working
// directory, and the project name must come from [WithSetupGitOrigin] or
// [WithSetupGoModule] or [ErrMkdirNeedsName] is returned.
//
// Example:
//
//	NewSetup("/path/to/dir")
//	NewSetup("")
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

	// Captured before the switch, which fills the module and the name from the
	// root when the caller supplied neither.
	named := sup.origin != "" || sup.module != ""

	switch {
	case sup.origin != "":
		module := GoModuleName(sup.origin)
		if sup.module != "" && module != GoModuleName(sup.module) {
			return nil, ErrModuleOriginMismatch
		}
		sup.module = module
		sup.name = ProjectName(sup.origin)

	case sup.module != "":
		sup.name = ProjectName(sup.module)

	default:
		sup.module = GoModuleName(sup.root)
		sup.name = ProjectName(sup.root)
	}

	if sup.mkdir && root == "" {
		if err = sup.setMkdirRoot(named); err != nil {
			return nil, err
		}
	}
	return sup, nil
}

// setMkdirRoot points the root at a directory named after the project, below
// the root resolved so far. Only an origin or a module can name it - the
// working directory's own name would make a directory inside itself - so
// without one it returns [ErrMkdirNeedsName].
func (sup *Setup) setMkdirRoot(named bool) error {
	if !named || sup.name == "" {
		return ErrMkdirNeedsName
	}
	sup.root = filepath.Join(sup.root, sup.name)
	return nil
}

// makeRoot creates the project root directory when [WithSetupMkdir] is set. An
// existing directory is an error unless [WithSetupForce] says it was meant.
func (sup *Setup) makeRoot() error {
	if !sup.mkdir {
		return nil
	}
	if err := os.Mkdir(sup.root, 0o755); err != nil { //nolint:gosec
		if !sup.force || !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return nil
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
// initial commit and tags it v0.0.0.
//
// Every check runs before the first filesystem write: a missing structure
// block fails, then a root that is not a directory returns [ErrNotDir] and one
// that already holds entries returns [ErrDirNotEmpty] unless [WithSetupForce]
// is set. With [WithSetupMkdir] the root is then created, and an existing one
// is an error unless [WithSetupForce] says it was meant.
func (sup *Setup) Setup(ctx context.Context, rng *ring.Ring) error {
	str, err := loadStructure(rng)
	if err != nil {
		return err
	}
	if err = checkSetupRoot(sup.root, sup.force); err != nil {
		return err
	}
	if err = sup.makeRoot(); err != nil {
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

	if err = sup.addScmRepo(rng); err != nil {
		return err
	}

	if !gomake.FileExists(filepath.Join(sup.root, "go.mod")) {
		if err = gmgo.InitModule(ctx, rng, sup.root, sup.module); err != nil {
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

// addScmRepo appends the source repository variable to the project
// configuration file when a Docker repository is configured; it is a no-op
// when none is set.
func (sup *Setup) addScmRepo(rng *ring.Ring) error {
	if sup.repo == "" {
		return nil
	}
	pth := filepath.Join(sup.root, CfgPath)
	fil, err := os.OpenFile(pth, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o666) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()

	line := fmt.Sprintf("%s=%s\n", xdef.EnvScmRepo, sup.repo)
	if _, err = fil.WriteString(line); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "added %s to: %s\n", xdef.EnvScmRepo, pth)
	return nil
}

// initScmRepo initializes a git repository at the setup root, creates the
// initial commit, and tags it v0.0.0.
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
