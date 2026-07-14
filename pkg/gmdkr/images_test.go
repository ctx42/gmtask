// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/dkrkit"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/netkit"
	"github.com/ctx42/testkit/pkg/randkit"

	"github.com/ctx42/gmtask/internal/gmtest"
)

func Test_ImageInfo_UnmarshalJSON(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		data := `{
            "ID": "id", 
            "Repository": "repo.com:5000/proj/image", 
            "Tag": "v1.2.3", 
            "CreatedAt": "2000-01-02 03:04:05 +0200 CEST"
        }`
		img := &ImageInfo{}

		// --- When ---
		err := json.Unmarshal([]byte(data), img)

		// --- Then ---
		assert.NoError(t, err)
		want := &ImageInfo{
			ID:         "id",
			Repository: "repo.com:5000/proj/image",
			Tag:        "v1.2.3",
			CreatedAt:  time.Date(2000, 1, 2, 1, 4, 5, 0, time.UTC),
		}
		assert.Equal(t, want, img)
		assert.Zone(t, time.UTC, img.CreatedAt.Location())
	})

	t.Run("error - invalid JSON format", func(t *testing.T) {
		// --- Given ---
		img := &ImageInfo{}

		// --- When ---
		err := json.Unmarshal([]byte(`{!!!}`), img)

		// --- Then ---
		assert.ErrorContain(t, "invalid character", err)
	})

	t.Run("error - invalid CreatedAt format", func(t *testing.T) {
		// --- Given ---
		data := `{
            "ID": "id", 
            "Repository": "repo.com:5000/proj/image", 
            "Tag": "v1.2.3", 
            "CreatedAt": "2000-01-02T03:04:05+02:00"
        }`
		img := &ImageInfo{}

		// --- When ---
		err := json.Unmarshal([]byte(data), img)

		// --- Then ---
		assert.ErrorContain(t, "cannot parse", err)
	})

	t.Run("missing embedded fields", func(t *testing.T) {
		// --- Given ---
		data := `{"CreatedAt": "2000-01-02 03:04:05 +0200 CEST"}`
		img := &ImageInfo{}

		// --- When ---
		err := json.Unmarshal([]byte(data), img)

		// --- Then ---
		assert.NoError(t, err)
		want := time.Date(2000, 1, 2, 1, 4, 5, 0, time.UTC)
		assert.Equal(t, want, img.CreatedAt)
	})
}

func Test_ImageInfos_Find(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		// --- Given ---
		ims := ImageInfos{
			{ID: "0", Repository: "repo0", Tag: "tag0"},
			{ID: "1", Repository: "repo1", Tag: "tag1"},
			{ID: "2", Repository: "repo2", Tag: "tag2"},
		}

		// --- When ---
		img := ims.Find("repo1:tag1")

		// --- Then ---
		assert.Equal(t, "1", img.ID)
		assert.Equal(t, "repo1", img.Repository)
		assert.Equal(t, "tag1", img.Tag)
	})

	t.Run("not found", func(t *testing.T) {
		// --- Given ---
		ims := ImageInfos{
			{ID: "0", Repository: "repo0", Tag: "tag0"},
			{ID: "1", Repository: "repo1", Tag: "tag1"},
			{ID: "2", Repository: "repo2", Tag: "tag2"},
		}

		// --- When ---
		img := ims.Find("repo1:tag2")

		// --- Then ---
		assert.Nil(t, img)
	})

	t.Run("nil", func(t *testing.T) {
		// --- When ---
		img := ImageInfos(nil).Find("repo1:tag2")

		// --- Then ---
		assert.Nil(t, img)
	})
}

func Test_ImageInfos_RemoveDuplicates(t *testing.T) {
	t.Run("removed", func(t *testing.T) {
		// --- Given ---
		have := ImageInfos{
			{ID: "0", Repository: "repo0", Tag: "tag0"},
			{ID: "1", Repository: "repo1", Tag: "tag1"},
			{ID: "0", Repository: "repo2", Tag: "tag2"},
		}

		// --- When ---
		have.RemoveDuplicates()

		// --- Then ---
		want := ImageInfos{
			{ID: "0", Repository: "repo0", Tag: "tag0"},
			{ID: "1", Repository: "repo1", Tag: "tag1"},
		}
		assert.Equal(t, want, have)
	})

	t.Run("not duplicates", func(t *testing.T) {
		// --- Given ---
		have := ImageInfos{
			{ID: "0", Repository: "repo0", Tag: "tag0"},
			{ID: "1", Repository: "repo1", Tag: "tag1"},
			{ID: "2", Repository: "repo2", Tag: "tag2"},
		}

		// --- When ---
		have.RemoveDuplicates()

		// --- Then ---
		want := ImageInfos{
			{ID: "0", Repository: "repo0", Tag: "tag0"},
			{ID: "1", Repository: "repo1", Tag: "tag1"},
			{ID: "2", Repository: "repo2", Tag: "tag2"},
		}
		assert.Equal(t, want, have)
	})

	t.Run("nil", func(t *testing.T) {
		// --- Given ---
		have := ImageInfos(nil)

		// --- When ---
		have.RemoveDuplicates()

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_ImgLs(t *testing.T) {
	tstImgLabel := randkit.Str()
	tstImgName := dkrkit.RandName()
	tstImgTag := dkrkit.RandTag()
	tstImgRef := tstImgName + ":" + tstImgTag

	exe := exekit.New(
		t,
		exekit.WithWd("testdata"),
		exekit.WithTimeout(10*time.Second),
	)
	exe.Exe("docker", "build", "-t", tstImgRef, "--label="+tstImgLabel, ".")

	t.Run("error - cannot connect to docker host", func(t *testing.T) {
		// --- Given ---
		port := must.Value(netkit.GetFreePort())
		host := fmt.Sprintf("tcp://127.0.0.1:%d", port)

		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring()
		rng.EnvSet("DOCKER_HOST", host)

		// --- When ---
		ims, err := ImgLs(ctx, rng)

		// --- Then ---
		assert.ErrorContain(t, "Cannot connect to the Docker daemon at", err)
		assert.Nil(t, ims)
	})

	t.Run("image found", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		rng := tst.Ring()

		// --- When ---
		ims, err := ImgLs(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)

		img := ims.Find(tstImgRef)
		assert.NotEmpty(t, img.ID)
		assert.Equal(t, tstImgName, img.Repository)
		assert.Equal(t, tstImgTag, img.Tag)
	})

	t.Run("with filter", func(t *testing.T) {
		// --- Given ---
		ctx := context.Background()
		tst := ringtest.New(t)

		prj := gmtest.NewProject(t)
		prj.Close()

		args := []string{"--filter=label=" + tstImgLabel}
		rng := tst.Ring()
		rng.SetArgs(args)

		// --- When ---
		ims, err := ImgLs(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, ims)
	})
}
