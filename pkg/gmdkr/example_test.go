// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmdkr_test

import (
	"fmt"
	"strings"

	"github.com/ctx42/gmtask/pkg/gmdkr"
)

func ExampleImgName() {
	fmt.Println(gmdkr.ImgName("app"))
	fmt.Println(gmdkr.ImgName("dki-app"))
	// Output:
	// dki-app
	// dki-app
}

func ExampleImgTag() {
	fmt.Println(gmdkr.ImgTag("v1.2.4-dev.3+g7f93fb4"))
	// Output:
	// v1.2.4-dev.3_g7f93fb4
}

func ExampleBuild_Cmd() {
	bld, err := gmdkr.NewBuild(*gmdkr.NewConfig("dki-app", "v1.0.0"))
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("docker " + strings.Join(bld.Cmd(), " "))
	// Output:
	// docker build --platform linux/amd64 -t dki-app:v1.0.0 --file Dockerfile .
}
