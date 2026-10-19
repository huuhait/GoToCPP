//go:build unix

package tests

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The commands that the tests run start process groups of their own: gocpp
// run starts a C++ compiler and then the test program, and when a test
// times out, the program must stop with gocpp instead of running on.

// running holds the process groups of the commands that run now.
var running = struct {
	sync.Mutex
	groups map[int]bool
	// stopping is set by stopRunning, for good: the test binary is about
	// to die, and a command that starts after that is killed at once.
	stopping bool
}{groups: make(map[int]bool)}

// inGroup makes cmd start a process group of its own, and makes canceling
// it (a timeout, too much output) stop the whole group: with SIGTERM first,
// so that gocpp can remove its work folder. What is still running after
// the command's WaitDelay is killed (see run).
func inGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
}

// track records that the process group pgid of a started command runs, or
// no longer does. While the harness stops (stopRunning), a group that
// starts is killed instead: the parallel tests go on starting commands
// until the test binary dies.
func track(pgid int, runs bool) {
	running.Lock()
	defer running.Unlock()
	switch {
	case !runs:
		delete(running.groups, pgid)
	case running.stopping:
		killGroup(pgid)
	default:
		running.groups[pgid] = true
	}
}

// isStopping reports whether stopRunning was called: then run starts no
// more commands.
func isStopping() bool {
	running.Lock()
	defer running.Unlock()
	return running.stopping
}

// killGroup kills the process group pgid. The harness calls it after each
// command too, for any process that the command left running.
func killGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// stopRunning stops the process groups of the commands that run now: with
// SIGTERM, so that gocpp can clean up, and half a second later with
// SIGKILL. It is for good (see track): it is called when the test binary is
// about to die.
func stopRunning() {
	running.Lock()
	running.stopping = true
	var groups []int
	for pgid := range running.groups {
		groups = append(groups, pgid)
	}
	running.Unlock()
	windowsStop() // the Wine server of the Windows mode, if there is one (windows_test.go)
	if len(groups) == 0 {
		return
	}
	for _, pgid := range groups {
		syscall.Kill(-pgid, syscall.SIGTERM)
	}
	time.Sleep(500 * time.Millisecond)
	for _, pgid := range groups {
		killGroup(pgid)
	}
}

// killOnSignal makes an interrupt (Ctrl-C), a hangup or a termination kill
// the running process groups and remove the folder tmp (TestMain's, with
// the gocpp binary) before the test binary dies of the signal: the groups
// are not the terminal's, so the signal doesn't reach them, and TestMain
// doesn't get to clean up. A signal that was ignored when the test binary
// started (a background job, nohup) stays ignored.
func killOnSignal(tmp string) {
	var sigs []os.Signal
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} {
		if !signal.Ignored(sig) {
			sigs = append(sigs, sig)
		}
	}
	if len(sigs) == 0 {
		return
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, sigs...)
	go func() {
		sig := <-c
		stopRunning()
		os.RemoveAll(tmp)
		signal.Reset(sig)
		syscall.Kill(os.Getpid(), sig.(syscall.Signal))
	}()
}

func TestStopIsForGood(t *testing.T) {
	// The commands that start while stopRunning works, and after it, must
	// not survive it: the parallel tests go on starting commands until the
	// test binary dies.
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("needs sleep:", err)
	}
	defer func() { // stopRunning is for good: put things back for the other tests
		running.Lock()
		running.stopping = false
		running.Unlock()
	}()
	start := func() *exec.Cmd {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), sleep, "60")
		inGroup(cmd)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		track(cmd.Process.Pid, true)
		return cmd
	}
	before := start()
	done := make(chan struct{})
	go func() { stopRunning(); close(done) }()
	time.Sleep(100 * time.Millisecond) // most likely inside its half second between SIGTERM and SIGKILL
	during := start()
	<-done
	after := start()
	if !isStopping() {
		t.Error("isStopping() = false after stopRunning")
	}
	for _, c := range []struct {
		when string
		cmd  *exec.Cmd
	}{{"before", before}, {"during", during}, {"after", after}} {
		exited := make(chan error, 1)
		go func() { exited <- c.cmd.Wait() }()
		select {
		case err := <-exited:
			if err == nil {
				t.Errorf("the command started %s stopRunning exited by itself, want it killed", c.when)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("the command started %s stopRunning still runs", c.when)
			killGroup(c.cmd.Process.Pid)
			<-exited
		}
		track(c.cmd.Process.Pid, false)
	}
}
