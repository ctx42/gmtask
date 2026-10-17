// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmprj"
)

// ErrNoRunID is returned by [Run.Clean] when the environment belongs to no
// CI/CD run.
var ErrNoRunID = errors.New("no CI/CD run ID: set " + xdef.EnvCCRunID)

// Run groups the ":docker:run:*" gomake targets, which act on the Docker
// resources a CI/CD run created.
type Run Docker

// Clean removes the Docker resources the CI/CD run created. Those are the
// containers, networks, volumes, and images labeled [xdef.LabCCRunID] with
// the run's ID, then the images the --ids-file lists, which the run built and
// did not push. The run ID comes from [xdef.CCRunID]; without one it returns
// [ErrNoRunID], so it can never remove unlabeled resources. It prints one line
// for each resource it removes; a dry run only prints them.
//
// A resource already gone counts as removed. Any other failure is returned
// once all the remaining removals ran.
func (Run) Clean(ctx context.Context, rng *ring.Ring) error {
	fp := NewFlagParser(":docker:run:clean", rng.Stderr())
	fp.Add(FlagHelp, FlagDryRun, FlagIDsFile)
	if err := fp.Parse(rng.Args()); err != nil {
		return err
	}
	if fp.fls.Help {
		fp.fs.Usage()
		return nil
	}
	if fp.fs.NArg() > 0 {
		return fmt.Errorf("%w: %v", gmprj.ErrTooManyArgs, fp.fs.Args())
	}
	runID := xdef.CCRunID(rng.EnvAll())
	if runID == "" {
		return ErrNoRunID
	}
	return cleanRun(ctx, rng.Clone().SetArgs(nil), runID, fp.fls)
}

// cleanRun removes the Docker resources labeled with runID, kind by kind in
// the order they hold each other, then the images listed in fls.IDsFile, and
// the file itself.
func cleanRun(
	ctx context.Context,
	rng *ring.Ring,
	runID string,
	fls *Flags,
) error {

	filter := []string{"--filter", "label=" + xdef.LabCCRunID + "=" + runID}
	rmImage := []string{"image", "rm", "--force"}
	// Untagged images, such as the ones a test builds without a name, are
	// listed only with --all.
	kinds := []struct {
		list []string // Lists the full IDs of the kind.
		rm   []string // Removes one resource of the kind.
	}{
		{
			[]string{"container", "ls", "--all", "--quiet", "--no-trunc"},
			[]string{"container", "rm", "--force"},
		},
		{
			[]string{"network", "ls", "--quiet", "--no-trunc"},
			[]string{"network", "rm"},
		},
		{[]string{"volume", "ls", "--quiet"}, []string{"volume", "rm"}},
		{[]string{"image", "ls", "--all", "--quiet", "--no-trunc"}, rmImage},
	}

	var errs []error
	for _, kind := range kinds {
		out, err := dockerQuiet(ctx, rng, slices.Concat(kind.list, filter)...)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, id := range uniqueFields(out) {
			err = removeResource(ctx, rng, kind.rm, id, fls.DryRun)
			errs = append(errs, err)
		}
	}

	if fls.IDsFile == "" {
		return errors.Join(errs...)
	}
	ids, err := readImageIDs(fls.IDsFile)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, id := range ids {
		errs = append(errs, removeResource(ctx, rng, rmImage, id, fls.DryRun))
	}
	if !fls.DryRun {
		err = os.Remove(fls.IDsFile)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove ids file: %w", err))
		}
	}
	return errors.Join(errs...)
}

// removeResource prints and runs the docker command rm with id appended. A
// dry run only prints it. A resource already gone counts as removed.
func removeResource(
	ctx context.Context,
	rng *ring.Ring,
	rm []string,
	id string,
	dryRun bool,
) error {

	args := append(slices.Clone(rm), id)
	_, _ = fmt.Fprintf(
		rng.Stderr(),
		"#gomake INFO# docker %s\n",
		strings.Join(args, " "),
	)
	if dryRun {
		return nil
	}
	_, err := dockerQuiet(ctx, rng, args...)
	if err != nil && isGone(err) {
		return nil
	}
	return err
}

// isGone reports whether err is docker saying the resource does not exist.
// It matches the docker CLI's English messages, which differ by kind.
func isGone(err error) bool {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such") {
		return true
	}
	return strings.Contains(msg, "not found")
}

// dockerQuiet runs docker with args and returns its trimmed standard output,
// printing nothing.
func dockerQuiet(
	ctx context.Context,
	rng *ring.Ring,
	args ...string,
) (string, error) {

	rngDC := rng.Clone()
	rngDC.SetStdout(&bytes.Buffer{})
	rngDC.SetStderr(&bytes.Buffer{})
	rngDC.SetArgs(args)
	out, _, err := runDockerCmd(ctx, rngDC)
	return out, err
}

// recordImageID appends the "<image ID> <reference>" line of the image ref to
// the ids file pth, creating it when missing.
func recordImageID(
	ctx context.Context,
	rng *ring.Ring,
	pth string,
	ref string,
) error {

	args := []string{"image", "inspect", "--format={{.Id}}", ref}
	id, err := dockerQuiet(ctx, rng, args...)
	if err != nil {
		return fmt.Errorf("record image %s: %w", ref, err)
	}
	flag := os.O_CREATE | os.O_APPEND | os.O_WRONLY
	fil, err := os.OpenFile(pth, flag, 0o600)
	if err != nil {
		return fmt.Errorf("record image %s: %w", ref, err)
	}
	_, err = fmt.Fprintf(fil, "%s %s\n", id, ref)
	return errors.Join(err, fil.Close())
}

// forgetImageRef removes the lines of the image reference ref from the ids
// file pth: the run pushed that image, so it keeps it. A missing file has
// nothing to forget.
func forgetImageRef(pth, ref string) error {
	data, err := os.ReadFile(pth)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("forget image %s: %w", ref, err)
	}
	var keep []string
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		_, lineRef, _ := strings.Cut(line, " ")
		if line == "" || lineRef == ref {
			continue
		}
		keep = append(keep, line+"\n")
	}
	err = os.WriteFile(pth, []byte(strings.Join(keep, "")), 0o600)
	if err != nil {
		return fmt.Errorf("forget image %s: %w", ref, err)
	}
	return nil
}

// readImageIDs returns the image IDs the ids file pth lists, each once, in
// the order of the file. A missing file lists none.
func readImageIDs(pth string) ([]string, error) {
	data, err := os.ReadFile(pth)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ids file: %w", err)
	}
	var ids []string
	for line := range strings.Lines(string(data)) {
		if id, _, _ := strings.Cut(strings.TrimSpace(line), " "); id != "" {
			ids = append(ids, id)
		}
	}
	return uniqueFields(strings.Join(ids, " ")), nil
}

// uniqueFields returns the white-space separated fields of str, each once, in
// the order they first appear.
func uniqueFields(str string) []string {
	var fields []string
	for _, field := range strings.Fields(str) {
		if !slices.Contains(fields, field) {
			fields = append(fields, field)
		}
	}
	return fields
}
