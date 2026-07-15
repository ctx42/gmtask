// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo_test

import (
	"fmt"

	"github.com/ctx42/gmtask/pkg/gmgo"
)

func ExampleLDFlags() {
	vars := []gmgo.LDVar{
		{Name: "version", Value: "v1.2.3"},
		{Name: "date", Value: "2026-01-02"},
	}

	fmt.Println(gmgo.LDFlags("app/build", vars))
	// Output: -X 'app/build.version=v1.2.3' -X 'app/build.date=2026-01-02'
}
