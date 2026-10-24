package build

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/load"
)

// watched starts the command (in a group of its own, as the compilers are), watches it with
// WatchPipes, and returns stop's error and Wait's. The pipes are opened once at the end, so that
// nothing stays stuck if the watch failed; a run that takes more than a minute fails the test.
func watched(t *testing.T, cmd *exec.Cmd, pipes ...string) (pipeErr, waitErr error) {
	t.Helper()
	t.Cleanup(func() {
		for _, pipe := range pipes {
			if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
				f.Close()
			}
		}
	})
	InGroup(cmd)
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = 5 * time.Second // as every compiler run of gocpp has one (WatchPipes)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { KillGroup(cmd) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		stop := WatchPipes(cmd)
		waitErr = cmd.Wait()
		pipeErr = stop()
	}()
	select {
	case <-done:
	case <-time.After(time.Minute):
		KillGroup(cmd)
		<-done
		t.Fatalf("%v still runs after a minute: the wait for the named pipe was not found", cmd.Args)
	}
	return pipeErr, waitErr
}

// pipeIn makes the named pipe name in dir.
func pipeIn(t *testing.T, dir, name string) string {
	t.Helper()
	pipe := filepath.Join(dir, name)
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	return pipe
}

// mayNameThePipe reports whether this system lets gocpp read the system call of its child
// processes (kernel.yama.ptrace_scope below 2): then a PipeError names the pipe.
func mayNameThePipe() bool {
	b, err := os.ReadFile("/proc/sys/kernel/yama/ptrace_scope")
	return err != nil || len(b) > 0 && b[0] < '2'
}

// TestWatchPipesStopsAReader: a process that opens a named pipe without a writer is stopped, the
// process that runs it as a child too, with the pipe's absolute name (relative to the folder of
// the command, or absolute), as load's error for a named pipe.
func TestWatchPipesStopsAReader(t *testing.T) {
	testStopsAReader(t)
}

// TestWatchPipesStopsAReaderByScan: the same on a kernel without the children files of /proc: the
// children of a process are found by the parent in the stat of every process (childrenByScan).
func TestWatchPipesStopsAReaderByScan(t *testing.T) {
	saved := childrenFiles
	childrenFiles = func() bool { return false }
	defer func() { childrenFiles = saved }()
	testStopsAReader(t)
}

func testStopsAReader(t *testing.T) {
	dir := t.TempDir()
	pipe := pipeIn(t, dir, "pipe.h")
	for _, c := range []struct {
		name string
		args []string
	}{
		{"relative", []string{"cat", "pipe.h"}},
		{"absolute", []string{"cat", pipe}},
		{"child", []string{"sh", "-c", "cat pipe.h; echo after"}}, // cat is a child of sh
		{"grandchild", []string{"sh", "-c", "sh -c 'cat pipe.h; echo after'; echo after"}},
		{"cwd-changed", []string{"sh", "-c", "cd / && cat \"$0\"", "./" + pipe[1:]}},
		// cat leaves the group (and writes its pid into the file pid): it is stopped too, and does
		// not hold the output that Wait reads (checked below).
		{"setsid", []string{"sh", "-c", "setsid sh -c 'echo $$ > pid; exec cat pipe.h'; echo after"}},
		// cat ignores SIGTERM (inherited from the shell): it gets SIGKILL after pipeStopGrace.
		{"ignores-TERM", []string{"sh", "-c", "trap '' TERM; exec cat pipe.h"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), c.args[0], c.args[1:]...)
			cmd.Dir = dir
			var out bytes.Buffer
			cmd.Stdout = &out
			pipeErr, waitErr := watched(t, cmd, pipe)
			if c.name == "setsid" {
				b, err := os.ReadFile(filepath.Join(dir, "pid"))
				if err != nil {
					t.Fatal(err)
				}
				waitGone(t, strings.TrimSpace(string(b)))
			}
			var pe *PipeError
			if !errors.As(pipeErr, &pe) {
				t.Fatalf("stop() = %v (Wait: %v), want a *PipeError", pipeErr, waitErr)
			}
			if waitErr == nil {
				t.Errorf("Wait() = nil, want the error of a killed process")
			}
			if !mayNameThePipe() {
				t.Skipf("the system hides the system calls of child processes: %v", pe)
			}
			if pe.Path != pipe {
				t.Errorf("PipeError.Path = %q, want %q", pe.Path, pipe)
			}
			if !load.IsNotRegular(pe) {
				t.Errorf("load.IsNotRegular(%v) = false", pe)
			}
			if want := "open " + pipe + ": not a regular file: it is a named pipe (the compiler waited for a writer of it and was stopped)"; pe.Error() != want {
				t.Errorf("Error() = %q\nwant      %q", pe.Error(), want)
			}
		})
	}
}

// TestWatchPipesLeavesOtherWaits: a command that waits for something else (a sleep, a read of a
// named pipe that has a writer) is not stopped.
func TestWatchPipesLeavesOtherWaits(t *testing.T) {
	saved := pipePollMax
	pipePollMax = 50 * time.Millisecond // many looks while the command runs
	defer func() { pipePollMax = saved }()
	t.Run("sleep", func(t *testing.T) {
		pipeErr, waitErr := watched(t, exec.CommandContext(t.Context(), "sleep", "1"))
		if pipeErr != nil || waitErr != nil {
			t.Errorf("stop() = %v, Wait() = %v; want nil, nil", pipeErr, waitErr)
		}
	})
	t.Run("pipe-with-writer", func(t *testing.T) {
		dir := t.TempDir()
		pipe := pipeIn(t, dir, "fed.h")
		// The writer holds the pipe open for a second before it writes: cat waits in a read
		// (pipe_read), not in the open, and must go on.
		w, err := os.OpenFile(pipe, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			time.Sleep(time.Second)
			w.WriteString("int x;\n")
			w.Close()
		}()
		cmd := exec.CommandContext(t.Context(), "cat", pipe)
		pipeErr, waitErr := watched(t, cmd, pipe)
		if pipeErr != nil || waitErr != nil {
			t.Errorf("stop() = %v, Wait() = %v; want nil, nil", pipeErr, waitErr)
		}
	})
	t.Run("not-started", func(t *testing.T) {
		if err := WatchPipes(exec.CommandContext(t.Context(), "true"))(); err != nil {
			t.Errorf("stop() of a command that did not start = %v", err)
		}
	})
}

// TestCompileIncludesANamedPipe: a C file of a package whose #include names a named pipe, next to
// it or in an -I folder, is a compile error that names the pipe, not a compiler that waits for
// ever; with both compilers, and the compiler's temporary files do not stay in its TMPDIR (gcc's
// cc*.s: the driver is stopped with SIGTERM first).
func TestCompileIncludesANamedPipe(t *testing.T) {
	t.Setenv("CCACHE_DISABLE", "1") // the compiler itself, with its own temporary files
	for _, name := range []string{"g++", "clang++"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("GOCPP_CXX", name)
			cxx, err := FindCXX(t.Context())
			if err != nil {
				t.Skip(err)
			}
			cc, err := FindCC(t.Context(), cxx)
			if err != nil {
				t.Skip(err)
			}
			for _, c := range []struct{ name, include, pipe string }{
				{"next-to-it", `"pipe.h"`, "pipe.h"},
				{"through-I", "<deep.h>", "inc/deep.h"},
				{"nested", `"ok.h"`, "pipe.h"}, // ok.h includes pipe.h
			} {
				t.Run(c.name, func(t *testing.T) { compileIncludesAPipe(t, cc, c.include, c.pipe) })
			}
		})
	}
}

// compileIncludesAPipe compiles a C file with the line #include include, where the file pipe (in the
// folder of the C file) is a named pipe, and checks the error and the compiler's TMPDIR.
func compileIncludesAPipe(t *testing.T, cc CXX, include, name string) {
	dir := t.TempDir()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	if err := os.Mkdir(filepath.Join(dir, "inc"), 0o755); err != nil {
		t.Fatal(err)
	}
	pipe := pipeIn(t, dir, name)
	src := filepath.Join(dir, "a.c")
	if err := os.WriteFile(src, []byte("#include "+include+"\nint f(void) { return 1; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.h"), []byte("#include \"pipe.h\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			f.Close()
		}
	})
	b := &Builder{Work: t.TempDir()}
	done := make(chan error, 1)
	go func() {
		done <- b.run(t.Context(), cc, "the C file a.c", false,
			[]string{"-iquote", dir, "-I", filepath.Join(dir, "inc"), "-c", "-x", "c", src, "-o", filepath.Join(b.Work, "a.o")})
	}()
	var err error
	select {
	case err = <-done:
	case <-time.After(time.Minute):
		t.Fatal("the compile still runs after a minute")
	}
	var ce *CompileError
	var pe *PipeError
	if !errors.As(err, &ce) || !errors.As(err, &pe) {
		t.Fatalf("compile = %v, want a CompileError with a PipeError", err)
	}
	if mayNameThePipe() && pe.Path != pipe {
		t.Errorf("PipeError.Path = %q, want %q", pe.Path, pipe)
	}
	if left, _ := os.ReadDir(tmp); len(left) > 0 {
		var names []string
		for _, e := range left {
			names = append(names, e.Name())
		}
		t.Errorf("the stopped compiler left %v in its TMPDIR", names)
	}
}

// TestChildrenByScanIsShared: the look at every process is made once for all the watchers of the
// moment: within procScanLife it is used again (a child that started after it is not in it), and
// after that a new look finds the child.
func TestChildrenByScanIsShared(t *testing.T) {
	procScan.Lock()
	procScan.children = nil
	procScan.Unlock()
	childrenByScan(os.Getpid()) // the look
	cmd := exec.CommandContext(t.Context(), "sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	has := func() bool {
		for _, p := range childrenByScan(os.Getpid()) {
			if p == cmd.Process.Pid {
				return true
			}
		}
		return false
	}
	if has() {
		t.Error("a look younger than procScanLife was made again")
	}
	procScan.Lock()
	procScan.at = procScan.at.Add(-2 * procScanLife)
	procScan.Unlock()
	if !has() {
		t.Error("a new look does not find the child")
	}
}

// waitGone waits until the process pid has ended (it is not in /proc, or it is a zombie), and fails
// the test when it still runs after a minute.
func waitGone(t *testing.T, pid string) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		stat, err := os.ReadFile("/proc/" + pid + "/stat")
		if i := bytes.LastIndexByte(stat, ')'); err != nil || i >= 0 && strings.HasPrefix(string(stat[i+1:]), " Z") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("process %s, which waited for the pipe in a session of its own, still runs after a minute", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestWatchPipesLeavesTheCommandAlone: WatchPipes does not change the command after its Start: the
// goroutine of os/exec that ends a command whose context is done reads it at the same time (go test
// -race reported a data race when WatchPipes set the WaitDelay). The context here ends while the
// command runs, so that goroutine works during the watch.
func TestWatchPipesLeavesTheCommandAlone(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sleep", "1")
	before := cmd.WaitDelay
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := WatchPipes(cmd)
	werr := cmd.Wait()
	if err := stop(); err != nil {
		t.Errorf("stop() = %v for a command that waits for no pipe", err)
	}
	if werr == nil {
		t.Error("Wait() = nil, want the error of a command whose context ended")
	}
	if cmd.WaitDelay != before {
		t.Errorf("WaitDelay = %v after the watch, want %v: WatchPipes changed the command", cmd.WaitDelay, before)
	}
}
