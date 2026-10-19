//go:build unix

package probe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

func fakeCXX(t *testing.T, script string) build.CXX {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "clang++")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return build.CXX{Path: path, Family: build.Clang, Version: "fake"}
}

func TestRunFakeGood(t *testing.T) {
	cxx := fakeCXX(t, `printf '%s\n' '`+goodDecl+`'`)
	tab, err := new(Runner).Run(context.Background(), Config{CXX: cxx, Dir: t.TempDir()}, simpleSource(t))
	if err != nil {
		t.Fatal(err)
	}
	r := tab.Result(0)
	if !r.OK || r.Type != "int" || r.Category != CatPRValue || r.Key != "::BUFSIZ" {
		t.Errorf("result %+v", r)
	}
	if tab.Stats.Decls != 1 || tab.Stats.ExitCode != 0 || tab.Compiler == "" || tab.Key == "" {
		t.Errorf("table %+v", tab)
	}
}

func TestRunRejectsNonClang(t *testing.T) {
	cxx := fakeCXX(t, "exit 0")
	cxx.Family = build.GCC
	_, err := new(Runner).Run(context.Background(), Config{CXX: cxx}, simpleSource(t))
	if !errors.Is(err, ErrNoClang) {
		t.Fatalf("error %v", err)
	}
}

func TestRunGarbageOutput(t *testing.T) {
	cxx := fakeCXX(t, `echo hello world`)
	_, err := new(Runner).Run(context.Background(), Config{CXX: cxx}, simpleSource(t))
	if err == nil || !strings.Contains(err.Error(), "AST dump") {
		t.Fatalf("error %v", err)
	}
}

func TestRunNoOutputWithErrors(t *testing.T) {
	cxx := fakeCXX(t, `echo "gp_probe.cpp:2:1: error: use of undeclared identifier 'x'" >&2; exit 1`)
	src := simpleSource(t)
	tab, err := new(Runner).Run(context.Background(), Config{CXX: cxx}, src)
	if err != nil {
		t.Fatal(err)
	}
	r := tab.Result(0)
	if r.OK || r.Error != "no result for the probe (the compiler printed no error for it)" && !strings.Contains(r.Error, "undeclared") {
		t.Errorf("result %+v", r)
	}
	// Line 2 is an #include line, not a probe line: a header error.
	if len(tab.HeaderErrors) == 0 && r.Error == "" {
		t.Error("the error was lost")
	}
}

func TestRunFailureWithoutDecls(t *testing.T) {
	cxx := fakeCXX(t, `echo "clang: error: something is wrong" >&2; exit 2`)
	_, err := new(Runner).Run(context.Background(), Config{CXX: cxx}, simpleSource(t))
	if err == nil || !strings.Contains(err.Error(), "something is wrong") {
		t.Fatalf("error %v", err)
	}
}

func TestRunCrash(t *testing.T) {
	cxx := fakeCXX(t, `kill -SEGV $$`)
	_, err := new(Runner).Run(context.Background(), Config{CXX: cxx}, simpleSource(t))
	if err == nil || !strings.Contains(err.Error(), "signal") {
		t.Fatalf("error %v", err)
	}
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func readPID(t *testing.T, file string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(file); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no pid file")
	return 0
}

// A compiler that hangs is killed at the timeout, with its children.
func TestRunTimeoutKillsTheGroup(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("FAKE_PIDFILE", pidfile)
	cxx := fakeCXX(t, `sleep 300 &
echo $! > "$FAKE_PIDFILE"
sleep 300`)
	start := time.Now()
	_, err := new(Runner).Run(context.Background(), Config{CXX: cxx, Timeout: 300 * time.Millisecond}, simpleSource(t))
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error %v", err)
	}
	if time.Since(start) > 20*time.Second {
		t.Errorf("took %v", time.Since(start))
	}
	pid := readPID(t, pidfile)
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	for i := 0; i < 200 && alive(pid); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if alive(pid) {
		t.Errorf("the child %d of the compiler is still running", pid)
	}
}

func TestRunCancelKillsTheGroup(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("FAKE_PIDFILE", pidfile)
	cxx := fakeCXX(t, `sleep 300 &
echo $! > "$FAKE_PIDFILE"
sleep 300`)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := new(Runner).Run(ctx, Config{CXX: cxx}, simpleSource(t))
		done <- err
	}()
	pid := readPID(t, pidfile)
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	cancel()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "stopped") {
			t.Fatalf("error %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Run didn't return after the cancel")
	}
	for i := 0; i < 200 && alive(pid); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if alive(pid) {
		t.Errorf("the child %d of the compiler is still running", pid)
	}
}

func TestRunHugeDumpGuard(t *testing.T) {
	cxx := fakeCXX(t, `exec yes '{"name":"gp_probe_0","pad":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}'`)
	_, err := new(Runner).Run(context.Background(), Config{CXX: cxx, MaxJSON: 1 << 20, Timeout: time.Minute}, simpleSource(t))
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("error %v", err)
	}
}

func TestRunHugeDeclumpIsAnErrorOfThatProbe(t *testing.T) {
	cxx := fakeCXX(t, `printf '{"name":"gp_probe_0","pad":"'; head -c 3000000 /dev/zero | tr '\0' a; printf '"}'`)
	tab, err := new(Runner).Run(context.Background(), Config{CXX: cxx, MaxDecl: 1 << 20}, simpleSource(t))
	if err != nil {
		t.Fatal(err)
	}
	if r := tab.Result(0); r.OK || !strings.Contains(r.Error, "larger than the limit") || tab.Stats.TooLarge != 1 {
		t.Errorf("result %+v stats %+v", r, tab.Stats)
	}
}

func TestRunStderrIsCapped(t *testing.T) {
	cxx := fakeCXX(t, `yes 'gp_probe.cpp:1:1: warning: noise' | head -c 12000000 >&2; printf '%s\n' '`+goodDecl+`'`)
	tab, err := new(Runner).Run(context.Background(), Config{CXX: cxx}, simpleSource(t))
	if err != nil {
		t.Fatal(err)
	}
	if !tab.Stats.StderrTruncated || len(tab.Stderr) != maxStderr || tab.Stats.StderrBytes != 12000000 {
		t.Errorf("truncated %v, kept %d, total %d", tab.Stats.StderrTruncated, len(tab.Stderr), tab.Stats.StderrBytes)
	}
	if !tab.Result(0).OK {
		t.Error("the result was lost")
	}
}

func TestRunCleansTheTempFolder(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	for _, script := range []string{`printf '%s\n' '` + goodDecl + `'`, `echo hello`, `kill -SEGV $$`} {
		cxx := fakeCXX(t, script)
		new(Runner).Run(context.Background(), Config{CXX: cxx}, simpleSource(t))
		entries, _ := os.ReadDir(tmp)
		if len(entries) != 0 {
			t.Fatalf("after %q the temporary folder has %v", script, entries)
		}
	}
}

func TestRunPassesTheFlagsAndTheFolder(t *testing.T) {
	// The compiler runs in the package folder, with -iquote of it, the package's flags,
	// -fsyntax-only and the filtered JSON dump.
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_ARGS", out)
	cxx := fakeCXX(t, `pwd > "$FAKE_ARGS"; for a in "$@"; do echo "$a" >> "$FAKE_ARGS"; done; echo `+"'"+goodDecl+"'")
	if _, err := new(Runner).Run(context.Background(), Config{CXX: cxx, Dir: dir, Flags: []string{"-DX=5", "-I/inc"}}, simpleSource(t)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	got := string(b)
	for _, want := range []string{dir + "\n", "-std=c++20\n", "-iquote\n" + dir + "\n", "-DX=5\n", "-I/inc\n", "-fsyntax-only\n",
		"-ast-dump=json\n", "-ast-dump-filter=gp_probe_\n", "-ferror-limit=0\n", "-x\nc++\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in the compiler's arguments:\n%s", want, got)
		}
	}
}

// Callers of the same probe file wait for one compiler run.
func TestRunnerSingleflight(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	t.Setenv("FAKE_COUNTER", counter)
	cxx := fakeCXX(t, `echo run >> "$FAKE_COUNTER"
sleep 1
printf '%s\n' '`+goodDecl+`'`)
	rn := new(Runner)
	cfg := Config{CXX: cxx, Dir: t.TempDir()}
	src := simpleSource(t)
	const n = 6
	errs := make(chan error, n)
	cached := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			tab, err := rn.Run(context.Background(), cfg, src)
			if err == nil {
				cached <- tab.Cached
			}
			errs <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(counter)
	if got := strings.Count(string(data), "run"); got != 1 || rn.Runs() != 1 {
		t.Errorf("%d compiler runs (Runs %d), want 1", got, rn.Runs())
	}
	fresh := 0
	for i := 0; i < n; i++ {
		if !<-cached {
			fresh++
		}
	}
	if fresh != 1 {
		t.Errorf("%d fresh tables, want 1", fresh)
	}
}

// A probe whose type is not the helper's wrapper is never OK.
func TestRunUnwrappedTypeIsNotOK(t *testing.T) {
	cxx := fakeCXX(t, `printf '%s\n' '{"id":"0x1","kind":"TypeAliasDecl","name":"gp_probe_0","type":{"qualType":"gp_c"}}'`)
	tab, err := new(Runner).Run(context.Background(), Config{CXX: cxx, Dir: t.TempDir()}, simpleSource(t))
	if err != nil {
		t.Fatal(err)
	}
	if r := tab.Result(0); r.OK || !strings.Contains(r.Error, "collides") {
		t.Errorf("result %+v", r)
	}
}

// A compiler error on a line of the helper block fails the run.
func TestRunErrorOnAHelperLine(t *testing.T) {
	src := simpleSource(t)
	line := src.HelperFrom + 3
	cxx := fakeCXX(t, `echo "gp_probe.cpp:`+strconv.Itoa(line)+`:8: error: redefinition of 'gocpp_probe_internal_c'" >&2; exit 1`)
	_, err := new(Runner).Run(context.Background(), Config{CXX: cxx}, src)
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("error %v", err)
	}
	if src.HelperFrom <= 0 || src.HelperTo <= src.HelperFrom || !strings.HasPrefix(strings.Split(src.Text, "\n")[src.HelperFrom-1], "namespace gocpp_probe_internal_pr") {
		t.Errorf("helper lines %d..%d", src.HelperFrom, src.HelperTo)
	}
}

// S7a: a compiler that dies in the middle of the dump is reported with its signal and messages.
func TestRunCrashMidDump(t *testing.T) {
	cxx := fakeCXX(t, `printf '{"id":"0x1","kind":"TypeAliasDecl","name":"gp_probe_0","type":{"qual'
echo 'PLEASE submit a bug report to llvm' >&2
kill -SEGV $$`)
	_, err := new(Runner).Run(context.Background(), Config{CXX: cxx}, simpleSource(t))
	if err == nil || !strings.Contains(err.Error(), "signal") || !strings.Contains(err.Error(), "PLEASE submit a bug report") {
		t.Fatalf("error %v", err)
	}
}
