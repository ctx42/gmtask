// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Sentinel errors.
var (
	// ErrEmptyTag is the error returned when the Docker image tag is empty.
	ErrEmptyTag = errors.New("image tag must not be empty")

	// ErrEmptyName is the error returned when the Docker image name is empty.
	ErrEmptyName = errors.New("image name must not be empty")
)

type hidBC = Config // Don't export embedded struct.

// Build represents single "docker build" command.
type Build struct {
	hidBC
}

// NewBuild returns new instance of Build.
func NewBuild(cfg Config) (*Build, error) {
	bld := &Build{hidBC: cfg}
	if bld.name == "" {
		return nil, ErrEmptyName
	}
	if bld.tag == "" {
		return nil, ErrEmptyTag
	}
	if bld.args == nil {
		bld.args = make(map[string]string, 10)
	}
	return bld, nil
}

// ImgStem returns image name without target. If target is not set it returns
// the same value as ImgName method.
func (bld *Build) ImgStem() string {
	ref := bld.name
	if bld.repo != "" {
		ref = bld.repo + "/" + ref
	}
	return ref
}

// ImgName returns Docker image name.
//
// Examples:
//   - with private repo: my.nexus.dev:5000/repo/image
//   - without private repo: image
//   - with target: image-target
//   - with target and private repo: my.nexus.dev:5000/repo/image-target
func (bld *Build) ImgName() string {
	ref := bld.ImgStem()
	if bld.target != "" {
		ref += "-" + bld.target
	}
	return ref
}

// ImgRef returns Docker image reference.
//
// Reference is private repo + image name + image tag
//
// Examples:
//   - example.com/repo/project:tag
//   - project:tag
func (bld *Build) ImgRef() string {
	return fmt.Sprintf("%s:%s", bld.ImgName(), bld.tag)
}

// ImgRefLatest returns latest Docker image reference. (prvRepo+imgName+latest).
//
// Reference is private repo + image name + "latest"
//
// Examples:
//   - example.com/repo/project:latest
//   - project:latest
func (bld *Build) ImgRefLatest() string {
	return bld.ImgName() + ":latest"
}

// ImgTag returns Docker image tag.
func (bld *Build) ImgTag() string {
	return bld.tag
}

// Env returns environment to use for "docker build" command.
func (bld *Build) Env() []string {
	if bld.kit {
		return []string{"DOCKER_BUILDKIT=1"}
	}
	return nil
}

// Cmd returns a "docker build" command that builds from "./Dockerfile" in the
// current working directory. The last element is always the context path ".".
func (bld *Build) Cmd() []string {
	return append(bld.cmdArgs(), "--file", "Dockerfile", ".")
}

// cmdArgs returns the shared "docker build" prefix without the Dockerfile path
// or build context.
func (bld *Build) cmdArgs() []string {
	cmd := []string{"build", "--platform", bld.platform}
	if bld.ssh != "" {
		cmd = append(cmd, "--ssh", fmt.Sprintf("default=%s", bld.ssh))
	}
	cmd = append(cmd, "-t", bld.ImgRef())
	if bld.tag != "latest" && bld.latest {
		cmd = append(cmd, "-t", bld.ImgRefLatest())
	}
	if bld.target != "" {
		cmd = append(cmd, "--target", bld.target)
	}
	if bld.noCache {
		cmd = append(cmd, "--no-cache")
	}
	names := make([]string, 0, len(bld.args))
	for name := range bld.args {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		arg := fmt.Sprintf("%s=%s", name, bld.args[name])
		cmd = append(cmd, "--build-arg", arg)
	}
	return cmd
}

// String returns the build command as a string.
func (bld *Build) String() string {
	env := strings.Join(bld.Env(), " ")
	if env != "" {
		env += " "
	}
	return env + "docker " + strings.Join(bld.Cmd(), " ")
}
