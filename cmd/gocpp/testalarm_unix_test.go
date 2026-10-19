//go:build unix

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// alarmDriver returns a driver that is enough for execTest.
func alarmDriver(t *testing.T) *driver {
	t.Helper()
	d := &driver{flags: &buildFlags{}, sigs: make(chan os.Signal, 4), done: make(chan struct{})}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	t.Cleanup(func() { signal.Stop(d.sigs); d.cancel() })
	return d
}

// pidGone reports whether the process pid is gone, waiting a bounded time for init to reap it.
func pidGone(pid int) bool {
	for i := 0; i < 500; i++ {
		if syscall.Kill(pid, 0) != nil {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestWedgedProgram is not a test: TestExecTestBackupAlarm runs this test binary again with
// GOCPP_TEST_WEDGED set, as a stand-in for a test binary that does not stop. "child" is the process
// that the wedged program started: it only waits. Otherwise the program starts a child, writes its
// pid to the file in GOCPP_TEST_PIDFILE, and waits; with "ignore" it ignores SIGQUIT first.
func TestWedgedProgram(t *testing.T) {
	mode := os.Getenv("GOCPP_TEST_WEDGED")
	if mode == "" {
		t.Skip("run by TestExecTestBackupAlarm")
	}
	if mode == "env" {
		os.Stdout.WriteString("ASAN_OPTIONS=" + os.Getenv("ASAN_OPTIONS") + "\n")
		return
	}
	if mode == "ignore" || mode == "child" {
		signal.Ignore(syscall.SIGQUIT) // a program that does not stop; Go's runtime dumps its goroutines and exits otherwise
	}
	if mode == "child" {
		signal.Ignore(syscall.SIGINT) // like a background job of a shell
		os.WriteFile(os.Getenv("GOCPP_TEST_PIDFILE")+".ready", nil, 0o644)
	}
	if mode != "child" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestWedgedProgram$")
		cmd.Env = append(os.Environ(), "GOCPP_TEST_WEDGED=child")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		// The pid is written when the child has set up its signals.
		for i := 0; i < 1000; i++ {
			if _, err := os.Stat(os.Getenv("GOCPP_TEST_PIDFILE") + ".ready"); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		os.WriteFile(os.Getenv("GOCPP_TEST_PIDFILE"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	}
	select {}
}

// TestExecTestBackupAlarm checks the alarm of execTest on a stand-in for a wedged test binary: SIGQUIT
// to the program, SIGKILL to its process group after testHardKillDelay, and nothing left running.
func TestExecTestBackupAlarm(t *testing.T) {
	old := testHardKillDelay
	t.Cleanup(func() { testHardKillDelay = old })

	run := func(t *testing.T, mode string, hard time.Duration) (killed string, child int, elapsed time.Duration, err error) {
		dir := t.TempDir()
		pidFile := filepath.Join(dir, "pid")
		t.Setenv("GOCPP_CHECK_PROCESS", "1") // TestMain makes no folders of its own for the stand-in
		t.Setenv("GOCPP_TEST_WEDGED", mode)
		t.Setenv("GOCPP_TEST_PIDFILE", pidFile)
		testHardKillDelay = hard
		d := alarmDriver(t)
		var out bytes.Buffer
		t0 := time.Now()
		err = d.execTest(&program{exe: os.Args[0], dir: dir}, []string{"-test.run=^TestWedgedProgram$"}, &out, 1500*time.Millisecond, &killed)
		elapsed = time.Since(t0)
		data, rerr := os.ReadFile(pidFile)
		if rerr != nil {
			t.Fatalf("the program did not start its child: %v (output %q)", rerr, out.String())
		}
		child, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		if child <= 0 {
			t.Fatalf("bad child pid in %q", data)
		}
		return killed, child, elapsed, err
	}

	t.Run("sigquit", func(t *testing.T) {
		// Go's runtime dumps the goroutines and exits with status 2 on SIGQUIT, long before the hard kill.
		killed, child, elapsed, err := run(t, "wedged", 15*time.Second)
		var ee *exec.ExitError
		if killed != "quit" || !errors.As(err, &ee) || ee.ExitCode() != 2 {
			t.Errorf("killed = %q, err = %v; want quit, exit status 2 (SIGQUIT)", killed, err)
		}
		if elapsed > 10*time.Second {
			t.Errorf("took %v: the program was not stopped by SIGQUIT", elapsed)
		}
		if !pidGone(child) {
			t.Errorf("the process that the program started (%d) is still running", child)
		}
	})
	t.Run("sigkill", func(t *testing.T) {
		// The program ignores SIGQUIT: the hard kill ends it and the process it started.
		killed, child, _, err := run(t, "ignore", 300*time.Millisecond)
		if killed != "quit" || err == nil || !strings.Contains(err.Error(), "signal: killed") {
			t.Errorf("killed = %q, err = %v; want quit, signal: killed", killed, err)
		}
		if !pidGone(child) {
			t.Errorf("the process that the program started (%d) is still running", child)
		}
	})
}

// TestInterruptKillsGroup checks that a SIGINT that gocpp test gets (the terminal sends it to gocpp only,
// the test binary is in a group of its own) is passed on to the test binary and its group, and that a
// process of the tests that ignores it (a background job of a shell does) does not outlive the run.
func TestInterruptKillsGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	t.Setenv("GOCPP_CHECK_PROCESS", "1")
	t.Setenv("GOCPP_TEST_WEDGED", "wedged")
	t.Setenv("GOCPP_TEST_PIDFILE", pidFile)
	d := alarmDriver(t)
	go d.watchSignals()
	t.Cleanup(func() { close(d.done) })
	go func() {
		// Once the program has started its child.
		for i := 0; i < 1000; i++ {
			if data, err := os.ReadFile(pidFile); err == nil && len(data) > 0 {
				d.sigs <- syscall.SIGINT
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	var killed string
	var out bytes.Buffer
	err := d.execTest(&program{exe: os.Args[0], dir: dir}, []string{"-test.run=^TestWedgedProgram$"}, &out, 0, &killed)
	data, rerr := os.ReadFile(pidFile)
	if rerr != nil {
		t.Fatalf("the program did not start its child: %v (output %q)", rerr, out.String())
	}
	child, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if err == nil {
		t.Errorf("the program ended without error after SIGINT")
	}
	if child <= 0 || !pidGone(child) {
		t.Errorf("the process that the program started (%d) is still running after SIGINT", child)
	}
}

// TestASanLeakCheckIsOffForTests checks the environment of the test binary: with -asan, LeakSanitizer is
// turned off (memory is reference counted, so a cycle that a test leaves is no failure of the test)
// unless the caller set ASAN_OPTIONS, and without -asan nothing is set.
func TestASanLeakCheckIsOffForTests(t *testing.T) {
	t.Setenv("GOCPP_CHECK_PROCESS", "1")
	t.Setenv("GOCPP_TEST_WEDGED", "env")
	for _, tt := range []struct {
		name string
		asan bool
		env  string // ASAN_OPTIONS of the caller, "" for none
		want string
	}{
		{"asan", true, "", "ASAN_OPTIONS=detect_leaks=0\n"},
		{"asan with options", true, "detect_leaks=1", "ASAN_OPTIONS=detect_leaks=1\n"},
		{"no asan", false, "", "ASAN_OPTIONS=\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASAN_OPTIONS", tt.env)
			d := alarmDriver(t)
			d.flags.asan = tt.asan
			var out bytes.Buffer
			var killed string
			if err := d.execTest(&program{exe: os.Args[0], dir: t.TempDir()}, []string{"-test.run=^TestWedgedProgram$"}, &out, 0, &killed); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(out.String(), tt.want) {
				t.Errorf("the test binary printed %q, want it to start with %q", out.String(), tt.want)
			}
		})
	}
}
