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
		{"time limit", "(" + spin() + ") &\necho $! > \"$0.child\"\n" + spin()},
		// The compiler exits, but leaves a child running that closed its
		// output: the probe kills what is left of the group.
		{"left behind", "(exec >/dev/null 2>&1; " + spin() + ") &\necho $! > \"$0.child\"\nexit 0"},
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

// slowFlags returns the flags that keep the compiler (of the family of cxx) on the probe file
// for minutes, whatever the load is: they include a header with slow constant evaluations whose
// limits are raised so that the compiler doesn't give up early. On a quiet machine g++ needs about
// 250 s for its two evaluations (each stops at the raised operation limit) and clang++ about 300 s
// for its one: far more than the minute the test waits for a probe to return, and over by
// themselves soon after a test that failed. The test that uses them cancels the probe.
func slowFlags(t *testing.T, cxx build.CXX, dir string) []string {
	t.Helper()
	slow := filepath.Join(dir, "slow.h")
	src := "constexpr long gp_slow(long n) { long s = 0; for (long i = 0; i < n; i++) s += i % 7; return s; }\n"
	limits := []string{"-fconstexpr-steps=2000000000"}
	if cxx.Family == build.GCC {
		// (the arguments differ, so that the second evaluation is not the first one's cached answer)
		src += "static_assert(gp_slow(1000000000L) > 0);\nstatic_assert(gp_slow(1000000001L) > 0);\n"
		limits = []string{"-fconstexpr-ops-limit=1000000000", "-fconstexpr-loop-limit=2000000000"}
	} else {
		src += "static_assert(gp_slow(200000000L) > 0);\n"
	}
	if err := os.WriteFile(slow, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return append([]string{"-include", slow}, limits...)
}

// killGroups kills the process group of every process whose command line contains s (the
// compiler and its children are a group of their own, build.InGroup), and the process itself. The
// group is read from /proc/<pid>/stat right after the command line, in the same look at the
// process, so that a pid that was reused in between is not taken for the compiler.
func killGroups(s string) {
	dirs, _ := os.ReadDir("/proc")
	for _, d := range dirs {
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		cmdline, err := os.ReadFile("/proc/" + d.Name() + "/cmdline")
		if err != nil || !strings.Contains(string(cmdline), s) {
			continue
		}
		stat, err := os.ReadFile("/proc/" + d.Name() + "/stat")
		if err != nil {
			continue
		}
		// "pid (comm) state ppid pgrp ...": the fields after the last ")".
		var group int
		if i := bytes.LastIndexByte(stat, ')'); i >= 0 {
			if f := strings.Fields(string(stat[i+1:])); len(f) >= 3 {
				group, _ = strconv.Atoi(f[2])
			}
		}
		if group > 1 && group != syscall.Getpgrp() {
			syscall.Kill(-group, syscall.SIGKILL)
		}
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

func TestProbeCancelStopsTheCompiler(t *testing.T) {
	// A canceled probe returns without waiting for the compiler, and no process of the
	// compiler (g++ runs cc1plus as a child) goes on with the probe file. The compiler is busy
	// for minutes with slow constant evaluations (slowFlags): a probe that returned within the
	// minute did not wait for it, so no clock is needed to tell; the minute only turns a probe
	// that never returns into a failure.
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		dir := t.TempDir()
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp) // the probe file is in a folder in tmp
		probeFile := filepath.Join(tmp, "gocpp-probe-")
		// A test that fails (a probe that does not stop its compiler) leaves no compiler behind.
		t.Cleanup(func() { killGroups(probeFile) })
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
		want := 1
		if cxx.Family == build.GCC {
			want = 2
		}
		running := waitFor(time.Minute, func() bool {
			select {
			case err := <-done:
				t.Fatalf("the probe ended before it was canceled: %v", err)
			default:
			}
			return len(processesWith(probeFile)) >= want
		})
		if !running {
			t.Fatalf("the compiler didn't start on the probe file: processes %v", processesWith(probeFile))
		}
		start := time.Now()
		cancel()
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "context canceled") {
				t.Errorf("canceled probe: %v, want a context error", err)
			}
			t.Logf("the canceled probe returned after %v", time.Since(start))
		case <-time.After(time.Minute):
			t.Fatal("the canceled probe didn't return in a minute")
		}
		if pids := processesWith(probeFile); len(pids) > 0 {
			t.Errorf("processes %v still run with the probe file after the probe returned", pids)
		}
	})
}
