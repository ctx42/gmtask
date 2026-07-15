// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmprj"
)

// ImgName returns Docker image name based on project name.
func ImgName(projectName string) string {
	if strings.HasPrefix(projectName, "dki-") ||
		projectName == "" ||
		strings.Contains(projectName, "-dki-") {

		return projectName
	}
	return "dki-" + projectName
}

// splitTargets splits list of comma separated target names.
func splitTargets(targets string) []string {
	split := strings.Split(targets, ",")
	ret := make([]string, 0, len(split))
	for i := range split {
		if val := strings.TrimSpace(split[i]); val != "" {
			ret = append(ret, val)
		}
	}
	return ret
}

// pickTargets checks all wanted tags in wantTgs are in haveTgs and returns
// them as slice with the same order as in haveTgs. Returns error when:
//   - haveTgs is empty and wantTgs is not,
//   - wantTgs has names that are not in haveTgs.
func pickTargets(haveTgs, wantTgs []string) ([]string, error) {
	if len(haveTgs) == 0 {
		if len(wantTgs) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrNoTargets, wantTgs)
	}

	if len(haveTgs) > 0 && len(wantTgs) == 0 {
		return haveTgs, nil
	}

	// Clone so the caller-owned slice is not mutated by slices.Delete below.
	wantTgs = slices.Clone(wantTgs)
	var picked []string
	for _, haveTgt := range haveTgs {
		if i := slices.Index(wantTgs, haveTgt); i > -1 {
			picked = append(picked, haveTgt)
			wantTgs = slices.Delete(wantTgs, i, i+1)
		}
	}

	if len(wantTgs) > 0 {
		return nil, fmt.Errorf("%w: %v", ErrNoTarget, wantTgs)
	}

	return picked, nil
}

// updateInfo adds project info custom variables from builds.
// NOTICE: It assumes all builds share the same private repo and tag.
func updateInfo(inf *gmprj.Info, bls []*Build) {
	if len(bls) == 1 {
		bld := bls[0] //nolint:gosec
		inf.Set(EnvDkrImgName, bld.ImgName())
		inf.Set(EnvDkrImgTag, bld.ImgTag())
		inf.Set(EnvDkrImgRef, bld.ImgRef())
		return
	}

	if len(bls) > 0 {
		bld := bls[0]
		inf.Set(EnvDkrImgNameStem, bld.ImgStem())
		inf.Set(EnvDkrImgTag, bld.ImgTag())
	}

	var refs, names []string
	for _, bld := range bls {
		refs = append(refs, bld.ImgRef())
		names = append(names, bld.ImgName())
	}

	if len(refs) > 0 {
		inf.Set(EnvDkrImgNames, strings.Join(names, ","))
		inf.Set(EnvDkrImgRefs, strings.Join(refs, ","))
	}
}

// sshAuthSock returns value of SSH_AUTH_SOCK environment variable.
func sshAuthSock(env []string) string {
	if val, set := gomake.LookupEnv(env, EnvSSHSock); set {
		return os.Expand(val, gomake.Expander(env))
	}
	return ""
}

// runDockerCmd runs docker command with arguments and environment. Everything
// "docker" prints to standard output or standard error is printed without
// modification to the same streams. Returns copy of messages printed to
// standard output and standard error and execution error if any. Whitespace is
// trimmed on both returned streams.
func runDockerCmd(ctx context.Context, rng *ring.Ring) (string, string, error) {
	sin, sout, eout := rng.Stdin(), rng.Stdout(), rng.Stderr()

	// Printing to standard output and returning it so the caller can examine it.
	soutCatch := &bytes.Buffer{}
	soutMulti := io.MultiWriter(sout, soutCatch)

	// Printing to standard error and returning it so the caller can examine it.
	eoutCatch := &bytes.Buffer{}
	eoutMulti := io.MultiWriter(eout, eoutCatch)

	args := slices.Clone(rng.Args())
	for i := range args {
		args[i] = os.Expand(args[i], gomake.Expander(rng.EnvAll()))
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = sin, soutMulti, eoutMulti
	cmd.Env = rng.EnvAll()

	err := cmd.Run()
	soutS := strings.TrimSpace(soutCatch.String())
	eoutS := strings.TrimSpace(eoutCatch.String())

	if err != nil {
		both := strings.TrimSpace(soutS + "\n" + eoutS)
		return soutS, eoutS, dockerErrorOr(both, err)
	}

	return soutS, eoutS, nil
}

// rxERROR is regular expression finding lines starting with "ERROR:" string.
var rxERROR = regexp.MustCompile("(?m)^ERROR: (.*)")

// filterError keeps only lines starting with "ERROR:" in given message.
func filterError(msg string) string {
	sm := rxERROR.FindAllStringSubmatch(msg, -1)
	var ret []string
	for _, m := range sm {
		ret = append(ret, m[0])
	}
	if len(ret) == 0 {
		return msg
	}
	return strings.Join(ret, "\n")
}

// dockerErrorOr takes error message printed by the docker command and returns
// a matching sentinel error or an error with the message text. If the message
// text is empty err will be returned.
func dockerErrorOr(msg string, err error) error {
	msg = filterError(msg)
	switch {
	case msg == "":
		if err == nil {
			msg = "empty docker error message and nil error parameter"
			return errors.New(msg)
		}
		return err

	case strings.Contains(msg, "failed to solve: target stage"):
		return ErrUnkTarget

	default:
		return errors.New(msg)
	}
}

// isRemoteSet returns true if docker remote repository is set.
func isRemoteSet(cfg map[string]string) bool {
	host := cfg[xdef.EnvRegHost]
	repo := cfg[xdef.EnvRegRepo]
	return host != "" && repo != ""
}

// GetGIDbyName returns system group ID by its name.
func GetGIDbyName(name string) (int, error) {
	sout, eout := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := exec.Command("getent", "group", name)
	cmd.Stdout, cmd.Stderr = sout, eout
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(eout.String())
		return 0, fmt.Errorf("getent group %q: %w: %s", name, err, msg)
	}
	return parseGetent(sout.String())
}

// parseGetent parses "getent group grp_name" command result and returns
// group ID.
func parseGetent(line string) (int, error) {
	elems := strings.Split(line, ":")
	if len(elems) != 4 {
		return 0, errors.New("unexpected getent response format")
	}
	gid, err := strconv.Atoi(elems[2])
	if err != nil {
		return 0, fmt.Errorf("parsing group id %q: %w", elems[2], err)
	}
	return gid, nil
}

// DockerSocket returns current Docker socket. On error it will return default
// value "/var/run/docker.sock".
func DockerSocket() string {
	sout, eout := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := exec.Command("docker", "context", "ls", "--format", "json")
	cmd.Stdout, cmd.Stderr = sout, eout
	if err := cmd.Run(); err != nil {
		return "/var/run/docker.sock"
	}

	dst := struct {
		Current        bool
		DockerEndpoint string
	}{}

	dec := json.NewDecoder(sout)
	for {
		err := dec.Decode(&dst)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "/var/run/docker.sock"
		}
		if dst.Current {
			if after, ok := strings.CutPrefix(dst.DockerEndpoint, "unix://"); ok {
				return after
			}
			return "/var/run/docker.sock"
		}
	}
	return "/var/run/docker.sock"
}

// deleteImage deletes an image by ID with force.
func deleteImage(ctx context.Context, rng *ring.Ring, id string) error {
	sout, eout := &bytes.Buffer{}, &bytes.Buffer{}
	argsRm := []string{"rmi", "--force", id}
	rngDC := rng.Clone()
	rngDC.SetStdout(sout)
	rngDC.SetStderr(eout)
	rngDC.SetArgs(argsRm)
	_, _, err := runDockerCmd(ctx, rngDC)
	return err
}
