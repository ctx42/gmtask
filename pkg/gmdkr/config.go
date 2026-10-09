// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"maps"
	"time"

	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmgo"
	"github.com/ctx42/gmtask/pkg/gmprj"
)

// Config represents docker build configuration.
type Config struct {
	// Docker image name.
	name string

	// Docker image tag.
	tag string

	// Tag docker image with "latest" when building (default: true).
	latest bool

	// Dockerfile target to build. When empty it means default target.
	target string

	// SSH agent socket or keys to expose to the build.
	//   format: default|<id>[=<socket>|<key>[,<key>]]
	ssh string

	// Docker private repository.
	repo string

	// Docker image platform when building (default: linux/amd64).
	platform string

	// Use build kit when building (default: true).
	kit bool

	// Build date.
	buildDate time.Time

	// Dockerfile arguments.
	args map[string]string

	// Do not use cache when building images.
	noCache bool

	// Push the built image(s) to the registry instead of keeping them in the
	// builder.
	push bool

	// External cache specifications "docker build" imports from and
	// exports to, with [cacheImgVar] still unexpanded. Empty keeps only the
	// builder's own cache.
	cacheFrom string
	cacheTo   string
}

// NewConfig returns new instance of Config with default field values. It does
// not move the "latest" tag; only [ConfigFrom] sets that, and only for a
// release asked for with the latest flag.
func NewConfig(name, tag string) *Config {
	return &Config{
		name:      name,
		tag:       tag,
		latest:    false,
		platform:  "linux/amd64",
		args:      make(map[string]string),
		kit:       true,
		buildDate: time.Now().UTC().Truncate(time.Millisecond),
	}
}

// ConfigFrom instantiates Config based on data in project information and
// gomake target arguments.
func ConfigFrom(inf *gmprj.Info, fls *Flags) *Config {
	// The Dockerfiles stamp C42_SCM_REV and the version label from the build
	// argument unchanged, so the assembled revision is what reaches the
	// build. The image tag cannot be that same string - a Docker tag rejects
	// the "+" opening SemVer build metadata - so it takes the sanitized form.
	cfg := NewConfig(ImgName(inf.Get(xdef.EnvPrjName)), ImgTag(inf.Version.Rev))
	if !inf.BuildDate.IsZero() {
		cfg.buildDate = inf.BuildDate
	}

	cfg.repo = inf.CfgGet(xdef.EnvRegRepo)
	if inf.Config != nil {
		cfg.args = maps.Clone(inf.Config)
	}
	cfg.fromInfo(inf, xdef.EnvPrjName)
	cfg.fromInfo(inf, xdef.EnvScmRepo)
	cfg.fromInfo(inf, xdef.EnvScmHash)
	cfg.fromInfo(inf, xdef.EnvScmRev)
	cfg.fromInfo(inf, xdef.EnvRegRepo)
	cfg.fromInfo(inf, EnvSSHSock)
	if val, ok := cfg.args[EnvSSHSock]; ok && val != "" {
		cfg.ssh = val
	}

	if fls != nil {
		if fls.ImgName != "" {
			cfg.name = fls.ImgName
		}
		if fls.ImgTag != "" {
			cfg.tag = fls.ImgTag
		}
		// Only a release may move "latest". A dirty tree or a commit past the
		// tag is not the version the tag names, whatever the flag asks for,
		// and a pre-release tag names a release that has not been cut yet.
		rel := inf.Version.Release && isFinal(inf.Version.Tag)
		cfg.latest = fls.ImgLatest && rel
		cfg.noCache = fls.Rebuild
		cfg.push = fls.Push
		cfg.cacheFrom = fls.CacheFrom
		cfg.cacheTo = fls.CacheTo
	}

	cfg.args[xdef.EnvBldDate] = gmgo.BldDateFmt(cfg.buildDate)
	return cfg
}

// ForTarget clones config and sets target on the clone.
func (cfg *Config) ForTarget(target string) Config {
	cpy := *cfg
	cpy.target = target
	cpy.args = maps.Clone(cfg.args)
	return cpy
}

// Args returns a copy of configuration arguments.
func (cfg *Config) Args() map[string]string {
	return maps.Clone(cfg.args)
}

// fromInfo copies name from inf into the config argument map when the key is
// present, including when its value is empty.
func (cfg *Config) fromInfo(inf *gmprj.Info, name string) {
	if val, ok := inf.Lookup(name); ok {
		cfg.args[name] = val
	}
}
