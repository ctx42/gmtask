// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmprj_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gmtask/pkg/gmprj"
)

func ExampleGetInfo() {
	// A project needs only its configuration file to be inspected.
	tmp, err := os.MkdirTemp("", "gmprj-example-")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	root := filepath.Join(tmp, "demo")
	cfg := filepath.Join(root, gmprj.CfgPath)
	if err = os.MkdirAll(filepath.Dir(cfg), 0o750); err != nil {
		fmt.Println(err)
		return
	}
	if err = os.WriteFile(cfg, []byte("OWNER=acme\n"), 0o600); err != nil {
		fmt.Println(err)
		return
	}

	inf, err := gmprj.GetInfo(context.Background(), nil, root)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(inf.Get(xdef.EnvPrjName))
	fmt.Println(inf.Get(xdef.EnvScmState))
	fmt.Println(inf.Get("OWNER"))
	// Output:
	// demo
	// no-scm
	// acme
}

func ExampleExportEnv() {
	env := []string{"NAME=demo", "DESC=it's here"}

	for _, line := range gmprj.ExportEnv(env) {
		fmt.Println(line)
	}
	// Output:
	// export NAME='demo'
	// export DESC='it'\''s here'
}
