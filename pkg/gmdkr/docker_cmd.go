// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmprj"
)

// DockerCmd represents a docker command.
type DockerCmd struct {
	// Arguments passed to gomake target.
	Flags *Flags

	// Docker build configuration.
	Config *Config

	// Project info initialized based on the current working directory.
	Info *gmprj.Info

	// Build instances created based on project info and command arguments.
	Builds []*Build
}

// NewDockerCmd returns a new instance of DockerCmd.
func NewDockerCmd(fls *Flags) *DockerCmd {
	return &DockerCmd{Flags: fls}
}

// Init initializes the project information from the project in the root
// directory and creates Build instances based on the gomake arguments.
func (dc *DockerCmd) Init(
	ctx context.Context,
	env []string,
	root string,
) error {

	var err error
	if dc.Info, err = gmprj.GetInfo(ctx, env, root); err != nil {
		return err
	}
	if err = checkDockerfile(dc.Info); err != nil {
		return err
	}

	var wantTgs []string
	haveTgs := splitTargets(dc.Info.CfgGet(xdef.EnvBldImgTargets))
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

// checkDockerfile returns an error when the project in inf has no Dockerfile
// to build: neither its own nor, when it names a Dockerfile module, the
// module's. It returns an error wrapping [ErrTwoDockerfiles] when it has both.
func checkDockerfile(inf *gmprj.Info) error {
	own := filepath.Join(inf.Root, "Dockerfile")
	if inf.DkfDir == "" {
		if !inf.HasDockerfile {
			return ErrNoDockerfile
		}
		return nil
	}
	mod := filepath.Join(inf.DkfDir, "Dockerfile")
	if inf.HasDockerfile {
		return fmt.Errorf("%w: %s and %s", ErrTwoDockerfiles, own, mod)
	}
	if !gomake.FileExists(mod) {
		return fmt.Errorf("%w: %s", ErrNoDockerfile, mod)
	}
	return nil
}

// Build builds docker image(s).
func (dc *DockerCmd) Build(ctx context.Context, rng *ring.Ring) error {
	if dc.Config == nil || dc.Info == nil {
		return ErrNoBuilds
	}
	if dc.Config.push && !isRemoteSet(dc.Info.Config) {
		return ErrNoPrvRepo
	}
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
			// An image pushed while building is the registry's, and one
			// built at a version tag is the version the registry gets or
			// already has. The run keeps both, so neither is recorded for
			// removal: another run on the same daemon may build the same
			// image ID at the same tag, and removing it would pull it from
			// under that run's push.
			if dc.Flags.IDsFile != "" && !dc.Config.push && !dc.Config.release {
				pth, ref := dc.Flags.IDsFile, bld.ImgRef()
				if err := recordImageID(ctx, rng, pth, ref); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Push pushes images to private docker repository. An image allowed to move
// "latest" is tagged "latest" from the pushed reference first, so the pushed
// "latest" is the version just pushed, never an older local one.
func (dc *DockerCmd) Push(ctx context.Context, rng *ring.Ring) error {
	if dc.Info == nil {
		return ErrNoBuilds
	}
	if !isRemoteSet(dc.Info.Config) {
		return ErrNoPrvRepo
	}
	for _, bld := range dc.Builds {
		cmds := [][]string{{"push", bld.ImgRef()}}
		if bld.tagsLatest() {
			cmds = append(
				cmds,
				[]string{"tag", bld.ImgRef(), bld.ImgRefLatest()},
				[]string{"push", bld.ImgRefLatest()},
			)
		}
		for _, args := range cmds {
			_, _ = fmt.Fprintf(
				rng.Stderr(),
				"%s%s\n",
				"#gomake INFO# docker ",
				strings.Join(args, " "),
			)
			if dc.Flags.DryRun {
				continue
			}
			rngDC := rng.Clone()
			rngDC.SetArgs(args)
			if _, _, err := runDockerCmd(ctx, rngDC); err != nil {
				return err
			}
		}
		if dc.Flags.IDsFile != "" && !dc.Flags.DryRun {
			pth, ref := dc.Flags.IDsFile, bld.ImgRef()
			if err := forgetImageRef(pth, ref); err != nil {
				return err
			}
		}
	}
	return nil
}

// Run runs Docker image or target defined by project, with the project
// mounted read-only at the configured [xdef.EnvCtrPrjRoot], "/ctx42/project"
// when the configuration sets none.
func (dc *DockerCmd) Run(ctx context.Context, rng *ring.Ring) error {
	if len(dc.Builds) == 0 {
		return ErrNoBuilds
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

	prjRoot, _ := dc.mounts()
	args := []string{"run", "--rm"}
	volume := dc.Info.Root + ":" + prjRoot + ":ro"
	args = append(args, "-v", volume, ref)
	args = append(args, dc.Flags.Args...)
	_, _ = fmt.Fprintf(
		rng.Stderr(),
		"%s%s\n",
		"#gomake INFO# docker ",
		strings.Join(args, " "),
	)
	if !dc.Flags.DryRun {
		// Interactive or long-running: stream, do not capture.
		if err := runDockerStream(ctx, rng.Clone().SetArgs(args)); err != nil {
			return err
		}
	}
	return nil
}

// Sh runs image and starts its shell. It mounts the project as [DockerCmd.Run]
// does, and the SSH agent socket, when there is one, as "ssh-auth-sock" in the
// configured [xdef.EnvCtrRoot], "/ctx42" when the configuration sets none.
func (dc *DockerCmd) Sh(ctx context.Context, rng *ring.Ring) error {
	if len(dc.Builds) == 0 {
		return ErrNoBuilds
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

	prjRoot, sshSock := dc.mounts()
	args := []string{"run", "--rm", "-it"}
	volume := dc.Info.Root + ":" + prjRoot + ":ro"
	args = append(args, "-v", volume)

	if dc.Config.ssh != "" {
		args = append(
			args,
			"-v", dc.Config.ssh+":"+sshSock,
			"-e", EnvSSHSock+"="+sshSock,
		)
	}

	cmd := dc.Flags.Cmd
	if cmd == "" {
		cmd = "/bin/sh --login"
	}
	cmdArgs, err := splitArgs(cmd)
	if err != nil {
		return err
	}
	args = append(args, ref)
	args = append(args, cmdArgs...)
	args = append(args, dc.Flags.Args...)
	_, _ = fmt.Fprintf(
		rng.Stderr(),
		"%s%s\n",
		"#gomake INFO# docker ",
		strings.Join(args, " "),
	)
	if !dc.Flags.DryRun {
		// Interactive or long-running: stream, do not capture.
		if err := runDockerStream(ctx, rng.Clone().SetArgs(args)); err != nil {
			return err
		}
	}
	return nil
}

// Reference returns the selected image reference.
func (dc *DockerCmd) Reference() (string, error) {
	if len(dc.Builds) == 0 {
		return "", ErrNoBuilds
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
// "ctx42-tst-img-" that were created more than an hour ago.
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

// build builds image with given reference if it does not exist yet. A dry run
// always prints the build command.
func (dc *DockerCmd) build(
	ctx context.Context,
	rng *ring.Ring,
	ref string,
	rebuild bool,
) error {

	// A dry run only prints commands, so it builds without asking the
	// daemon whether the image exists.
	build := rebuild || dc.Flags.DryRun
	if !build {
		// The ring's arguments are the container's; they must not become
		// "docker image ls" filters.
		ims, err := ImgLs(ctx, rng.Clone().SetArgs(nil))
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

// mounts returns where the project and the SSH agent socket are mounted in an
// image built from the project configuration. The build passes the configured
// [xdef.EnvCtrPrjRoot] and [xdef.EnvCtrRoot] to the image as build arguments,
// so they name the image's layout; a key left empty keeps the ctx42 base
// image layout.
func (dc *DockerCmd) mounts() (prjRoot, sshSock string) {
	prjRoot, sshSock = ctrPrjRoot, ctrSSHSock
	if val := dc.Info.CfgGet(xdef.EnvCtrPrjRoot); val != "" {
		prjRoot = val
	}
	if val := dc.Info.CfgGet(xdef.EnvCtrRoot); val != "" {
		sshSock = path.Join(val, ctrSSHSockName)
	}
	return prjRoot, sshSock
}
