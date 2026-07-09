package gmclog_test

import (
	"fmt"
	"time"

	"github.com/ctx42/gmtask/pkg/lib/gmclog"
)

func ExampleRelease_String() {
	date := time.Date(2000, 1, 2, 3, 4, 6, 0, time.UTC)
	rel, err := gmclog.NewRelease("v0.1.2", date)
	if err != nil {
		fmt.Println(err)
		return
	}
	rel.AddChange("Add feature", "Fix bug")

	fmt.Print(rel.String())
	// Output:
	// ## v0.1.2 (Sun, 02 Jan 2000 03:04:06 UTC)
	// - Add feature.
	// - Fix bug.
}

func ExampleReadReleases() {
	cl, err := gmclog.ReadReleases("testdata", "changelog_0.md")
	if err != nil {
		fmt.Println(err)
		return
	}

	rel := cl.Releases[0]
	fmt.Println(rel.Version.Original())
	fmt.Println(rel.Changes)
	// Output:
	// v0.1.5
	// [- Change 1. - Change 2.]
}
