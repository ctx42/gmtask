// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/gomake/pkg/gomake"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/jsonkit"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_structNode_UnmarshalJSON(t *testing.T) {
	t.Run("file node attributes", func(t *testing.T) {
		// --- Given ---
		data := []byte(`{` +
			`"type":"file",` +
			`"content":"body",` +
			`"mode":"0755",` +
			`"feature":"git"` +
			`}`)

		// --- When ---
		var have structNode
		err := json.Unmarshal(data, &have)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, typeFile, have.Type)
		assert.Equal(t, "body", have.Content)
		assert.Equal(t, "0755", have.Mode)
		assert.Equal(t, "git", have.Feature)
		assert.Len(t, 0, have.Children)
	})

	t.Run("reserved keys never become children", func(t *testing.T) {
		// --- Given ---
		data := []byte(`{"type":"dir","cmd":{"type":"dir"}}`)

		// --- When ---
		var have structNode
		err := json.Unmarshal(data, &have)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, typeDir, have.Type)
		assert.Len(t, 1, have.Children)
		assert.NotNil(t, have.Children["cmd"])
	})

	t.Run("nested tree decodes fully", func(t *testing.T) {
		// --- Given ---
		data := []byte(`{` +
			`"type":"dir",` +
			`"dev":{"type":"dir","idea":{` +
			`"type":"dir","run.xml":{"type":"file","content":"x"}}}` +
			`}`)

		// --- When ---
		var have structNode
		err := json.Unmarshal(data, &have)

		// --- Then ---
		assert.NoError(t, err)
		idea := have.Children["dev"].Children["idea"]
		run := idea.Children["run.xml"]
		assert.Equal(t, typeFile, run.Type)
		assert.Equal(t, "x", run.Content)
	})

	t.Run("error - attribute of wrong JSON kind", func(t *testing.T) {
		// --- Given ---
		data := []byte(`{"content":123}`)

		// --- When ---
		var have structNode
		err := json.Unmarshal(data, &have)

		// --- Then ---
		assert.ErrorContain(t, `structure node "content"`, err)
	})

	t.Run("error - not an object", func(t *testing.T) {
		// --- Given ---
		data := []byte(`"file"`)

		// --- When ---
		var have structNode
		err := json.Unmarshal(data, &have)

		// --- Then ---
		assert.ErrorContain(t, "cannot unmarshal", err)
	})
}

func Test_loadStructure(t *testing.T) {
	t.Run("block present", func(t *testing.T) {
		// --- Given ---
		block := map[string]any{
			"structure": map[string]any{
				"cmd": map[string]any{"type": "dir"},
				"README.md": map[string]any{
					"type":    "file",
					"content": "hi",
				},
			},
		}
		rng := ringtest.New(t).Ring()
		rng.MetaSet(gomake.ConfigMetaKey, jsonkit.To(t, block))

		// --- When ---
		have, err := loadStructure(rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 2, have)
		assert.Equal(t, typeDir, have["cmd"].Type)
		assert.Equal(t, "hi", have["README.md"].Content)
	})

	t.Run("error - no structure key", func(t *testing.T) {
		// --- Given ---
		block := map[string]any{"other": "x"}
		rng := ringtest.New(t).Ring()
		rng.MetaSet(gomake.ConfigMetaKey, jsonkit.To(t, block))

		// --- When ---
		have, err := loadStructure(rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoStructure, err)
		assert.Nil(t, have)
	})

	t.Run("error - empty structure block", func(t *testing.T) {
		// --- Given ---
		block := map[string]any{"structure": map[string]any{}}
		rng := ringtest.New(t).Ring()
		rng.MetaSet(gomake.ConfigMetaKey, jsonkit.To(t, block))

		// --- When ---
		have, err := loadStructure(rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoStructure, err)
		assert.Nil(t, have)
	})

	t.Run("error - no config delivered", func(t *testing.T) {
		// --- Given ---
		rng := ringtest.New(t).Ring()

		// --- When ---
		have, err := loadStructure(rng)

		// --- Then ---
		assert.ErrorIs(t, ErrNoStructure, err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid block rejected", func(t *testing.T) {
		// --- Given ---
		block := map[string]any{
			"structure": map[string]any{
				"cmd": map[string]any{"type": "directory"},
			},
		}
		rng := ringtest.New(t).Ring()
		rng.MetaSet(gomake.ConfigMetaKey, jsonkit.To(t, block))

		// --- When ---
		have, err := loadStructure(rng)

		// --- Then ---
		assert.ErrorContain(t, `cmd: invalid type "directory"`, err)
		assert.Nil(t, have)
	})

	t.Run("error - structure block not a map", func(t *testing.T) {
		// --- Given ---
		block := map[string]any{"structure": "nope"}
		rng := ringtest.New(t).Ring()
		rng.MetaSet(gomake.ConfigMetaKey, jsonkit.To(t, block))

		// --- When ---
		have, err := loadStructure(rng)

		// --- Then ---
		assert.ErrorContain(t, "config type mismatch", err)
		assert.Nil(t, have)
	})
}

func Test_structure_validate(t *testing.T) {
	t.Run("clean tree passes", func(t *testing.T) {
		// --- Given ---
		str := structure{
			"cmd":        {Type: typeDir},
			".gitignore": {Type: typeFile, Content: "tmp", Feature: featureGit},
			"dev": {Type: typeDir, Children: map[string]*structNode{
				"run.xml": {Type: typeFile, Content: "x", Mode: "0644"},
			}},
		}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - invalid feature", func(t *testing.T) {
		// --- Given ---
		str := structure{"cmd": {Type: typeDir, Feature: "gti"}}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.ErrorContain(t, `cmd: invalid feature "gti"`, err)
	})

	t.Run("error - invalid type", func(t *testing.T) {
		// --- Given ---
		str := structure{"cmd": {Type: "directory"}}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.ErrorContain(t, `cmd: invalid type "directory"`, err)
	})

	t.Run("error - content on directory", func(t *testing.T) {
		// --- Given ---
		str := structure{"cmd": {Type: typeDir, Content: "x"}}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.ErrorContain(t, "cmd: directory has content", err)
	})

	t.Run("error - file has children", func(t *testing.T) {
		// --- Given ---
		str := structure{"f": {Type: typeFile, Children: map[string]*structNode{
			"c": {Type: typeDir},
		}}}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.ErrorContain(t, "f: file has children", err)
	})

	t.Run("error - bad mode", func(t *testing.T) {
		// --- Given ---
		str := structure{"f": {Type: typeFile, Mode: "0999"}}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.ErrorContain(t, `f: invalid mode "0999"`, err)
	})

	t.Run("error - nested node named by path", func(t *testing.T) {
		// --- Given ---
		str := structure{"dev": {Type: typeDir, Children: map[string]*structNode{
			"idea": {Type: "bad"},
		}}}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.ErrorContain(t, `dev/idea: invalid type "bad"`, err)
	})

	t.Run("error - nil node", func(t *testing.T) {
		// --- Given ---
		str := structure{"cmd": nil}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.ErrorContain(t, "cmd: empty node", err)
	})

	t.Run("error - nested nil node", func(t *testing.T) {
		// --- Given ---
		str := structure{"dev": {Type: typeDir, Children: map[string]*structNode{
			"idea": nil,
		}}}

		// --- When ---
		err := str.validate()

		// --- Then ---
		assert.ErrorContain(t, "dev/idea: empty node", err)
	})
}

func Test_tmplVars_render(t *testing.T) {
	t.Run("substitutes project name", func(t *testing.T) {
		// --- Given ---
		vrs := tmplVars{ProjectName: "myproj"}
		content := `<module name="{{.ProjectName}}" />`

		// --- When ---
		have, err := vrs.render(content)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, `<module name="myproj" />`, have)
	})

	t.Run("resolves all five variables", func(t *testing.T) {
		// --- Given ---
		vrs := tmplVars{
			ProjectName: "proj",
			Module:      "example.com/x/proj",
			Package:     "proj",
			Origin:      "git@host:x/proj.git",
			Repo:        "registry.example.com",
		}
		content := "" +
			"{{.ProjectName}} {{.Module}} {{.Package}} " +
			"{{.Origin}} {{.Repo}}"

		// --- When ---
		have, err := vrs.render(content)

		// --- Then ---
		want := "proj example.com/x/proj proj " +
			"git@host:x/proj.git registry.example.com"
		assert.NoError(t, err)
		assert.Equal(t, want, have)
	})

	t.Run("error - malformed template", func(t *testing.T) {
		// --- Given ---
		vrs := tmplVars{}

		// --- When ---
		have, err := vrs.render("{{.ProjectName")

		// --- Then ---
		assert.Error(t, err)
		assert.Equal(t, "", have)
	})

	t.Run("error - unknown variable writes nothing", func(t *testing.T) {
		// --- Given ---
		vrs := tmplVars{ProjectName: "proj"}

		// --- When ---
		have, err := vrs.render("head {{.Nope}} tail")

		// --- Then ---
		assert.ErrorContain(t, "Nope", err)
		assert.Equal(t, "", have)
	})
}

func Test_structure_materialize(t *testing.T) {
	t.Run("creates the declared tree", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		var log bytes.Buffer

		str := structure{
			"cmd": {Type: typeDir},
			"dev": {Type: typeDir, Children: map[string]*structNode{
				"idea": {Type: typeDir, Children: map[string]*structNode{
					"run.xml": {Type: typeFile, Content: "name={{.ProjectName}}"},
				}},
			}},
			"README.md": {Type: typeFile, Content: ""},
		}
		vrs := tmplVars{ProjectName: "proj"}

		// --- When ---
		err := str.materialize(&log, root, vrs)

		// --- Then ---
		assert.NoError(t, err)
		assert.DirExist(t, filepath.Join(root, "cmd"))
		assert.DirExist(t, filepath.Join(root, "dev", "idea"))
		assert.FileExist(t, filepath.Join(root, "README.md"))
		assert.Equal(t, "name=proj",
			oskit.ReadFileStr(t, root, "dev", "idea", "run.xml"))

		want := "" +
			"file created: README.md\n" +
			"dir created: cmd\n" +
			"dir created: dev\n" +
			"dir created: dev/idea\n" +
			"file created: dev/idea/run.xml\n"
		assert.Equal(t, want, log.String())
	})

	t.Run("applies explicit file mode", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		str := structure{
			"run.sh": {Type: typeFile, Content: "#!/bin/sh\n", Mode: "0755"},
		}

		// --- When ---
		err := str.materialize(io.Discard, root, tmplVars{})

		// --- Then ---
		assert.NoError(t, err)
		info := oskit.Stat(t, root, "run.sh")
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	})

	t.Run("applies explicit directory mode past the umask", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		str := structure{"secret": {Type: typeDir, Mode: "0770"}}

		// --- When ---
		err := str.materialize(io.Discard, root, tmplVars{})

		// --- Then ---
		assert.NoError(t, err)
		info := oskit.Stat(t, root, "secret")
		assert.Equal(t, os.FileMode(0o770), info.Mode().Perm())
	})

	t.Run("existing file is not overwritten and logs nothing", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.Write(t, "original", root, "README.md")
		var log bytes.Buffer

		str := structure{"README.md": {Type: typeFile, Content: "new"}}

		// --- When ---
		err := str.materialize(&log, root, tmplVars{})

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "original", oskit.ReadFileStr(t, root, "README.md"))
		assert.Equal(t, "", log.String())
	})

	t.Run("existing directory logs nothing", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		oskit.MkdirAll(t, root, "cmd")
		var log bytes.Buffer

		str := structure{"cmd": {Type: typeDir}}

		// --- When ---
		err := str.materialize(&log, root, tmplVars{})

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "", log.String())
	})

	t.Run("re-run is idempotent", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		first := structure{
			"cmd":       {Type: typeDir},
			"README.md": {Type: typeFile, Content: "v1"},
		}
		must.Nil(first.materialize(io.Discard, root, tmplVars{}))

		var log bytes.Buffer
		second := structure{
			"cmd":       {Type: typeDir},
			"README.md": {Type: typeFile, Content: "v2"},
		}

		// --- When ---
		err := second.materialize(&log, root, tmplVars{})

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1", oskit.ReadFileStr(t, root, "README.md"))
		assert.Equal(t, "", log.String())
	})

	t.Run("error - render failure leaves no file", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		str := structure{"bad": {Type: typeFile, Content: "{{.Nope}}"}}

		// --- When ---
		err := str.materialize(io.Discard, root, tmplVars{})

		// --- Then ---
		assert.ErrorContain(t, "Nope", err)
		assert.False(t, oskit.PathExists(t, root, "bad"))
	})

	t.Run("skips nodes whose feature is not enabled", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		str := structure{
			"README.md":  {Type: typeFile, Content: "b", Feature: featureBase},
			".gitignore": {Type: typeFile, Content: "g", Feature: featureGit},
			"go.sum":     {Type: typeFile, Content: "o", Feature: featureGolang},
		}

		// --- When ---
		err := str.materialize(io.Discard, root, tmplVars{})

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, filepath.Join(root, "README.md"))
		assert.False(t, oskit.PathExists(t, root, ".gitignore"))
		assert.False(t, oskit.PathExists(t, root, "go.sum"))
	})

	t.Run("creates nodes for enabled features", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		str := structure{
			"README.md":  {Type: typeFile, Content: "b", Feature: featureBase},
			".gitignore": {Type: typeFile, Content: "g", Feature: featureGit},
			"go.sum":     {Type: typeFile, Content: "o", Feature: featureGolang},
		}

		// --- When ---
		err := str.materialize(
			io.Discard,
			root,
			tmplVars{},
			featureGit,
			featureGolang,
		)

		// --- Then ---
		assert.NoError(t, err)
		assert.FileExist(t, filepath.Join(root, "README.md"))
		assert.FileExist(t, filepath.Join(root, ".gitignore"))
		assert.FileExist(t, filepath.Join(root, "go.sum"))
	})
}

func Test_structNode_create(t *testing.T) {
	t.Run("file lands when its parent is not a declared node", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		nod := &structNode{Type: typeFile, Content: "x"}
		rel := filepath.Join("sub", "deep", "f.txt")

		// --- When ---
		err := nod.create(io.Discard, root, rel, tmplVars{}, featureSet())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "x", oskit.ReadFileStr(t, root, "sub", "deep", "f.txt"))
	})

	t.Run("directory with children is created recursively", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		nod := &structNode{Type: typeDir, Children: map[string]*structNode{
			"f.txt": {Type: typeFile, Content: "x"},
		}}

		// --- When ---
		err := nod.create(io.Discard, root, "sub", tmplVars{}, featureSet())

		// --- Then ---
		assert.NoError(t, err)
		assert.DirExist(t, filepath.Join(root, "sub"))
		assert.Equal(t, "x", oskit.ReadFileStr(t, root, "sub", "f.txt"))
	})

	t.Run("error - directory with invalid mode", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		nod := &structNode{Type: typeDir, Mode: "0999"}

		// --- When ---
		err := nod.create(io.Discard, root, "bad", tmplVars{}, featureSet())

		// --- Then ---
		assert.ErrorContain(t, `invalid mode "0999"`, err)
	})

	t.Run("error - child fails to render", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		nod := &structNode{Type: typeDir, Children: map[string]*structNode{
			"f.txt": {Type: typeFile, Content: "{{.Nope}}"},
		}}

		// --- When ---
		err := nod.create(io.Discard, root, "sub", tmplVars{}, featureSet())

		// --- Then ---
		assert.ErrorContain(t, "Nope", err)
	})

	t.Run("error - file with invalid mode", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		nod := &structNode{Type: typeFile, Content: "x", Mode: "0999"}

		// --- When ---
		err := nod.create(io.Discard, root, "f.txt", tmplVars{}, featureSet())

		// --- Then ---
		assert.ErrorContain(t, `invalid mode "0999"`, err)
	})

	t.Run("skips node when feature not enabled", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		nod := &structNode{
			Type:    typeFile,
			Content: "x",
			Feature: featureGit,
		}

		// --- When ---
		err := nod.create(io.Discard, root, "skip.txt", tmplVars{}, featureSet())

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, oskit.PathExists(t, root, "skip.txt"))
	})
}

func Test_fixture_gomake_json(t *testing.T) {
	// --- Given ---
	// gomake parses the user's gomake.yaml and delivers the target's block to
	// the target as JSON; this fixture is that JSON, mirroring doc/gomake.yaml.
	block := oskit.ReadFile(t, "testdata", "gomake.json")

	rng := ringtest.New(t).Ring()
	rng.MetaSet(gomake.ConfigMetaKey, block)

	root := t.TempDir()
	vrs := tmplVars{ProjectName: "acme"}

	// --- When ---
	str, err := loadStructure(rng)

	// --- Then ---
	assert.NoError(t, err)
	assert.NoError(t, str.materialize(
		io.Discard,
		root,
		vrs,
		featureGit,
		featureGolang,
	))

	assert.DirExist(t, filepath.Join(root, "dev", "idea"))
	assert.DirExist(t, filepath.Join(root, "build"))
	assert.False(t, oskit.PathExists(t, filepath.Join(root, "build", "idea")))

	xml := oskit.ReadFileStr(t, root, "dev", "idea", "go-test-all.run.xml")
	assert.Contain(t, `name="acme"`, xml)

	assert.Contain(t, "root = true", oskit.ReadFileStr(t, root, ".editorconfig"))
	assert.Contain(t, ".idea/", oskit.ReadFileStr(t, root, ".gitignore"))
	assert.FileExist(t, filepath.Join(root, "configs", "project.conf"))
}

func Test_structNode_feature(t *testing.T) {
	t.Run("defaults to base when unset", func(t *testing.T) {
		// --- Given ---
		nod := &structNode{Type: typeDir}

		// --- When ---
		have := nod.feature()

		// --- Then ---
		assert.Equal(t, featureBase, have)
	})

	t.Run("returns the set feature", func(t *testing.T) {
		// --- Given ---
		nod := &structNode{Type: typeFile, Feature: featureGolang}

		// --- When ---
		have := nod.feature()

		// --- Then ---
		assert.Equal(t, featureGolang, have)
	})
}

func Test_structNode_perm(t *testing.T) {
	t.Run("default when no mode", func(t *testing.T) {
		// --- Given ---
		nod := &structNode{Type: typeDir}

		// --- When ---
		have, err := nod.perm(dirMode)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, dirMode, have)
	})

	t.Run("explicit octal mode", func(t *testing.T) {
		// --- Given ---
		nod := &structNode{Type: typeFile, Mode: "0644"}

		// --- When ---
		have, err := nod.perm(fileMode)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), have)
	})

	t.Run("error - unparsable mode", func(t *testing.T) {
		// --- Given ---
		nod := &structNode{Type: typeFile, Mode: "0999"}

		// --- When ---
		_, err := nod.perm(fileMode)

		// --- Then ---
		assert.ErrorContain(t, `invalid mode "0999"`, err)
	})
}
