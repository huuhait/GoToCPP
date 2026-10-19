package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// setProcessGroup makes the command a process group of its own and kills the whole group when
// its context ends, so a timed-out gocpp does not leave its compiler behind.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 10 * time.Second
}

// TestProcessGroupKilledOnTimeout: when the context of a command ends, its children die with it
// (a gocpp that timed out must not leave g++ running).
func TestProcessGroupKilledOnTimeout(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 300 & echo $! > "+pidFile+"; wait")
	setProcessGroup(cmd)
	if err := cmd.Run(); err == nil {
		t.Fatal("the command was not stopped")
	}
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ { // bounded: 5 seconds
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Errorf("the child %d of the timed out command is still running", pid)
}
