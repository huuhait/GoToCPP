package interop

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests in this file check that a stopped probe stops all the
// processes of the compiler (the process group, build.InGroup). They read
// /proc, so they are for Linux.

// state returns the state of the process pid, as /proc/<pid>/stat says
// it ("R", "S", "Z" for a zombie: killed, but not yet reaped by its new
// parent), "gone" when there is no such process, or a description of what
// went wrong.
func state(pid int) string {
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return "gone"
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, fs.ErrNotExist) {
		return "gone" // a zombie that was reaped after the kill(pid, 0)
	}
	if err != nil {
		return err.Error()
	}
	// "pid (comm) S ...": the state follows the last ")".
	if i := bytes.LastIndexByte(stat, ')'); i >= 0 && i+2 < len(stat) {
		return string(stat[i+2])
	}
	return "unreadable stat " + string(stat)
}

// alive reports whether the process pid may still run: only "gone" and a
// zombie count as dead.
func alive(pid int) bool {
	s := state(pid)
	return s != "gone" && s != "Z"
}

// processesWith returns the PIDs of the running processes whose command
// line contains s.
func processesWith(s string) []int {
	var pids []int
	dirs, _ := os.ReadDir("/proc")
	for _, d := range dirs {
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		cmdline, err := os.ReadFile("/proc/" + d.Name() + "/cmdline")
		if err == nil && strings.Contains(string(cmdline), s) && alive(pid) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// waitFor waits up to d for cond, and reports whether it held.
func waitFor(d time.Duration, cond func() bool) bool {
	for stop := time.Now().Add(d); ; time.Sleep(10 * time.Millisecond) {
		if cond() {
			return true
		}
		if time.Now().After(stop) {
			return false
		}
	}
}

func TestProbeKillsTheProcessGroup(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		// The compiler hangs, with a child that keeps the output open: the
		// probe stops at its time limit, and the child must be gone too.
		{"time limit", "(while :; do :; done) &\necho $! > \"$0.child\"\nwhile :; do :; done"},
		// The compiler exits, but leaves a child running that closed its
		// output: the probe kills what is left of the group.
		{"left behind", "(exec >/dev/null 2>&1; while :; do :; done) &\necho $! > \"$0.child\"\nexit 0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cxx := fakeCompiler(t, build.GCC, c.body)
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			if _, err := newCache().probe(ctx, cxx, t.TempDir(), nil, []string{"<vector>"}, []string{"std"}); err == nil {
				t.Fatal("probe with a broken compiler: no error")
			}
			data, err := os.ReadFile(cxx.Path + ".child")
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			if !waitFor(time.Second, func() bool { return !alive(pid) }) {
				st := state(pid)
				syscall.Kill(pid, syscall.SIGKILL)
				t.Errorf("the compiler's child %d still runs after the probe returned (state %s)", pid, st)
			}
		})
	}
}

func TestProbeCancelStopsTheCompiler(t *testing.T) {
	// A canceled probe returns at once, and no process of the compiler
	// (g++ runs cc1plus as a child) goes on with the probe file.
	heavy := []string{"<vector>", "<filesystem>", "<regex>", "<ranges>", "<format>", "<chrono>", "<iostream>",
		"<map>", "<unordered_map>", "<algorithm>", "<future>", "<random>", "<variant>", "<complex>", "<valarray>"}
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		dir := t.TempDir()
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp) // the probe file is in a folder in tmp
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, err := newCache().probe(ctx, cxx, dir, nil, heavy, []string{"std"})
			done <- err
		}()
		// Wait until the compiler works on the probe file (for g++, until
		// cc1plus runs too).
		probeFile := filepath.Join(tmp, "gocpp-probe-")
		want := 1
		if cxx.Family == build.GCC {
			want = 2
		}
		if !waitFor(5*time.Second, func() bool { return len(processesWith(probeFile)) >= want }) {
			t.Fatalf("the compiler didn't start on the probe file: processes %v", processesWith(probeFile))
		}
		time.Sleep(50 * time.Millisecond)
		start := time.Now()
		cancel()
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "context canceled") {
				t.Errorf("canceled probe: %v, want a context error", err)
			}
			if d := time.Since(start); d > 200*time.Millisecond {
				t.Errorf("the canceled probe returned after %v, want at most 200ms", d)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("the canceled probe didn't return")
		}
		if pids := processesWith(probeFile); len(pids) > 0 {
			for _, pid := range pids {
				syscall.Kill(pid, syscall.SIGKILL)
			}
			t.Errorf("processes %v still run with the probe file after the probe returned", pids)
		}
	})
}
