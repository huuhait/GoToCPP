//go:build unix

package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The runtime library of a key is made by one process, whatever the number of processes that ask
// for it at the same time (gocpp build in an editor, in a terminal and in CI, one cache): the others
// wait for its lock file and use its library. The tests of rtlib_test.go do it with builds in
// one process; here the builds are processes (this test binary, run again), as in real life.

const xprocEnv = "GOCPP_TEST_RTLIB_XPROC"

func TestRuntimeLibAcrossProcesses(t *testing.T) {
	e := newRTEnv(t)
	slow := fakeCompiler(t, GCC, "sleep 0.05")
	const n = 5
	results := filepath.Join(t.TempDir(), "results")
	if err := os.Mkdir(results, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmds := make([]*exec.Cmd, n)
	for i := range cmds {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRuntimeLibChildProcess$", "-test.count=1")
		cmd.Env = append(os.Environ(), xprocEnv+"="+strings.Join([]string{e.cache, slow.Path, e.ar, filepath.Join(results, fmt.Sprint(i))}, "|"))
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		setPdeathsig(cmd.SysProcAttr)
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		cmd.WaitDelay = 10 * time.Second
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds[i] = cmd
	}
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("process %d: %v", i, err)
		}
	}
	var first string
	for i := range cmds {
		data, err := os.ReadFile(filepath.Join(results, fmt.Sprint(i)))
		if err != nil || !libOK(string(data)) || first != "" && string(data) != first {
			t.Fatalf("process %d: library %q, %v (first %q)", i, data, err, first)
		}
		first = string(data)
	}
	if got := len(lines(t, slow.Path)); got != nSources() {
		t.Errorf("%d processes compiled %d files, want %d: one process builds, the others wait", n, got, nSources())
	}
	if got := len(lines(t, e.ar)); got != 1 {
		t.Errorf("the archiver ran %d times, want 1", got)
	}
	dir, _ := ExtractRuntime(e.cache)
	if got := libFolders(t, dir); len(got) != 1 {
		t.Errorf("lib/ holds %q, want one folder", got)
	}
	assertNoTemp(t, filepath.Join(dir, "lib"))
}

// TestRuntimeLibChildProcess is the process of TestRuntimeLibAcrossProcesses: it asks for the
// runtime library and writes its path to a file.
func TestRuntimeLibChildProcess(t *testing.T) {
	arg := os.Getenv(xprocEnv)
	if arg == "" {
		t.Skip("only as a process of TestRuntimeLibAcrossProcesses")
	}
	parts := strings.Split(arg, "|")
	cache, cxx, ar, result := parts[0], parts[1], parts[2], parts[3]
	t.Setenv("GOCPP_NOPCH", "1")
	t.Setenv("GOCPP_NORTLIB", "")
	t.Setenv("GOCPP_AR", ar)
	b := &Builder{CXX: CXX{Path: cxx, Family: GCC, Version: "15"}, GOOS: "linux", Work: t.TempDir(), Cache: cache}
	_, objs, err := b.runtime(context.Background(), b.runtimeFlags(), false)
	if err != nil || len(objs) != 1 {
		t.Fatalf("runtime: %v %v", objs, err)
	}
	if err := os.WriteFile(result, []byte(objs[0]), 0o644); err != nil {
		t.Fatal(err)
	}
}
