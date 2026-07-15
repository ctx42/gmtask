// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gmdkr provides gomake targets for building, running, and publishing
// a project's Docker images.
package gmdkr

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"
	"github.com/ctx42/xflag/pkg/xflag"

	"github.com/ctx42/gmtask/pkg/gmprj"
)

// shim is a simple Dockerfile used to create image with users and groups
// matching the current user.
//
//go:embed data/Dockerfile
var shim string

// EnvSSHSock is the well-known environment variable pointing at the SSH agent
// socket exposed to "docker build".
const EnvSSHSock = "SSH_AUTH_SOCK"

// Environment variable names. The variables are set based on the project state.
const (
	// EnvDkrImgNameStem holds stem of Docker image file. The value is the
	// image name stem used as a prefix for image names when
	// [xdef.EnvBldTargets] is set in the project configuration file.
	EnvDkrImgNameStem = "C42_DKI_NAME_STEM"

	// EnvDkrImgName holds Docker image name. Set when project builds only one
	// image.
	EnvDkrImgName = "C42_DKI_NAME"

	// EnvDkrImgNames holds comma-delimited list of Docker images. Set when
	// [xdef.EnvBldTargets] is used and project builds more than one image.
	EnvDkrImgNames = "C42_DKI_NAMES"

	// EnvDkrImgTag is an environment variable name representing Docker image tag.
	// In the docker image reference:
	//
	//   my.nexus.dev:5000/repo/image:1.2.3
	//
	// the tag is "1.2.3".
	EnvDkrImgTag = "C42_DKI_TAG"

	// EnvDkrImgRef is environment variable name representing docker image
	// reference. The image reference is constructed from Docker private repo
	// (if applicable), image name and image tag. Example:
	//
	//   my.nexus.dev:5000/repo/image:1.2.3
	EnvDkrImgRef = "C42_DKI_REF"

	// EnvDkrImgRefs is environment variable name representing comma delimited
	// list of Docker images references. Set when [xdef.EnvBldTargets] is used in
	// the project configuration file and project builds more than one image.
	EnvDkrImgRefs = "C42_DKI_REFS"
)

// goImageLatest is the latest Docker image reference with Go and test tools
// installed.
const goImageLatest = "ghcr.io/ctx42/dkigo-test:latest"

// Sentinel errors.
var (
	// ErrNoTargets is returned when --targets/-T names are given but the
	// project defines no Docker targets.
	ErrNoTargets = errors.New("no targets defined")

	// ErrNoTarget is returned when --targets/-T names a target that is not
	// defined by [xdef.EnvBldTargets].
	ErrNoTarget = errors.New("unknown target")

	// ErrNoDockerfile is returned when no Dockerfile is found.
	ErrNoDockerfile = errors.New("no Dockerfile found")

	// ErrNoPrvRepo is returned when private Docker repository is required.
	ErrNoPrvRepo = errors.New("private docker repository is required")

	// ErrMultiTarget is returned when multiple targets are picked when only
	// one is allowed.
	ErrMultiTarget = errors.New("can pick only one target")

	// ErrReqTarget is returned when target is required but not provided.
	ErrReqTarget = errors.New("target name required")

	// ErrUnkTarget is returned when target is unknown.
	ErrUnkTarget = errors.New("unknown target name")
)

// Docker groups the ":docker:*" gomake targets.
type Docker struct{} //gomake:ns_root

// Login logs in to Docker private repo.
func (Docker) Login(ctx context.Context, rng *ring.Ring) error {
	fp := NewFlagParser(":docker:login", rng.Stderr())
	fp.Add(FlagHelp)
	if err := fp.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fp.fs.Args())
	if fp.fls.Help {
		fp.fs.Usage()
		return nil
	}
	inf, err := gmprj.GetInfo(ctx, rng.EnvAll(), ".")
	if err != nil {
		return err
	}
	dkrPrvRepo := inf.CfgGet(xdef.EnvRegRepo)
	if dkrPrvRepo == "" {
		format := "docker private repo not configured in %s"
		return fmt.Errorf(format, gmprj.CfgPath)
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "login to %s\n", dkrPrvRepo)
	rngDC := rng.SetArgs([]string{"login", dkrPrvRepo})
	_, _, err = runDockerCmd(ctx, rngDC)
	return err
}

// Image groups the ":docker:image:*" gomake targets.
type Image Docker

// initTarget parses the flags of the target named name and initializes a
// [DockerCmd] for the current working directory. It returns a nil DockerCmd
// and nil error when the help flag was set (usage is printed as a side
// effect); the caller should then return without doing further work.
func initTarget(
	ctx context.Context,
	rng *ring.Ring,
	name string,
	flags ...func(*FlagParser),
) (*DockerCmd, *ring.Ring, error) {

	fp := NewFlagParser(name, rng.Stderr())
	fp.Add(flags...)
	if err := fp.Parse(rng.Args()); err != nil {
		return nil, rng, err
	}
	rng = rng.SetArgs(fp.fs.Args())
	if fp.fls.Help {
		fp.fs.Usage()
		return nil, rng, nil
	}
	dc := NewDockerCmd(fp.fls)
	if err := dc.Init(ctx, rng.EnvAll(), "."); err != nil {
		return nil, rng, err
	}
	return dc, rng, nil
}

// Build builds image(s) based on Dockerfile. Runs docker commands in current
// working directory.
func (Image) Build(ctx context.Context, rng *ring.Ring) error {
	dc, rng, err := initTarget(
		ctx,
		rng,
		":docker:image:build",
		FlagHelp,
		FlagTargets,
		FlagImgName,
		FlagImgTag,
		FlagImgLatest,
		FlagDryRun,
		FlagRebuild,
	)
	if dc == nil {
		return err
	}
	return dc.Build(ctx, rng)
}

// Push pushes image(s) to private Docker repository. Runs docker commands in
// current working directory.
func (Image) Push(ctx context.Context, rng *ring.Ring) error {
	dc, rng, err := initTarget(
		ctx,
		rng,
		":docker:image:push",
		FlagHelp,
		FlagTargets,
		FlagImgName,
		FlagImgTag,
		FlagDryRun,
	)
	if dc == nil {
		return err
	}
	return dc.Push(ctx, rng)
}

// Run runs image or target.
func (Image) Run(ctx context.Context, rng *ring.Ring) error {
	dc, rng, err := initTarget(
		ctx,
		rng,
		":docker:image:run",
		FlagHelp,
		FlagTargets,
		FlagImgName,
		FlagImgTag,
		FlagDryRun,
		FlagRebuild,
	)
	if dc == nil {
		return err
	}
	return dc.Run(ctx, rng)
}

// RunProj runs Docker image in current working directory.
//
// gomake:hidden This does not yet work - especially the UID, GID mapping.
func (Image) RunProj(ctx context.Context, rng *ring.Ring) error {
	fp := NewFlagParser(":docker:image:run-proj", rng.Stderr())
	fp.Add(FlagHelp, FlagDryRun, FlagCmd)
	if err := fp.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fp.fs.Args())
	if fp.fls.Help {
		fp.fs.Usage()
		return nil
	}
	fls := fp.fls

	inf, err := gmprj.GetInfo(ctx, rng.EnvAll(), ".")
	if err != nil {
		return err
	}

	usrUID := os.Getuid()
	usrGID := os.Getgid()
	dkrGID, err := GetGIDbyName("docker")
	if err != nil {
		return err
	}

	cfg := NewConfig(inf.Get(xdef.EnvProjName), "latest")
	cfg.latest = false
	cfg.args[xdef.EnvBldImgBase] = goImageLatest
	cfg.args["C42_USR_UID"] = strconv.Itoa(usrUID)
	cfg.args["C42_USR_GID"] = strconv.Itoa(usrGID)
	cfg.args["C42_DKR_GID"] = strconv.Itoa(dkrGID)

	bld, err := NewBuild(*cfg)
	if err != nil {
		return err
	}

	args := bld.Cmd()
	args = args[:len(args)-3]
	args = append(args, "-")
	_, _ = fmt.Fprintf(
		rng.Stderr(),
		"%s%s\n",
		"#gomake INFO# docker ",
		strings.Join(args, " "),
	)
	if !fls.DryRun {
		rngDC := rng.Clone()
		rngDC.SetStdin(strings.NewReader(shim))
		rngDC.SetArgs(args)
		if _, _, err := runDockerCmd(ctx, rngDC); err != nil {
			return err
		}
	}

	user := fmt.Sprintf("%d:%d", usrUID, usrGID)
	volume := inf.Get(xdef.EnvProjRootDir) + ":/ctx42/project:rw,z"

	args = []string{"run", "--rm", "-it", "-u", user}
	args = append(
		args,
		"-v", volume,
		"-v", DockerSocket()+":/var/run/docker.sock",
	)
	if sock := sshAuthSock(rng.EnvAll()); sock != "" {
		args = append(
			args,
			"-v", sock+":/ctx42/ssh-auth-sock",
			"-e", "SSH_AUTH_SOCK=/ctx42/ssh-auth-sock",
		)
	}
	args = append(
		args,
		"--group-add", strconv.Itoa(dkrGID),
		inf.Get(xdef.EnvProjName)+":latest",
	)
	cmd := fls.Cmd
	if cmd == "" {
		cmd = "/bin/bash"
	}
	args = append(args, strings.Fields(cmd)...)
	args = append(args, fls.Args...)

	_, _ = fmt.Fprintf(
		rng.Stderr(),
		"%s%s\n",
		"#gomake INFO# docker ",
		strings.Join(args, " "),
	)
	if !fls.DryRun {
		rngDC := rng.SetArgs(args)
		if _, _, err := runDockerCmd(ctx, rngDC); err != nil {
			return err
		}
	}
	return nil
}

// Sh runs image shell.
func (Image) Sh(ctx context.Context, rng *ring.Ring) error {
	dc, rng, err := initTarget(
		ctx,
		rng,
		":docker:image:sh",
		FlagHelp,
		FlagTargets,
		FlagImgName,
		FlagImgTag,
		FlagDryRun,
		FlagCmd,
	)
	if dc == nil {
		return err
	}
	return dc.Sh(ctx, rng)
}

// Reference prints docker image reference. When there are multiple targets
// defined in project configuration file you must provide which target name
// you want reference for using command arguments.
func (Image) Reference(ctx context.Context, rng *ring.Ring) error {
	fp := NewFlagParser(":docker:image:reference", rng.Stderr())
	fp.Add(FlagHelp, FlagTargets)
	if err := fp.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fp.fs.Args())
	if fp.fls.Help {
		fp.fs.Usage()
		return nil
	}
	dc := NewDockerCmd(fp.fls)
	if err := dc.Init(ctx, rng.EnvAll(), "."); err != nil {
		return err
	}

	ref, err := dc.Reference()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(rng.Stdout(), ref)
	return nil
}

// Env prints information about project in current working directory in the
// same format as Linux shell command "env". Additional argument may be passed
// to the target in form of environment variable name to display only it's
// value.
func (Image) Env(ctx context.Context, rng *ring.Ring) error {
	fp := NewFlagParser(":docker:image:env", rng.Stderr())
	fp.Add(FlagHelp, FlagExport)
	if err := fp.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fp.fs.Args())
	if fp.fls.Help {
		fp.fs.Usage()
		return nil
	}

	args := rng.Args()
	if len(args) > 1 {
		return gmprj.ErrTooManyArgs
	}

	dc := NewDockerCmd(fp.fls)
	if err := dc.Init(ctx, rng.EnvAll(), "."); err != nil {
		return err
	}

	switch len(args) {
	case 0:
		env := dc.Info.Env()
		if dc.Flags.Export {
			for i := range env {
				env[i] = "export " + env[i]
			}
		}
		_, _ = fmt.Fprint(rng.Stdout(), strings.Join(env, "\n")+"\n")

	case 1:
		_, _ = fmt.Fprint(rng.Stdout(), gomake.Getenv(dc.Info.Env(), args[0]))
	}
	return nil
}

// Info prints information about project in current working directory. Its
// output is more readable version of "docker:image:env" target. Additional
// argument may be passed to the target in form of environment variable name
// to display only it's value.
func (Image) Info(ctx context.Context, rng *ring.Ring) error {
	tgtName := ":docker:image:info"
	eout := rng.Stderr()
	fs := xflag.NewFlagSet(tgtName, flag.ContinueOnError)
	fs.BoolSL("help", "h", false, "show help")
	fs.SetOutput(eout)
	fs.Usage = func() {
		head := fmt.Sprintf("Usage of %s:\n", tgtName)
		examples := "\nEXAMPLES:\n" +
			"\t:docker:image:info\n" +
			"\t:docker:image:info ENV_VAR_NAME\n"
		_, _ = fmt.Fprint(eout, head+xflag.HelpOptions(fs)+examples)
	}
	if err := fs.Parse(rng.Args()); err != nil {
		return err
	}
	rng = rng.SetArgs(fs.Args())
	if fs.GetBool("help") {
		fs.Usage()
		return nil
	}

	args := rng.Args()
	if len(args) > 1 {
		return gmprj.ErrTooManyArgs
	}

	dc := NewDockerCmd(NewFlags(tgtName))
	if err := dc.Init(ctx, rng.EnvAll(), "."); err != nil {
		return err
	}

	switch len(args) {
	case 0:
		return gomake.PrettyPrintEnv(dc.Info.Env(), rng.Stdout())

	case 1:
		_, _ = fmt.Fprint(rng.Stdout(), gomake.Getenv(dc.Info.Env(), args[0]))
	}
	return nil
}

// Clean removes no longer necessary test images.
//
// Removes dangling images and images that repository reference contains
// "ctx42-tst-img-" string that were created more than an hour ago.
func (Image) Clean(ctx context.Context, rng *ring.Ring) error {
	dc := NewDockerCmd(NewFlags(":docker:image:clean"))
	return dc.Clean(ctx, rng)
}
