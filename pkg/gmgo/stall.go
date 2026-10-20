// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gmgo

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// procDir is the directory of the /proc file system test binaries are found
// through.
const procDir = "/proc"

// runWatched runs cmd, a "go test -json" command, writing the output of its
// events to w, and signals the test binaries under it whenever they print
// nothing for stall: SIGQUIT, which makes a binary print its goroutines and
// exit, or SIGKILL to one that got SIGQUIT before. A line saying so is written
// to w. Silence while no test binary runs, as when compiling, is not a stall.
// It returns the error of the command, joined with one wrapping [ErrStall]
// when a binary was signalled.
func runWatched(cmd *exec.Cmd, w io.Writer, stall time.Duration) error {
	out, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("go test: %w", err)
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("go test: %w", err)
	}
	lines := make(chan string)
	go func() {
		scn := bufio.NewScanner(out)
		scn.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for scn.Scan() {
			lines <- scn.Text()
		}
		// A line too long stops the scan; the rest is drained so that
		// "go test" never blocks writing.
		_, _ = io.Copy(io.Discard, out)
		close(lines)
	}()

	tmr := time.NewTimer(stall)
	defer tmr.Stop()
	sent := make(map[int]bool)
loop:
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				break loop
			}
			writeEvent(w, line)

		case <-tmr.C:
			if pids := testProcs(procDir, cmd.Process.Pid); len(pids) > 0 {
				signalAll(w, pids, sent, stall)
			}
		}
		tmr.Reset(stall)
	}
	err = cmd.Wait()
	if len(sent) > 0 {
		format := "%w: no test output for %s"
		return errors.Join(fmt.Errorf(format, ErrStall, stall), err)
	}
	return err
}

// signalAll sends SIGQUIT to every process of pids, or SIGKILL to one sent
// marks, marks each in sent, and writes a line saying so to w, naming the
// duration stall the processes printed nothing for.
func signalAll(
	w io.Writer,
	pids []int,
	sent map[int]bool,
	stall time.Duration,
) {

	var quit, kill int
	for _, pid := range pids {
		sig := syscall.SIGQUIT
		if sent[pid] {
			sig = syscall.SIGKILL
		}
		prc, err := os.FindProcess(pid)
		if err != nil || prc.Signal(sig) != nil {
			continue
		}
		sent[pid] = true
		if sig == syscall.SIGQUIT {
			quit++
		} else {
			kill++
		}
	}
	format := "" +
		"=== STALL: no test output for %s: " +
		"SIGQUIT to %d, SIGKILL to %d test binaries\n"
	_, _ = fmt.Fprintf(w, format, stall, quit, kill)
}

// writeEvent writes to w the output of the "go test -json" event line; a line
// that is not an event is written as it is.
func writeEvent(w io.Writer, line string) {
	var evt struct {
		Output string `json:"Output"`
	}
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		_, _ = io.WriteString(w, line+"\n")
		return
	}
	_, _ = io.WriteString(w, evt.Output)
}

// testProcs returns, sorted, the IDs of the test binaries running under the
// process pid: every descendant whose executable name ends in ".test", read
// from the proc file system mounted at dir. It returns none when dir cannot be
// read.
func testProcs(dir string, pid int) []int {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	kids := make(map[int][]int)
	for _, ent := range ents {
		id, err := strconv.Atoi(ent.Name())
		if err != nil {
			continue
		}
		pth := filepath.Join(dir, ent.Name(), "stat")
		data, err := os.ReadFile(pth) //nolint:gosec
		if err != nil {
			continue
		}
		// The command name, in parentheses, may hold spaces; the state and
		// the parent ID follow the closing one.
		_, rest, ok := strings.Cut(string(data), ") ")
		fields := strings.Fields(rest)
		if !ok || len(fields) < 2 {
			continue
		}
		if ppid, err := strconv.Atoi(fields[1]); err == nil {
			kids[ppid] = append(kids[ppid], id)
		}
	}
	var pids []int
	todo := []int{pid}
	for len(todo) > 0 {
		cur := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		for _, kid := range kids[cur] {
			todo = append(todo, kid)
			pth := filepath.Join(dir, strconv.Itoa(kid), "cmdline")
			data, err := os.ReadFile(pth) //nolint:gosec
			if err != nil {
				continue
			}
			exe, _, _ := strings.Cut(string(data), "\x00")
			if strings.HasSuffix(filepath.Base(exe), ".test") {
				pids = append(pids, kid)
			}
		}
	}
	slices.Sort(pids)
	return pids
}
