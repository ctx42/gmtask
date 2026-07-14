// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr

import (
	"flag"
	"fmt"
	"io"

	"github.com/ctx42/xflag/pkg/xflag"
)

// FlagParser is responsible for parsing CLI arguments.
type FlagParser struct {
	fls   *Flags         // Parsed flags.
	fs    *xflag.FlagSet // Flag parser.
	binds []func()       // Copy parsed flag values into fls during Parse.
}

// NewFlagParser returns new instance of [FlagParser]. Initially flag parser
// has no flags - use [FlagParser.Add] method to add flags.
func NewFlagParser(name string, out io.Writer) *FlagParser {
	fp := &FlagParser{
		fs:  xflag.NewFlagSet(name, flag.ContinueOnError),
		fls: NewFlags(name),
	}
	fp.fs.SetOutput(out)
	fp.fs.Usage = func() {
		_, _ = fmt.Fprintf(out, "Usage of %s:\n", name)
		_, _ = fmt.Fprint(out, xflag.HelpOptions(fp.fs))
	}
	return fp
}

// Parse parses argument flags.
func (fp *FlagParser) Parse(args []string) error {
	if err := fp.fs.Parse(args); err != nil {
		return err
	}
	for _, bind := range fp.binds {
		bind()
	}
	fp.fls.Args = fp.fs.Args()
	return nil
}

// Add provides the way to add flags to [FlagParser].
func (fp *FlagParser) Add(flags ...func(*FlagParser)) *FlagParser {
	for _, flg := range flags {
		flg(fp)
	}
	return fp
}

// bindBool registers a boolean flag with long and short names and arranges for
// its parsed value to be copied into dst during [FlagParser.Parse].
func (fp *FlagParser) bindBool(long, short, usage string, dst *bool) {
	src := fp.fs.BoolSL(long, short, false, usage)
	fp.binds = append(fp.binds, func() { *dst = *src })
}

// bindStr registers a string flag with long and short names and arranges for
// its parsed value to be copied into dst during [FlagParser.Parse].
func (fp *FlagParser) bindStr(long, short, usage string, dst *string) {
	src := fp.fs.StringSL(long, short, "", usage)
	fp.binds = append(fp.binds, func() { *dst = *src })
}

// Flags represents values set from the CLI argument flags.
type Flags struct {
	Name      string   // Target name (mandatory).
	Args      []string // Arguments left after parsing target argument flags.
	Targets   []string // List of targets.
	ImgName   string   // Docker image name.
	ImgTag    string   // Docker image tag.
	ImgLatest bool     // Tag image with latest tag.
	DryRun    bool     // Do dry run for the target.
	Help      bool     // Show help.
	Rebuild   bool     // Force image rebuild.
	Export    bool     // Add "export" when listing environment variables.
	Cmd       string   // Command to run inside the container.
}

// NewFlags returns new instance of [Flags] with mandatory target name.
func NewFlags(name string) *Flags {
	return &Flags{Name: name}
}

// FlagHelp adds `help` flag to the [FlagParser].
func FlagHelp(fp *FlagParser) {
	fp.bindBool("help", "h", "show help", &fp.fls.Help)
}

// FlagTargets adds `targets` flag to the [FlagParser].
func FlagTargets(fp *FlagParser) {
	fn := func(s string) error { fp.fls.Targets = splitTargets(s); return nil }
	fp.fs.FuncSL("targets", "T", "comma separated docker targets", fn)
}

// FlagImgName adds Docker image `name` flag to the [FlagParser].
func FlagImgName(fp *FlagParser) {
	fp.bindStr("name", "n", "docker image name", &fp.fls.ImgName)
}

// FlagImgTag adds Docker image `tag` flag to the [FlagParser].
func FlagImgTag(fp *FlagParser) {
	fp.bindStr("tag", "t", "docker image tag", &fp.fls.ImgTag)
}

// FlagImgLatest adds `latest` flag to the [FlagParser].
func FlagImgLatest(fp *FlagParser) {
	fp.bindBool("latest", "l", "add latest image tag", &fp.fls.ImgLatest)
}

// FlagDryRun adds `dry-run` flag to the [FlagParser].
func FlagDryRun(fp *FlagParser) {
	fp.bindBool("dry-run", "d", "dry run", &fp.fls.DryRun)
}

// FlagRebuild adds `rebuild` flag to the [FlagParser].
func FlagRebuild(fp *FlagParser) {
	fp.bindBool("rebuild", "r", "rebuild image", &fp.fls.Rebuild)
}

// FlagExport adds `export` flag to the [FlagParser].
func FlagExport(fp *FlagParser) {
	fp.bindBool("export", "e", "export variables", &fp.fls.Export)
}

// FlagCmd adds `cmd` flag to the [FlagParser]. An empty value lets the target
// pick its own default command.
func FlagCmd(fp *FlagParser) {
	fp.bindStr("cmd", "c", "command to run inside the container", &fp.fls.Cmd)
}
