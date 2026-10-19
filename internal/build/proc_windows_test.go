//go:build windows

package build

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The job objects of InGroup, Start and KillGroup (proc_windows.go): canceling a command, or the end
// of a command that left a process behind, ends the whole tree. The tests run on the real PC
// (tests/windows_native_test.go runs this package's Windows tests there). The helpers are this test
// binary itself, started again with GOCPP_JOB_HELPER set.

// TestJobHelper is the child of the tests below: "parent" starts a "sleeper" after a pause (the
// start of a process is not inside its job for a moment, see proc_windows.go) and prints its process
// number; "leaver" does the same and exits at once, leaving the sleeper; "sleeper" waits.
func TestJobHelper(t *testing.T) {
	switch os.Getenv("GOCPP_JOB_HELPER") {
	case "":
		t.Skip("a helper of the job tests")
	case "sleeper":
		time.Sleep(10 * time.Minute)
	case "owner":
		// What gocpp does: start a command in a group, then wait. The test kills this process.
		ctx := context.Background()
		cmd, err := Start(ctx, func() *exec.Cmd {
			c := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestJobHelper$")
			c.Env = append(os.Environ(), "GOCPP_JOB_HELPER=sleeper")
			InGroup(c)
			return c
		})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(cmd.Process.Pid)
		time.Sleep(10 * time.Minute)
	case "parent", "leaver":
		time.Sleep(700 * time.Millisecond)
		c := exec.Command(os.Args[0], "-test.run=^TestJobHelper$")
		c.Env = append(os.Environ(), "GOCPP_JOB_HELPER=sleeper")
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		fmt.Println(c.Process.Pid)
		if os.Getenv("GOCPP_JOB_HELPER") == "leaver" {
			return
		}
		time.Sleep(10 * time.Minute)
	}
}

func alive(pid int) bool {
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}

// waitGone waits until the process is gone, up to a limit that only a stuck machine reaches.
func waitGone(pid int) bool {
	for range 300 {
		if !alive(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func startHelper(t *testing.T, ctx context.Context, mode string) (*exec.Cmd, int) {
	t.Helper()
	var out *bufio.Reader
	cmd, err := Start(ctx, func() *exec.Cmd {
		c := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestJobHelper$")
		c.Env = append(os.Environ(), "GOCPP_JOB_HELPER="+mode)
		pipe, _ := c.StdoutPipe()
		out = bufio.NewReader(pipe)
		InGroup(c)
		return c
	})
	if err != nil {
		t.Fatal(err)
	}
	line, err := out.ReadString('\n')
	if err != nil {
		t.Fatalf("the helper printed nothing: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("the helper printed %q", line)
	}
	return cmd, pid
}

func TestJobCancelKillsTheTree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd, grandchild := startHelper(t, ctx, "parent")
	defer KillGroup(cmd)
	if !alive(grandchild) {
		t.Fatal("the grandchild is not running before the cancel")
	}
	cancel()
	cmd.Wait()
	if !waitGone(grandchild) {
		t.Errorf("process %d, a child of the command, survived the cancel", grandchild)
	}
}

func TestJobKillGroupAfterExit(t *testing.T) {
	cmd, grandchild := startHelper(t, context.Background(), "leaver")
	cmd.Wait() // the command is over; its child still runs
	if !alive(grandchild) {
		t.Fatal("the grandchild ended with its parent: nothing to show")
	}
	KillGroup(cmd)
	if !waitGone(grandchild) {
		t.Errorf("process %d survived KillGroup", grandchild)
	}
}

func TestJobAfterTheJobIsGone(t *testing.T) {
	// KillGroup twice, and Cancel after KillGroup, do nothing and do not fail.
	cmd, _ := startHelper(t, context.Background(), "leaver")
	cmd.Wait()
	KillGroup(cmd)
	KillGroup(cmd)
}

// Killing gocpp (here the "owner" helper, which started a command in a group) ends the command too: the job
// is created with kill-on-close, and the system closes the owner's handle when it dies.
func TestJobDiesWithItsOwner(t *testing.T) {
	var out *bufio.Reader
	owner := exec.Command(os.Args[0], "-test.run=^TestJobHelper$")
	owner.Env = append(os.Environ(), "GOCPP_JOB_HELPER=owner")
	pipe, _ := owner.StdoutPipe()
	out = bufio.NewReader(pipe)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer owner.Process.Kill()
	line, err := out.ReadString('\n')
	if err != nil {
		t.Fatalf("the owner printed nothing: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("the owner printed %q", line)
	}
	if !alive(pid) {
		t.Fatal("the command is not running before the owner is killed")
	}
	owner.Process.Kill()
	owner.Wait()
	if !waitGone(pid) {
		t.Errorf("process %d, the command of a killed owner, survived", pid)
	}
}
