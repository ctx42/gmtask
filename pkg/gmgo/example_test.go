// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo_test

import (
	"fmt"
	"os"
	"os/exec"
	"time"

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

func ExampleWatchTests() {
	// A stand-in for "go test -json ./..." printing one event.
	event := `{"Action":"output","Output":"ok  \texample.com/x\t0.1s\n"}`
	cmd := exec.Command("sh", "-c", "printf '%s\\n' '"+event+"'")

	err := gmgo.WatchTests(cmd, os.Stdout, 10*time.Minute)
	fmt.Println(err)
	// Output:
	// ok  	example.com/x	0.1s
	// <nil>
}
