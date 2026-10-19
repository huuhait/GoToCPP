//go:build linux

package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// ---------------------------------------------------------------- real processes

// TestChildProbe is the body of the processes that TestCacheTwoProcessesOneRun starts.
func TestChildProbe(t *testing.T) {
	prog := os.Getenv("GOCPP_TEST_PROBE_CHILD")
	if prog == "" {
		t.Skip("only as a child of TestCacheTwoProcessesOneRun")
	}
	cfg := Config{CXX: build.CXX{Path: prog, Family: build.Clang, Version: "fake"}, Dir: os.Getenv("GOCPP_TEST_PROBE_DIR"), CacheDir: os.Getenv("GOCPP_TEST_PROBE_CACHE")}
	names := fnNames(8)
	tab, err := new(Runner).Probe(context.Background(), cfg, fnSet(names...), gc(names...))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(tab.Results)
	fmt.Fprintf(os.Stdout, "RESULT %s cached=%v\n", b, tab.Cached)
}

// Four gocpp processes that need the same answers at the same time: one compiler run, and
// all of them get the same results.
func TestCacheTwoProcessesOneRun(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	t.Setenv("FAKE_SLEEP", "1")
	t.Setenv("GOCPP_TEST_PROBE_CHILD", f.cxx.Path)
	t.Setenv("GOCPP_TEST_PROBE_DIR", f.dir)
	t.Setenv("GOCPP_TEST_PROBE_CACHE", f.cache)
	const n = 4
	cmds := make([]*exec.Cmd, n)
	outs := make([]*strings.Builder, n)
	for i := range cmds {
		outs[i] = new(strings.Builder)
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestChildProbe$", "-test.v")
		cmds[i].Stdout = outs[i]
		cmds[i].SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
		c := cmds[i]
		t.Cleanup(func() { syscall.Kill(-c.Process.Pid, syscall.SIGKILL); c.Wait() })
	}
	var results, cachedN []string
	for i, c := range cmds {
		if err := c.Wait(); err != nil {
			t.Fatalf("child %d: %v\n%s", i, err, outs[i])
		}
		for _, l := range strings.Split(outs[i].String(), "\n") {
			if r, ok := strings.CutPrefix(l, "RESULT "); ok {
				res, cached, _ := strings.Cut(r, " cached=")
				results = append(results, res)
				cachedN = append(cachedN, cached)
			}
		}
	}
	if len(results) != n {
		t.Fatalf("%d children reported", len(results))
	}
	for _, r := range results {
		if r != results[0] || !strings.Contains(r, `"type":"R_f0"`) {
			t.Errorf("results differ: %s", r)
		}
	}
	if f.runs() != 1 {
		t.Errorf("%d compiler runs for %d processes", f.runs(), n)
	}
	fresh := 0
	for _, c := range cachedN {
		if c == "false" {
			fresh++
		}
	}
	if fresh != 1 {
		t.Errorf("%d processes ran the compiler", fresh)
	}
	if entries, _ := os.ReadDir(filepath.Join(f.cache, "locks")); len(entries) != 0 {
		t.Errorf("locks left: %v", entries)
	}
}
