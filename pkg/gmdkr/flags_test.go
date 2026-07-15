// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"bytes"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_NewFlagParser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		buf := &bytes.Buffer{}

		// --- When ---
		fp := NewFlagParser("name", buf)

		// --- Then ---
		assert.Equal(t, "name", fp.fs.Name())
		assert.Equal(t, "name", fp.fls.Name)
		assert.Same(t, buf, fp.fs.Output())
		assert.Empty(t, buf.String())
	})

	t.Run("usage", func(t *testing.T) {
		// --- Given ---
		buf := &bytes.Buffer{}
		fp := NewFlagParser("name", buf)
		fp.fs.String("name", "def", "usage")

		// --- When ---
		fp.fs.Usage()

		// --- Then ---
		want := "" +
			"Usage of name:\n" +
			"      --name    usage\n"
		assert.Equal(t, want, buf.String())
	})
}

func Test_FlagParser_Parse(t *testing.T) {
	t.Run("parse all arguments", func(t *testing.T) {
		// --- Given ---
		buf := &bytes.Buffer{}
		args := []string{
			"--name", "img-name-value",
			"--tag", "img-tag-value",
			"--targets", "targets-value0,targets-value1",
			"--dry-run",
			"--rebuild",
			"--export",
			"--cmd", "cmd-value",
		}
		fp := NewFlagParser(":name", buf)
		fp.Add(
			FlagTargets,
			FlagImgName,
			FlagImgTag,
			FlagImgLatest,
			FlagDryRun,
			FlagRebuild,
			FlagExport,
			FlagCmd,
		)

		// --- When ---
		err := fp.Parse(args)

		// --- Then ---
		assert.NoError(t, err)

		fls := fp.fls
		assert.Equal(t, ":name", fls.Name)
		assert.Equal(
			t,
			[]string{"targets-value0", "targets-value1"},
			fls.Targets,
		)
		assert.Equal(t, "img-name-value", fls.ImgName)
		assert.Equal(t, "img-tag-value", fls.ImgTag)
		assert.False(t, fls.ImgLatest)
		assert.True(t, fls.DryRun)
		assert.True(t, fls.Rebuild)
		assert.True(t, fls.Export)
		assert.Equal(t, "cmd-value", fls.Cmd)
		assert.Empty(t, fls.Args)
		assert.Empty(t, buf.String())
	})

	t.Run("with latest", func(t *testing.T) {
		// --- Given ---
		buf := &bytes.Buffer{}
		args := []string{"--latest"}
		fp := NewFlagParser(":name", buf)
		fp.Add(FlagImgLatest)

		// --- When ---
		err := fp.Parse(args)

		// --- Then ---
		assert.NoError(t, err)

		fls := fp.fls
		assert.True(t, fls.ImgLatest)
	})

	t.Run("extra args", func(t *testing.T) {
		// --- Given ---
		buf := &bytes.Buffer{}
		args := []string{"extra", "--extra-arg", "extra-value"}
		fp := NewFlagParser(":name", buf)
		fp.Add(
			FlagTargets,
			FlagImgName,
			FlagImgTag,
			FlagImgLatest,
			FlagDryRun,
			FlagRebuild,
		)

		// --- When ---
		err := fp.Parse(args)

		// --- Then ---
		assert.NoError(t, err)

		fls := fp.fls
		assert.Equal(t, ":name", fls.Name)
		assert.Empty(t, fls.Targets)
		assert.Equal(t, "", fls.ImgName)
		assert.Equal(t, "", fls.ImgTag)
		assert.False(t, fls.ImgLatest)
		assert.False(t, fls.DryRun)
		assert.False(t, fls.Rebuild)
		assert.False(t, fls.Export)
		assert.Equal(
			t,
			[]string{"extra", "--extra-arg", "extra-value"},
			fls.Args,
		)
		assert.Empty(t, buf.String())
	})

	t.Run("extra args with separator", func(t *testing.T) {
		// --- Given ---
		buf := &bytes.Buffer{}
		args := []string{"--", "--extra-arg", "extra-value"}
		fp := NewFlagParser(":name", buf)
		fp.Add(
			FlagTargets,
			FlagImgName,
			FlagImgTag,
			FlagImgLatest,
			FlagDryRun,
			FlagRebuild,
		)

		// --- When ---
		err := fp.Parse(args)

		// --- Then ---
		assert.NoError(t, err)

		fls := fp.fls
		assert.Equal(t, ":name", fls.Name)
		assert.Empty(t, fls.Targets)
		assert.Equal(t, "", fls.ImgName)
		assert.Equal(t, "", fls.ImgTag)
		assert.False(t, fls.ImgLatest)
		assert.False(t, fls.DryRun)
		assert.False(t, fls.Rebuild)
		assert.False(t, fls.Export)
		assert.Equal(t, []string{"--extra-arg", "extra-value"}, fls.Args)
		assert.Empty(t, buf.String())
	})

	t.Run("error - invalid latest value", func(t *testing.T) {
		// --- Given ---
		buf := &bytes.Buffer{}
		args := []string{"--latest=invalid"}
		fp := NewFlagParser(":name", buf)
		fp.Add(FlagImgLatest)

		// --- When ---
		err := fp.Parse(args)

		// --- Then ---
		wMsg := "invalid boolean value \"invalid\" for -latest: parse error"
		assert.ErrorEqual(t, wMsg, err)
	})
}

func Test_NewFlags(t *testing.T) {
	// --- When ---
	fls := NewFlags("name")

	// --- Then ---
	assert.Equal(t, "name", fls.Name)
	assert.Nil(t, fls.Args)
	assert.Nil(t, fls.Targets)
	assert.Empty(t, fls.ImgName)
	assert.Empty(t, fls.ImgTag)
	assert.False(t, fls.ImgLatest)
	assert.False(t, fls.DryRun)
	assert.False(t, fls.Rebuild)
	assert.False(t, fls.Export)
	assert.Empty(t, fls.Cmd)
}
