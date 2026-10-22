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
			if !waitFor(time.Minute, func() bool { return !alive(pid) }) { // (the bound only ends a hang)
				st := state(pid)
				syscall.Kill(pid, syscall.SIGKILL)
				t.Errorf("the compiler's child %d still runs after the probe returned (state %s)", pid, st)
			}
		})
	}
}

// slowFlags returns the flags that make the compiler (of the family of cxx) spend a long time,
// whatever the load is, on the probe file: they include a header with a constant evaluation of
// a billion steps (the limits are raised so that the compiler doesn't give up early). The test
// that uses them cancels the probe; nothing waits for the evaluation to end.
func slowFlags(t *testing.T, cxx build.CXX, dir string) []string {
	t.Helper()
	slow := filepath.Join(dir, "slow.h")
	src := "constexpr long gp_slow(long n) { long s = 0; for (long i = 0; i < n; i++) s += i % 7; return s; }\n" +
		"static_assert(gp_slow(1000000000L) > 0);\n"
	if err := os.WriteFile(slow, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	limits := []string{"-fconstexpr-steps=2000000000"}
	if cxx.Family == build.GCC {
		limits = []string{"-fconstexpr-ops-limit=2000000000", "-fconstexpr-loop-limit=2000000000"}
	}
	return append([]string{"-include", slow}, limits...)
}

func TestProbeCancelStopsTheCompiler(t *testing.T) {
	// A canceled probe returns at once, and no process of the compiler
	// (g++ runs cc1plus as a child) goes on with the probe file. The compiler is busy
	// with a slow constant evaluation (slowFlags), so that it can't finish before the
	// cancel however slowly the test polls /proc.
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		dir := t.TempDir()
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp) // the probe file is in a folder in tmp
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		flags := slowFlags(t, cxx, dir)
		go func() {
			_, err := newCache().probe(ctx, cxx, dir, flags, []string{"<vector>"}, []string{"std"})
			done <- err
		}()
		// Wait until the compiler works on the probe file (for g++, until
		// cc1plus runs too).
		probeFile := filepath.Join(tmp, "gocpp-probe-")
		want := 1
		if cxx.Family == build.GCC {
			want = 2
		}
		var scan time.Duration // how long one look at /proc takes: the load
		running := waitFor(60*time.Second, func() bool {
			select {
			case err := <-done:
				t.Fatalf("the probe ended before it was canceled: %v", err)
			default:
			}
			start := time.Now()
			n := len(processesWith(probeFile))
			scan = time.Since(start)
			return n >= want
		})
		if !running {
			t.Fatalf("the compiler didn't start on the probe file: processes %v", processesWith(probeFile))
		}
		// The budget is the time that the kill needs when the machine is idle, 200 ms, or
		// many times what a look at /proc takes now.
		budget := max(200*time.Millisecond, 20*scan)
		start := time.Now()
		cancel()
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "context canceled") {
				t.Errorf("canceled probe: %v, want a context error", err)
			}
			if d := time.Since(start); d > budget {
				t.Errorf("the canceled probe returned after %v, want at most %v (a look at /proc takes %v)", d, budget, scan)
			}
		case <-time.After(30 * time.Second):
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
