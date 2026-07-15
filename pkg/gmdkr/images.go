// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/ctx42/ring/pkg/ring"
)

// ImageInfo represents Docker image information.
type ImageInfo struct {
	ID         string    `json:"ID"`         // Docker image ID.
	Repository string    `json:"Repository"` // Docker repository + image name.
	Tag        string    `json:"Tag"`        // Docker image tag.
	CreatedAt  time.Time `json:"CreatedAt"`  // Image creation date.
}

func (img *ImageInfo) UnmarshalJSON(i []byte) error {
	type T1 ImageInfo
	t1 := struct {
		*T1
		CreatedAt string `json:"CreatedAt"`
	}{T1: (*T1)(img)}
	if err := json.Unmarshal(i, &t1); err != nil {
		return err
	}
	tim, err := time.Parse("2006-01-02 15:04:05 -0700 MST", t1.CreatedAt)
	if err != nil {
		return err
	}
	img.CreatedAt = tim.UTC()
	return nil
}

// ImageInfos represents collection of Docker image information.
type ImageInfos []*ImageInfo

// Find finds image with given ref (image-name:image-tag) in the collection.
// Returns nil when image has not been found in the collection.
//
//goland:noinspection GoMixedReceiverTypes
func (ims ImageInfos) Find(ref string) *ImageInfo {
	for _, img := range ims {
		if ref == img.Repository+":"+img.Tag {
			return img
		}
	}
	return nil
}

// RemoveDuplicates removes duplicate images based on image ID.
//
//goland:noinspection GoMixedReceiverTypes
func (ims *ImageInfos) RemoveDuplicates() {
	unique := map[string]struct{}{}
	uniqueFn := func(img *ImageInfo) bool {
		if _, ok := unique[img.ID]; ok {
			return true
		}
		unique[img.ID] = struct{}{}
		return false
	}
	*ims = slices.DeleteFunc(*ims, uniqueFn)
}

// ImgLs lists docker images. The [ring.Ring.Args] may have additional
// arguments to "docker image ls" base command.
func ImgLs(ctx context.Context, rng *ring.Ring) (ImageInfos, error) {
	// We override printers because this function is utility function and its
	// results should not be printed to standard output or standard error
	// when used in context of other commands.
	sout, eout := &bytes.Buffer{}, &bytes.Buffer{}

	args := []string{"image", "ls", "--format={{json .}}"}
	args = append(args, rng.Args()...)
	rng = rng.Clone()
	rng.SetStdout(sout)
	rng.SetStderr(eout)
	rng.SetArgs(args)

	soutS, _, err := runDockerCmd(ctx, rng)
	if err != nil {
		return nil, err
	}

	var ims []*ImageInfo
	dec := json.NewDecoder(strings.NewReader(soutS))
	for dec.More() {
		var img *ImageInfo
		if err = dec.Decode(&img); err != nil {
			return nil, err
		}
		ims = append(ims, img)
	}
	return ims, nil
}
