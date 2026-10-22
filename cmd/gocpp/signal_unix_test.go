//go:build unix

package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The signal tests run the test binary as gocpp (GOCPP_TEST_MAIN=1, see
// TestMain), in a process group of its own, with a TMPDIR of its own, and
// check that gocpp stops and leaves no work folder behind: the test
// harness (tests/README.md) stops a command with SIGTERM to its process
// group and kills it 2 seconds later.

// sleepMain is a C++ main function that says it runs and then sleeps.
const sleepMain = `#include <chrono>
#include <cstdio>
#include <thread>
int main() {
    std::puts("ready");
    std::fflush(stdout);
    std::this_thread::sleep_for(std::chrono::seconds(60));
}
`

// crashMain is a C++ main function that says it runs and then raises the signal @SIG@. It must
// not leave a core dump: with a core_pattern that pipes to systemd-coredump, RLIMIT_CORE 0 does
// not stop the dump (the kernel starts the handler anyway), and under load a handler that has
// many crashes to log holds the dying process for minutes. PR_SET_DUMPABLE 0 does stop it.
const crashMain = `#include <csignal>
#include <cstdio>
#include <sys/resource.h>
#ifdef __linux__
#include <sys/prctl.h>
#endif
int main() {
    rlimit no{0, 0};
    setrlimit(RLIMIT_CORE, &no);
#ifdef __linux__
    prctl(PR_SET_DUMPABLE, 0);
#endif
    std::puts("ready");
    std::fflush(stdout);
    std::raise(@SIG@);
}
`

// startGocpp starts the test binary as gocpp with args in the folder dir,
// with the extra environment env and a new TMPDIR, which it returns. The
// process is killed at the end of the test.
func startGocpp(t *testing.T, dir string, env []string, args ...string) (cmd *exec.Cmd, stdout *bufio.Reader, stderr *bytes.Buffer, tmp string) {
	t.Helper()
	return startArgv(t, dir, env, append([]string{os.Args[0]}, args...))
}

// startArgv is startGocpp for the command line argv, which runs the test
// binary as gocpp (maybe through sh).
func startArgv(t *testing.T, dir string, env []string, argv []string) (cmd *exec.Cmd, stdout *bufio.Reader, stderr *bytes.Buffer, tmp string) {
	t.Helper()
	tmp = t.TempDir()
	cmd = exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), append([]string{"GOCPP_TEST_MAIN=1", "TMPDIR=" + tmp}, env...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr = new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cmd.Process.Kill()
		// A child that can't die (it is held by the kernel while a core dump handler
		// works on it) must not block the test binary for good: the test's timeout would
		// then be the only end of it.
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Minute):
			t.Logf("gocpp (pid %d) did not die within a minute of SIGKILL", cmd.Process.Pid)
		}
	})
	return cmd, bufio.NewReader(pipe), stderr, tmp
}

// waitExit waits up to a minute (it only ends a hang) for cmd to exit and returns its exit
// status.
func waitExit(t *testing.T, cmd *exec.Cmd, stdout *bufio.Reader) int {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		stdout.WriteTo(new(bytes.Buffer)) // read the rest, so that Wait can finish
		done <- cmd.Wait()
	}()
	select {
	case <-done:
	case <-time.After(time.Minute):
		t.Fatal("gocpp didn't exit within a minute after the signal")
	}
	return cmd.ProcessState.ExitCode()
}

// checkSignaled fails unless cmd died of the signal sig, without a core
// dump.
func checkSignaled(t *testing.T, cmd *exec.Cmd, sig syscall.Signal) {
	t.Helper()
	ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	switch {
	case !ok:
		t.Fatalf("no wait status: %v", cmd.ProcessState)
	case runtime.GOOS != "linux":
		// gocpp exits with 128+n where it can't raise the signal again.
		if ws.Signaled() && ws.Signal() != sig || !ws.Signaled() && ws.ExitStatus() != 128+int(sig) {
			t.Errorf("gocpp: %v, want death by %v or exit status %d", cmd.ProcessState, sig, 128+int(sig))
		}
	case !ws.Signaled() || ws.Signal() != sig:
		t.Errorf("gocpp: %v, want death by the signal %q", cmd.ProcessState, sig)
	case ws.CoreDump():
		t.Errorf("gocpp dumped a core of its own: %v", cmd.ProcessState)
	}
}

func TestDiesLikeTheProgram(t *testing.T) {
	// The program dies of a signal: gocpp dies of the same one, prints
	// nothing and leaves no work folder; also for the signals that Go's
	// runtime handles itself (SIGSEGV, SIGABRT) or ignores when kill sends
	// them (SIGPIPE).
	c := anyCXX(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"hello.gp": helloSrc})
	for _, sig := range []syscall.Signal{syscall.SIGPIPE, syscall.SIGSEGV, syscall.SIGABRT, syscall.SIGINT, syscall.SIGKILL} {
		t.Run(sig.String(), func(t *testing.T) {
			main := filepath.Join(dir, "main.cpp")
			src := strings.Replace(crashMain, "@SIG@", strconv.Itoa(int(sig)), 1)
			if err := os.WriteFile(main, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd, stdout, stderr, tmp := startGocpp(t, dir, []string{"GOCPP_CXX=" + c.name, "GOCPP_TEST_MAIN_CPP=" + main}, "run", "hello.gp")
			if line, err := stdout.ReadString('\n'); line != "ready\n" {
				t.Fatalf("the program printed %q, %v; stderr:\n%s", line, err, stderr)
			}
			waitExit(t, cmd, stdout)
			checkSignaled(t, cmd, sig)
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want nothing", stderr)
			}
			checkNoWork(t, tmp)
		})
	}
}

// checkNoWork fails if TMPDIR tmp has a work folder left.
func checkNoWork(t *testing.T, tmp string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(tmp, "gocpp-build*"))
	if len(left) > 0 {
		t.Errorf("the work folder %s is left", left[0])
	}
}

func TestSignalWhileRunning(t *testing.T) {
	c := anyCXX(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"hello.gp": helloSrc, "main.cpp": sleepMain})
	for _, group := range []bool{false, true} {
		name := "gocpp"
		if group {
			name = "group" // like the test harness
		}
		t.Run(name, func(t *testing.T) {
			cmd, stdout, stderr, tmp := startGocpp(t, dir, []string{"GOCPP_CXX=" + c.name, "GOCPP_TEST_MAIN_CPP=" + filepath.Join(dir, "main.cpp")}, "run", "hello.gp")
			line, err := stdout.ReadString('\n')
			if line != "ready\n" {
				t.Fatalf("the program printed %q, %v; stderr:\n%s", line, err, stderr)
			}
			pid := cmd.Process.Pid
			if group {
				pid = -pid
			}
			if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			// gocpp passes SIGTERM on to the program (or the program got it
			// too), and dies of it like the program.
			waitExit(t, cmd, stdout)
			checkSignaled(t, cmd, syscall.SIGTERM)
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want nothing", stderr)
			}
			checkNoWork(t, tmp)
		})
	}
}

func TestSignalWhileBuilding(t *testing.T) {
	// The compiler hangs; SIGTERM to gocpp alone kills it (it runs in a
	// process group of its own), and gocpp removes the work folder and
	// exits with 128+15.
	c := anyCXX(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"hello.gp": helloSrc})
	pidFile := filepath.Join(dir, "cxx.pid")
	slow := filepath.Join(dir, "slow-c++")
	writeScript(t, slow, "for a in \"$@\"; do if [ \"$a\" = -c ]; then echo $$ > "+shellQuote(pidFile)+"; sleep 60; fi; done\nexec "+shellQuote(c.cxx.Path)+" \"$@\"\n")
	cmd, stdout, stderr, tmp := startGocpp(t, dir, []string{"GOCPP_CXX=" + slow}, "build", "hello.gp")
	var pid int
	for stop := time.Now().Add(30 * time.Second); pid == 0; time.Sleep(20 * time.Millisecond) {
		if data, err := os.ReadFile(pidFile); err == nil && bytes.HasSuffix(data, []byte("\n")) {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		if time.Now().After(stop) {
			t.Fatalf("the compiler didn't start; stderr:\n%s", stderr)
		}
	}
	if err := syscall.Kill(cmd.Process.Pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitExit(t, cmd, stdout)
	checkSignaled(t, cmd, syscall.SIGTERM)
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	checkNoWork(t, tmp)
	// The compiler is gone (a zombie that nobody reaped counts as gone).
	for stop := time.Now().Add(time.Minute); ; time.Sleep(20 * time.Millisecond) {
		err := syscall.Kill(pid, 0)
		stat, _ := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil || bytes.Contains(stat, []byte(") Z ")) {
			break
		}
		if time.Now().After(stop) {
			t.Fatalf("the compiler (pid %d) still runs after gocpp exited", pid)
		}
	}
}

func TestIgnoredSignalsStayIgnored(t *testing.T) {
	// Under nohup (here: trap "" HUP), gocpp keeps ignoring SIGHUP, and the
	// program inherits that: a hangup stops neither.
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc")
	}
	c := anyCXX(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"hello.gp": helloSrc, "main.cpp": sleepMain})
	cmd, stdout, stderr, _ := startArgv(t, dir, []string{"GOCPP_CXX=" + c.name, "GOCPP_TEST_MAIN_CPP=" + filepath.Join(dir, "main.cpp")},
		[]string{"/bin/sh", "-c", `trap "" HUP; exec "$0" "$@"`, os.Args[0], "run", "hello.gp"})
	if line, err := stdout.ReadString('\n'); line != "ready\n" {
		t.Fatalf("the program printed %q, %v; stderr:\n%s", line, err, stderr)
	}
	pid := cmd.Process.Pid // gocpp: sh ran exec
	// The program is the child of one of gocpp's threads.
	tasks, _ := filepath.Glob("/proc/" + strconv.Itoa(pid) + "/task/*/children")
	var f []string
	for _, task := range tasks {
		children, _ := os.ReadFile(task)
		f = append(f, strings.Fields(string(children))...)
	}
	if len(f) != 1 {
		t.Fatalf("the children of gocpp: %q", f)
	}
	prog, _ := strconv.Atoi(f[0])
	status, err := os.ReadFile("/proc/" + f[0] + "/status")
	if err != nil {
		t.Fatal(err)
	}
	var ign uint64
	for _, line := range strings.Split(string(status), "\n") {
		if v, ok := strings.CutPrefix(line, "SigIgn:"); ok {
			ign, _ = strconv.ParseUint(strings.TrimSpace(v), 16, 64)
		}
	}
	if ign&1 == 0 {
		t.Errorf("the program doesn't ignore SIGHUP: SigIgn %#x", ign)
	}
	syscall.Kill(pid, syscall.SIGHUP)
	syscall.Kill(prog, syscall.SIGHUP)
	time.Sleep(300 * time.Millisecond)
	for _, p := range []int{pid, prog} {
		stat, _ := os.ReadFile("/proc/" + strconv.Itoa(p) + "/stat")
		if syscall.Kill(p, 0) != nil || bytes.Contains(stat, []byte(") Z ")) {
			t.Errorf("process %d (gocpp %d, the program %d) died of a SIGHUP that it ignores", p, pid, prog)
		}
	}
	// End it: SIGTERM is not ignored. gocpp dies of it like the program.
	syscall.Kill(pid, syscall.SIGTERM)
	waitExit(t, cmd, stdout)
	checkSignaled(t, cmd, syscall.SIGTERM)
}
