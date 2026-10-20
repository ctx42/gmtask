// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"bytes"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// fakeProc adds the process id, a child of ppid running the executable exe,
// to the proc file system mounted at dir.
func fakeProc(t tester.T, dir string, id, ppid int, exe string) {
	t.Helper()
	pid := oskit.MkdirAll(t, dir, strconv.Itoa(id))
	stat := strconv.Itoa(id) + " (a b) S " + strconv.Itoa(ppid) + " 1 1\n"
	oskit.Create(t, stat, pid, "stat")
	oskit.Create(t, exe+"\x00-test.v\x00", pid, "cmdline")
}

func Test_runWatched(t *testing.T) {
	t.Run("event output", func(t *testing.T) {
		// --- Given ---
		script := `printf '%s\n' '{"Action":"output","Output":"ok\n"}' plain`
		cmd := exec.Command("sh", "-c", script)
		out := &bytes.Buffer{}

		// --- When ---
		err := runWatched(cmd, out, time.Minute)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "ok\nplain\n", out.String())
	})

	t.Run("silence without test binaries", func(t *testing.T) {
		// --- Given ---
		cmd := exec.Command("sh", "-c", "sleep 0.3; echo done")
		out := &bytes.Buffer{}

		// --- When ---
		err := runWatched(cmd, out, 50*time.Millisecond)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "done\n", out.String())
	})

	t.Run("error - command fails", func(t *testing.T) {
		// --- Given ---
		cmd := exec.Command("sh", "-c", "exit 3")
		out := &bytes.Buffer{}

		// --- When ---
		err := runWatched(cmd, out, time.Minute)

		// --- Then ---
		assert.ErrorContain(t, "exit status 3", err)
	})

	t.Run("error - command cannot start", func(t *testing.T) {
		// --- Given ---
		cmd := exec.Command("/no/such/binary")
		out := &bytes.Buffer{}

		// --- When ---
		err := runWatched(cmd, out, time.Minute)

		// --- Then ---
		assert.ErrorContain(t, "go test: ", err)
	})
}

func Test_signalAll(t *testing.T) {
	t.Run("SIGQUIT", func(t *testing.T) {
		// --- Given ---
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		pid := cmd.Process.Pid
		sent := map[int]bool{}
		out := &bytes.Buffer{}

		// --- When ---
		signalAll(out, []int{pid}, sent, time.Second)

		// --- Then ---
		want := "" +
			"=== STALL: no test output for 1s: " +
			"SIGQUIT to 1, SIGKILL to 0 test binaries\n"
		assert.Equal(t, want, out.String())
		assert.Equal(t, map[int]bool{pid: true}, sent)
		assert.ErrorContain(t, "signal: quit", cmd.Wait())
	})

	t.Run("SIGKILL after SIGQUIT", func(t *testing.T) {
		// --- Given ---
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		pid := cmd.Process.Pid
		sent := map[int]bool{pid: true}
		out := &bytes.Buffer{}

		// --- When ---
		signalAll(out, []int{pid}, sent, time.Second)

		// --- Then ---
		want := "" +
			"=== STALL: no test output for 1s: " +
			"SIGQUIT to 0, SIGKILL to 1 test binaries\n"
		assert.Equal(t, want, out.String())
		assert.ErrorContain(t, "signal: killed", cmd.Wait())
	})
}

func Test_writeEvent_tabular(t *testing.T) {
	tt := []struct {
		testN string

		line string
		want string
	}{
		{"output event", `{"Action":"output","Output":"ok\n"}`, "ok\n"},
		{"event without output", `{"Action":"pass"}`, ""},
		{"not an event", "# example.com/x", "# example.com/x\n"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			out := &bytes.Buffer{}

			// --- When ---
			writeEvent(out, tc.line)

			// --- Then ---
			assert.Equal(t, tc.want, out.String())
		})
	}
}

func Test_testProcs(t *testing.T) {
	t.Run("test binaries under the process", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		fakeProc(t, dir, 10, 1, "go")
		fakeProc(t, dir, 11, 10, "/tmp/go-build1/b001/pkg.test")
		fakeProc(t, dir, 12, 10, "compile")
		fakeProc(t, dir, 13, 12, "/tmp/go-build1/b002/sub.test")
		fakeProc(t, dir, 20, 1, "/tmp/other/other.test")
		oskit.MkdirAll(t, dir, "self")

		// --- When ---
		have := testProcs(dir, 10)

		// --- Then ---
		assert.Equal(t, []int{11, 13}, have)
	})

	t.Run("no proc file system", func(t *testing.T) {
		// --- When ---
		have := testProcs("/no/such/proc", 10)

		// --- Then ---
		assert.Nil(t, have)
	})
}
