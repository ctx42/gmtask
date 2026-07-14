// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/internal/gmtest"
	"github.com/ctx42/gmtask/pkg/gmprj"
)

func Test_NewConfig(t *testing.T) {
	// --- When ---
	cfg := NewConfig("project", "tag")

	// --- Then ---
	assert.Equal(t, "project", cfg.name)
	assert.Equal(t, "tag", cfg.tag)
	assert.True(t, cfg.latest)
	assert.Equal(t, "", cfg.target)
	assert.Equal(t, "", cfg.ssh)
	assert.Equal(t, "", cfg.repo)
	assert.Equal(t, "linux/amd64", cfg.platform)
	assert.NotNil(t, cfg.args)
	assert.True(t, cfg.kit)
	assert.Within(t, time.Now(), "5ms", cfg.buildDate)
	assert.Len(t, 0, cfg.args)
	assert.False(t, cfg.noCache)
	assert.Fields(t, 11, Config{})
}

func Test_ConfigFrom(t *testing.T) {
	t.Run("nil info config and nil target args", func(t *testing.T) {
		// --- Given ---
		inf := &gmprj.Info{}

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.NotZero(t, cfg)
	})

	t.Run("empty info", func(t *testing.T) {
		// --- Given ---
		inf := &gmprj.Info{}

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Equal(t, "", cfg.name)
		assert.Equal(t, "", cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "", cfg.ssh)
		assert.Equal(t, "", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Within(t, time.Now(), "1s", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		have := must.Value(time.Parse(
			time.RFC3339Nano,
			cfg.args[xdef.EnvBuildDate],
		))
		assert.Within(t, time.Now(), "1s", have)
		assert.Len(t, 1, cfg.args)
	})

	t.Run("minimal project structure", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, "", cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "", cfg.ssh)
		assert.Equal(t, "", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		assert.HasKeyValue(t, xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z", cfg.args)
		assert.HasKeyValue(t, xdef.EnvCCID, xdef.PhUnknown, cfg.args)
		assert.Len(t, 2, cfg.args)
	})

	t.Run("project with local git no tags", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll()
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, cm.Hash, cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "", cfg.ssh)
		assert.Equal(t, "", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		assert.HasKeyValue(t, xdef.EnvScmHash, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvScmRev, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z", cfg.args)
		assert.HasKeyValue(t, xdef.EnvCCID, xdef.PhUnknown, cfg.args)
		assert.Len(t, 4, cfg.args)
	})

	t.Run("project with remote git no tags", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll()
		prj.GitSetRemote(prjkit.GitOrigin)
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, cm.Hash, cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "", cfg.ssh)
		assert.Equal(t, "", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		assert.HasKeyValue(t, xdef.EnvScmRepo, prjkit.GitOrigin, cfg.args)
		assert.HasKeyValue(t, xdef.EnvScmHash, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvScmRev, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z", cfg.args)
		assert.HasKeyValue(t, xdef.EnvCCID, xdef.PhUnknown, cfg.args)
		assert.Len(t, 5, cfg.args)
	})

	t.Run("project with git and tagged HEAD", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, cm.Rev, cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "", cfg.ssh)
		assert.Equal(t, "", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		assert.HasKeyValue(t, xdef.EnvScmHash, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvScmRev, cm.Rev, cfg.args)
		assert.HasKeyValue(t, xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z", cfg.args)
		assert.HasKeyValue(t, xdef.EnvCCID, xdef.PhUnknown, cfg.args)
		assert.Len(t, 4, cfg.args)
	})

	t.Run("project with git and tag and commit", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GoModInit()
		prj.GitInitAddAll("v1.2.3")
		prj.CreateFile("file.txt")
		cm := prj.GitCommit("")
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		rev := fmt.Sprintf("v1.2.3-1-g%s", cm.Hash)

		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, rev, cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "", cfg.ssh)
		assert.Equal(t, "", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		assert.HasKeyValue(t, xdef.EnvScmHash, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvScmRev, rev, cfg.args)
		assert.HasKeyValue(t, xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z", cfg.args)
		assert.HasKeyValue(t, xdef.EnvCCID, xdef.PhUnknown, cfg.args)
		assert.Len(t, 4, cfg.args)
	})

	t.Run("project with docker repo in project's config file", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, "v1.2.3", cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "", cfg.ssh)
		assert.Equal(t, "my.nexus.dev/repo", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		assert.HasKeyValue(t, xdef.EnvScmHash, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvScmRev, "v1.2.3", cfg.args)
		assert.HasKeyValue(t, xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z", cfg.args)
		assert.HasKeyValue(t, CfgDkrRegHost, "my.nexus.dev", cfg.args)
		assert.HasKeyValue(t, CfgDkrRepo, "my.nexus.dev/repo", cfg.args)
		assert.HasKeyValue(t, xdef.EnvCCID, xdef.PhUnknown, cfg.args)
		assert.Len(t, 6, cfg.args)
	})

	t.Run("project build in CI/CD", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvUnset(EnvSSHSock)
		rng.EnvSet(xdef.EnvCCID, "cc-tag")
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, "v1.2.3", cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "", cfg.ssh)
		assert.Equal(t, "my.nexus.dev/repo", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		assert.HasKeyValue(t, xdef.EnvScmHash, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvScmRev, "v1.2.3", cfg.args)
		assert.HasKeyValue(t, xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z", cfg.args)
		assert.HasKeyValue(t, xdef.EnvCCID, "cc-tag", cfg.args)
		assert.HasKeyValue(t, CfgDkrRegHost, "my.nexus.dev", cfg.args)
		assert.HasKeyValue(t, CfgDkrRepo, "my.nexus.dev/repo", cfg.args)
		assert.Len(t, 6, cfg.args)
	})

	t.Run("with SSH socket set", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()
		rng.EnvSet(EnvSSHSock, "ssh-sock")
		rng.EnvUnset(xdef.EnvCCID)
		rng.EnvSet(xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z")

		prj := gmtest.NewProject(t)
		prj.CfgRegRepoDef()
		prj.GoModInit()
		cm := prj.GitInitAddAll("v1.2.3")
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, "v1.2.3", cfg.tag)
		assert.True(t, cfg.latest)
		assert.Equal(t, "", cfg.target)
		assert.Equal(t, "ssh-sock", cfg.ssh)
		assert.Equal(t, "my.nexus.dev/repo", cfg.repo)
		assert.Equal(t, "linux/amd64", cfg.platform)
		assert.True(t, cfg.kit)
		assert.Time(t, "2000-01-02T03:04:05.6Z", cfg.buildDate)
		assert.False(t, cfg.noCache)
		assert.Fields(t, 11, Config{})

		assert.HasKeyValue(t, xdef.EnvScmHash, cm.Hash, cfg.args)
		assert.HasKeyValue(t, xdef.EnvScmRev, "v1.2.3", cfg.args)
		assert.HasKeyValue(t, EnvSSHSock, "ssh-sock", cfg.args)
		assert.HasKeyValue(t, xdef.EnvBuildDate, "2000-01-02T03:04:05.6Z", cfg.args)
		assert.HasKeyValue(t, CfgDkrRegHost, "my.nexus.dev", cfg.args)
		assert.HasKeyValue(t, CfgDkrRepo, "my.nexus.dev/repo", cfg.args)
		assert.HasKeyValue(t, xdef.EnvCCID, xdef.PhUnknown, cfg.args)
		assert.Len(t, 7, cfg.args)
	})

	t.Run("project info zero build date", func(t *testing.T) {
		// --- Given ---
		inf := &gmprj.Info{}

		// --- When ---
		cfg := ConfigFrom(inf, nil)

		// --- Then ---
		assert.Within(t, time.Now(), "10ms", cfg.buildDate)
	})

	t.Run("flag rebuild set", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		fls := NewFlags("name")
		fls.Rebuild = true

		// --- When ---
		cfg := ConfigFrom(inf, fls)

		// --- Then ---
		assert.True(t, cfg.noCache)
	})

	t.Run("target args override image name", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		cm := prj.GitInitAddAll()
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		fls := NewFlags("name")
		fls.ImgName = "my-name"

		// --- When ---
		cfg := ConfigFrom(inf, fls)

		// --- Then ---
		assert.Equal(t, "my-name", cfg.name)
		assert.Equal(t, cm.Hash, cfg.tag)
	})

	t.Run("target args override image tag", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)
		rng := tst.Ring()

		prj := gmtest.NewProject(t)
		prj.WithConfig()
		prj.GitInitAddAll()
		prj.Close()

		inf := must.Value(gmprj.GetInfo(ctx, rng.EnvAll(), prj.Root()))

		fls := NewFlags("name")
		fls.ImgTag = "my-tag"

		// --- When ---
		cfg := ConfigFrom(inf, fls)

		// --- Then ---
		assert.Equal(t, "dki-project", cfg.name)
		assert.Equal(t, "my-tag", cfg.tag)
	})
}

func Test_Config_ForTarget(t *testing.T) {
	// --- Given ---
	cfg := &Config{
		name:      "project",
		tag:       "cfg-tag",
		latest:    true,
		target:    "other",
		ssh:       "ssh",
		repo:      "repo",
		platform:  "platform",
		args:      map[string]string{xdef.EnvScmRev: "scm-rev"},
		kit:       true,
		buildDate: time.Date(2000, 1, 2, 3, 4, 5, 600_000_000, time.UTC),
		noCache:   true,
	}

	// --- When ---
	have := cfg.ForTarget("target")

	// --- Then ---
	assert.Equal(t, "project", have.name)
	assert.Equal(t, "cfg-tag", have.tag)
	assert.True(t, have.latest)
	assert.Equal(t, "target", have.target)
	assert.Equal(t, "ssh", have.ssh)
	assert.Equal(t, "repo", have.repo)
	assert.Equal(t, "platform", have.platform)
	wantArgs := map[string]string{xdef.EnvScmRev: "scm-rev"}
	assert.Equal(t, wantArgs, have.args)
	assert.True(t, have.kit)
	assert.Time(t, "2000-01-02T03:04:05.6Z", have.buildDate)
	assert.True(t, have.noCache)
	assert.Fields(t, 11, Config{})

	// Test it's independent.
	cfg.target = "other"
	cfg.args[xdef.EnvScmRev] = "change"

	assert.Equal(t, "target", have.target)
	assert.Equal(t, "scm-rev", have.args[xdef.EnvScmRev])
}

func Test_Config_fromInfo(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		// --- Given ---
		inf := gmprj.NewInfo(os.Environ())
		inf.Set("KEY0", "VAL0")
		inf.Set("KEY1", "VAL1")

		cfg := NewConfig("name", "tag")

		// --- When ---
		cfg.fromInfo(inf, "KEY0")

		// --- Then ---
		want := map[string]string{
			"KEY0": "VAL0",
		}
		assert.Equal(t, want, cfg.Args())
	})

	t.Run("not existing key", func(t *testing.T) {
		// --- Given ---
		inf := gmprj.NewInfo(os.Environ())
		inf.Set("KEY0", "VAL0")
		inf.Set("KEY1", "VAL1")

		cfg := NewConfig("name", "tag")

		// --- When ---
		cfg.fromInfo(inf, "KEY2")

		// --- Then ---
		assert.Empty(t, cfg.Args())
	})

	t.Run("empty key value", func(t *testing.T) {
		// --- Given ---
		inf := gmprj.NewInfo(os.Environ())
		inf.Set("KEY0", "VAL0")
		inf.Set("KEY1", "")

		cfg := NewConfig("name", "tag")

		// --- When ---
		cfg.fromInfo(inf, "KEY1")

		// --- Then ---
		want := map[string]string{
			"KEY1": "",
		}
		assert.Equal(t, want, cfg.Args())
	})
}
