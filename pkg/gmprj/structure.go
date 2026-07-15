// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"text/template"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring"
)

// Default permissions for structure nodes that set no explicit mode.
const (
	// dirMode is the default directory permission.
	dirMode os.FileMode = 0o777

	// fileMode is the default file permission.
	fileMode os.FileMode = 0o600
)

// Reserved attribute keys within a structure node. Every other key in a node
// names a child node — a nested file or directory.
const (
	// keyType selects a node's kind: typeFile or typeDir.
	keyType = "type"

	// keyContent holds a file node's body.
	keyContent = "content"

	// keyMode holds a node's octal permission string, e.g. "0755".
	keyMode = "mode"

	// keyFeature gates a node behind a project feature.
	keyFeature = "feature"
)

// Node kinds carried by a structure node's "type" key.
const (
	// typeFile marks a node that scaffolds a file.
	typeFile = "file"

	// typeDir marks a node that scaffolds a directory.
	typeDir = "dir"
)

// Project features gating a structure node. An empty node feature defaults to
// featureBase.
const (
	// featureBase marks a node always created, regardless of enabled features.
	featureBase = "base"

	// featureGit marks a node created only when the git feature is enabled.
	featureGit = "git"

	// featureGolang marks a node created only when the Go feature is enabled.
	featureGolang = "golang"
)

// structure is a project scaffold tree decoded from a gomake.yaml "structure"
// block: its top-level files and directories keyed by name.
type structure map[string]*structNode

// structNode is one file or directory in a [structure]. A directory nests its
// children under keys other than the reserved attribute keys (type, content,
// mode, feature), which carry the node's own settings.
type structNode struct {
	// Type is the node kind: typeFile or typeDir.
	Type string

	// Content is a file node's body, subject to template expansion.
	Content string

	// Mode is the node's octal permission string, e.g. "0755"; empty selects
	// the default permission.
	Mode string

	// Feature is the project feature gating the node; empty means the node is
	// part of the always-created base.
	Feature string

	// Children are the node's nested files and directories keyed by name.
	Children map[string]*structNode
}

var _ json.Unmarshaler = (*structNode)(nil)

func (nod *structNode) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key, val := range raw {
		var err error
		switch key {
		case keyType:
			err = json.Unmarshal(val, &nod.Type)

		case keyContent:
			err = json.Unmarshal(val, &nod.Content)

		case keyMode:
			err = json.Unmarshal(val, &nod.Mode)

		case keyFeature:
			err = json.Unmarshal(val, &nod.Feature)

		default:
			child := &structNode{}
			if err = json.Unmarshal(val, child); err == nil {
				if nod.Children == nil {
					nod.Children = map[string]*structNode{}
				}
				nod.Children[key] = child
			}
		}
		if err != nil {
			return fmt.Errorf("structure node %q: %w", key, err)
		}
	}
	return nil
}

// loadStructure reads the project scaffold tree from the running target's
// gomake.yaml "structure" block, delivered through the ring meta store. It
// returns [ErrNoStructure] when the block is absent or empty.
func loadStructure(rng *ring.Ring) (structure, error) {
	cfg, err := gomake.TargetConfig(rng)
	if err != nil {
		return nil, err
	}
	str, err := gomake.GetCfg[structure](cfg, "structure")
	if err != nil {
		if errors.Is(err, gomake.ErrMiss) {
			return nil, ErrNoStructure
		}
		return nil, err
	}
	if len(str) == 0 {
		return nil, ErrNoStructure
	}
	if err = str.validate(); err != nil {
		return nil, err
	}
	return str, nil
}

// validate checks every node in the structure and returns the first violation
// found, naming the offending node by its path. See [structNode.validate] for
// the rules.
func (str structure) validate() error {
	for name, nod := range str {
		if nod == nil {
			return fmt.Errorf("%s: empty node", name)
		}
		if err := nod.validate(name); err != nil {
			return err
		}
	}
	return nil
}

// validate checks the node and its descendants, returning the first violation
// found named by its path. A valid node has type file or dir; a file carries no
// children; a directory carries no content; a non-empty mode is an octal
// permission string.
func (nod *structNode) validate(path string) error {
	switch nod.Type {
	case typeFile:
		if len(nod.Children) > 0 {
			return fmt.Errorf("%s: file has children", path)
		}

	case typeDir:
		if nod.Content != "" {
			return fmt.Errorf("%s: directory has content", path)
		}

	default:
		return fmt.Errorf("%s: invalid type %q", path, nod.Type)
	}

	if nod.Mode != "" {
		if _, err := strconv.ParseUint(nod.Mode, 8, 32); err != nil {
			return fmt.Errorf("%s: invalid mode %q", path, nod.Mode)
		}
	}

	switch nod.feature() {
	case featureBase, featureGit, featureGolang:

	default:
		return fmt.Errorf("%s: invalid feature %q", path, nod.Feature)
	}

	for name, child := range nod.Children {
		sub := path + "/" + name
		if child == nil {
			return fmt.Errorf("%s: empty node", sub)
		}
		if err := child.validate(sub); err != nil {
			return err
		}
	}
	return nil
}

// tmplVars are the values exposed to a file node's content template.
type tmplVars struct {
	// ProjectName is the project's name.
	ProjectName string

	// Module is the Go module path.
	Module string

	// Package is the project's root Go package name.
	Package string

	// Origin is the git remote repository.
	Origin string

	// Repo is the Docker private repository.
	Repo string
}

// render expands the content template against the variables. A template that
// fails to parse or execute yields an error and no output, so a half-rendered
// file is never written.
func (vrs tmplVars) render(content string) (string, error) {
	tpl, err := template.New("content").Parse(content)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err = tpl.Execute(&buf, vrs); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// materialize creates the structure tree under the root directory, rendering
// each file's content against the template variables. Nodes are created in
// deterministic name order, parents before children. Existing files are left
// untouched, never overwritten; directories go through [os.MkdirAll]. For each
// node it actually creates, materialize writes a "dir created: <path>" or
// "file created: <path>" line, relative to root, to w.
func (str structure) materialize(w io.Writer, root string, vrs tmplVars) error {
	for _, name := range sortedNames(str) {
		if err := str[name].create(w, root, name, vrs); err != nil {
			return err
		}
	}
	return nil
}

// create writes the node at rel below root and, for a directory, recurses into
// its children. See [structure.materialize] for the creation and logging rules.
func (nod *structNode) create(
	w io.Writer,
	root string,
	rel string,
	vrs tmplVars,
) error {

	pth := filepath.Join(root, rel)
	if nod.Type == typeFile {
		return nod.createFile(w, pth, rel, vrs)
	}

	perm, err := nod.perm(dirMode)
	if err != nil {
		return err
	}
	_, err = os.Stat(pth)
	exists := err == nil
	if err = os.MkdirAll(pth, perm); err != nil {
		return err
	}
	if !exists {
		if nod.Mode != "" {
			// Force the exact mode past the umask, as createFile does.
			if err = os.Chmod(pth, perm); err != nil {
				return err
			}
		}
		_, _ = fmt.Fprintf(w, "dir created: %s\n", rel)
	}

	for _, name := range sortedNames(nod.Children) {
		sub := filepath.Join(rel, name)
		if err = nod.Children[name].create(w, root, sub, vrs); err != nil {
			return err
		}
	}
	return nil
}

// createFile renders and writes the file node at pth. An existing file is left
// untouched and logs nothing. The file's parent directory is created first so a
// file lands even when its parent is not a declared node.
func (nod *structNode) createFile(
	w io.Writer,
	pth string,
	rel string,
	vrs tmplVars,
) error {

	if gomake.FileExists(pth) {
		return nil
	}
	content, err := vrs.render(nod.Content)
	if err != nil {
		return err
	}
	perm, err := nod.perm(fileMode)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(pth), dirMode); err != nil {
		return err
	}
	if err = os.WriteFile(pth, []byte(content), perm); err != nil {
		return err
	}
	if nod.Mode != "" {
		// Force the exact mode: os.WriteFile applies the umask, which an
		// explicit permission such as "0755" must ignore.
		if err = os.Chmod(pth, perm); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(w, "file created: %s\n", rel)
	return nil
}

// feature returns the node's gating feature, defaulting to featureBase when the
// node sets none.
func (nod *structNode) feature() string {
	if nod.Feature == "" {
		return featureBase
	}
	return nod.Feature
}

// perm resolves the node's permission, falling back to def when it sets no
// explicit mode.
func (nod *structNode) perm(def os.FileMode) (os.FileMode, error) {
	if nod.Mode == "" {
		return def, nil
	}
	mode, err := strconv.ParseUint(nod.Mode, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode %q: %w", nod.Mode, err)
	}
	return os.FileMode(mode), nil
}

// sortedNames returns the node names in the map sorted for deterministic
// creation and logging order.
func sortedNames(nodes map[string]*structNode) []string {
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
