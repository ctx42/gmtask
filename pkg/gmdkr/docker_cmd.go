// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmprj"
)

// DockerCmd represents docker command.
type DockerCmd struct {
	// Arguments passed to gomake target.
	Flags *Flags

	// Docker build configuration.
	Config *Config

	// Project info initialized based on current working directory,
	Info *gmprj.Info

	// Build instances created based on project info and command arguments.
	Builds []*Build
}

// NewDockerCmd returns new instance of DockerCmd.
func NewDockerCmd(fls *Flags) *DockerCmd {
	return &DockerCmd{Flags: fls}
}

// Init based on gomake arguments and project in root directory initializes
// project information and creates Build instances.
func (dc *DockerCmd) Init(
	ctx context.Context,
	env []string,
	root string,
) error {

	var err error
	if dc.Info, err = gmprj.GetInfo(ctx, env, root); err != nil {
		return err
	}
	if !dc.Info.HasDockerfile {
		return ErrNoDockerfile
	}

	var wantTgs []string
	haveTgs := splitTargets(dc.Info.CfgGet(xdef.EnvBldTargets))
	if wantTgs, err = pickTargets(haveTgs, dc.Flags.Targets); err != nil {
		return err
	}

	dc.Builds = nil

	// No targets means instance for default build.
	if len(wantTgs) == 0 {
		wantTgs = append(wantTgs, "")
	}

	dc.Config = ConfigFrom(dc.Info, dc.Flags)
	dc.Config.ssh = sshAuthSock(env)
	for _, tgt := range wantTgs {
		bld, err := NewBuild(dc.Config.ForTarget(tgt))
		if err != nil {
			return err
		}
		dc.Builds = append(dc.Builds, bld)
	}
	updateInfo(dc.Info, dc.Builds)
	return nil
}

// Build builds docker image(s).
func (dc *DockerCmd) Build(ctx context.Context, rng *ring.Ring) error {
	if dc.Config.ssh == "" {
		msg := "#gomake WARN# SSH_AUTH_SOCK not set in the environment\n"
		_, _ = fmt.Fprint(rng.Stderr(), msg)
	}
	for _, bld := range dc.Builds {
		_, _ = fmt.Fprintf(
			rng.Stderr(),
			"%s%s\n",
			"#gomake INFO# ",
			bld.String(),
		)
		if !dc.Flags.DryRun {
			rngDC := rng.Clone()
			rngDC.SetArgs(bld.Cmd())
			rngDC.EnvSetWith(bld.Env())
			if _, _, err := runDockerCmd(ctx, rngDC); err != nil {
				return err
			}
		}
	}
	return nil
}

// Push pushes images to private docker repository.
func (dc *DockerCmd) Push(ctx context.Context, rng *ring.Ring) error {
	if !isRemoteSet(dc.Info.Config) {
		return ErrNoPrvRepo
	}
	for _, bld := range dc.Builds {
		args := []string{"push", bld.ImgRef()}
		_, _ = fmt.Fprintf(
			rng.Stderr(),
			"%s%s\n",
			"#gomake INFO# docker ",
			strings.Join(args, " "),
		)
		if !dc.Flags.DryRun {
			rngDC := rng.Clone()
			rngDC.SetArgs(args)
			if _, _, err := runDockerCmd(ctx, rngDC); err != nil {
				return err
			}
		}
	}
	return nil
}

// Run runs Docker image or target defined by project.
func (dc *DockerCmd) Run(ctx context.Context, rng *ring.Ring) error {
	if len(dc.Builds) == 0 {
		return ErrNoTarget
	}
	if len(dc.Builds) > 1 {
		argTgs := dc.Flags.Targets
		if len(argTgs) > 0 {
			return ErrMultiTarget
		}
		return ErrReqTarget
	}

	ref := dc.Builds[0].ImgRef()
	if err := dc.build(ctx, rng, ref, dc.Flags.Rebuild); err != nil {
		return err
	}

	args := []string{"run", "--rm"}
	volume := dc.Info.Get(xdef.EnvProjRootDir) + ":/ctx42/project:ro"
	args = append(args, "-v", volume, ref)
	args = append(args, dc.Flags.Args...)
	_, _ = fmt.Fprintf(
		rng.Stderr(),
		"%s%s\n",
		"#gomake INFO# docker ",
		strings.Join(args, " "),
	)
	if !dc.Flags.DryRun {
		rngDC := rng.Clone()
		rngDC.SetArgs(args)
		if _, _, err := runDockerCmd(ctx, rngDC); err != nil {
			return err
		}
	}
	return nil
}

// Sh runs image and starts its shell.
func (dc *DockerCmd) Sh(ctx context.Context, rng *ring.Ring) error {
	if len(dc.Builds) == 0 {
		return ErrNoTarget
	}
	if len(dc.Builds) > 1 {
		argTgs := dc.Flags.Targets
		if len(argTgs) > 0 {
			return ErrMultiTarget
		}
		return ErrReqTarget
	}

	ref := dc.Builds[0].ImgRef()

	if err := dc.build(ctx, rng, ref, dc.Flags.Rebuild); err != nil {
		return err
	}

	args := []string{"run", "--rm", "-it"}
	volume := dc.Info.Get(xdef.EnvProjRootDir) + ":/ctx42/project:ro"
	args = append(args, "-v", volume)

	if dc.Config.ssh != "" {
		args = append(
			args,
			"-v", dc.Config.ssh+":/ssh-sock",
			"-e", "SSH_AUTH_SOCK=/ssh-sock",
		)
	}

	cmd := dc.Flags.Cmd
	if cmd == "" {
		cmd = "/bin/sh --login"
	}
	args = append(args, ref)
	args = append(args, strings.Fields(cmd)...)
	args = append(args, dc.Flags.Args...)
	_, _ = fmt.Fprintf(
		rng.Stderr(),
		"%s%s\n",
		"#gomake INFO# docker ",
		strings.Join(args, " "),
	)
	if !dc.Flags.DryRun {
		rngDC := rng.Clone()
		rngDC.SetArgs(args)
		if _, _, err := runDockerCmd(ctx, rngDC); err != nil {
			return err
		}
	}
	return nil
}

// Reference prints image reference.
func (dc *DockerCmd) Reference() (string, error) {
	if len(dc.Builds) == 0 {
		return "", ErrNoTarget
	}
	argTgs := dc.Flags.Targets
	if len(argTgs) == 0 && len(dc.Builds) > 1 {
		return "", ErrReqTarget
	}
	if len(argTgs) > 1 {
		return "", ErrMultiTarget
	}
	return dc.Builds[0].ImgRef(), nil
}

// Clean removes dangling images and images whose repository reference contains
// the "ctx42-tst-img-" string that were created more than an hour ago.
func (dc *DockerCmd) Clean(ctx context.Context, rng *ring.Ring) error {
	// Clone so the dangling filter does not stick on rng for later ImgLs calls.
	ims, err := ImgLs(ctx, rng.Clone().SetArgs([]string{"-f", "dangling=true"}))
	if err != nil {
		return err
	}
	ims.RemoveDuplicates()

	for _, img := range ims {
		if err = deleteImage(ctx, rng, img.ID); err != nil {
			return fmt.Errorf("delete dangling image %s: %w", img.ID, err)
		}
	}

	ims, err = ImgLs(ctx, rng)
	if err != nil {
		return err
	}
	ims.RemoveDuplicates()

	for _, img := range ims {
		if img.CreatedAt.After(time.Now().Add(-1 * time.Hour)) {
			continue
		}
		if !strings.Contains(img.Repository, "ctx42-tst-img-") {
			continue
		}
		if err = deleteImage(ctx, rng, img.ID); err != nil {
			return fmt.Errorf("delete image %s: %w", img.ID, err)
		}
	}
	return nil
}

// build builds image with given reference if it does not exist yet.
func (dc *DockerCmd) build(
	ctx context.Context,
	rng *ring.Ring,
	ref string,
	rebuild bool,
) error {

	build := rebuild
	if !rebuild {
		ims, err := ImgLs(ctx, rng)
		if err != nil {
			return err
		}
		if ims.Find(ref) == nil {
			build = true
		}
	}
	if build {
		if err := dc.Build(ctx, rng); err != nil {
			return err
		}
	}
	return nil
}
