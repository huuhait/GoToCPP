//go:build linux

package tests

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
	"github.com/huuhait/gocpp/internal/wintest"
)

// An interrupted run (SIGTERM, as Ctrl-C) ends its processes on the PC, removes its folder there and its work
// folder here. The run is a child process of this test binary: TestWindowsSuiteInterruptChild.
func TestWindowsSuiteInterrupt(t *testing.T) {
	host := pcOrSkip(t)
	probe, err := winssh.NewUnlocked(host)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestWindowsSuiteInterruptChild$", "-test.v", "-test.timeout=15m")
	cmd.Env = append(os.Environ(), "GOCPP_INTERRUPT_CHILD=1", "GOCPP_WINDOWS_SSH="+host)
	cmd.Dir = filepath.Join(repoRoot, "tests")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var log strings.Builder
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 100)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	var runID, work string
	var seen []string
	ready := false
	deadline := time.After(10 * time.Minute)
	for !ready {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatalf("the child ended before it was ready:\n%s\n%s", strings.Join(seen, "\n"), log.String())
			}
			seen = append(seen, l)
			if v, ok := strings.CutPrefix(l, "CHILD-RUN "); ok {
				runID = v
			}
			if v, ok := strings.CutPrefix(l, "CHILD-WORK "); ok {
				work = v
			}
			ready = strings.HasPrefix(l, "CHILD-BUILD")
		case <-deadline:
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			t.Fatalf("the child was not ready:\n%s", strings.Join(seen, "\n"))
		}
	}
	if runID == "" || work == "" {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		t.Fatalf("the child did not say its run and work folder: %v", seen)
	}
	defer forceRemove(probe, runID)
	if n := waitProcs(t, probe, runID, func(n int) bool { return n >= 3 }); n < 3 { // the agent and two programs
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		t.Fatalf("the run's programs did not start on the PC (%d processes)", n)
	}
	time.Sleep(5 * time.Second)
	if err := syscall.Kill(cmd.Process.Pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for range lines {
		}
		done <- cmd.Wait()
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Minute):
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		t.Fatal("the child did not end after SIGTERM")
	}
	if n := waitProcs(t, probe, runID, func(n int) bool { return n == 0 }); n != 0 {
		t.Errorf("%d processes of the interrupted run are still on the PC", n)
	}
	res, err := probe.Exec(t.Context(), time.Minute, nil, `if exist ..\`+runID+` (echo present) else (echo gone)`)
	if err != nil || !strings.Contains(string(res.Stdout), "gone") {
		t.Errorf("the run folder %s is still on the PC: %v %s", runID, err, res.Stdout)
	}
	if _, err := os.Stat(work); err == nil {
		t.Errorf("the work folder %s is still here\nchild output:\n%s\nchild stderr:\n%s", work, strings.Join(seen, "\n"), log.String())
		os.RemoveAll(work)
	}
}

// TestWindowsSuiteInterruptChild is the run that TestWindowsSuiteInterrupt interrupts: two programs that sleep
// for ten minutes, through the same machine, signal hook and lock as TestWindowsSuite.
func TestWindowsSuiteInterruptChild(t *testing.T) {
	if os.Getenv("GOCPP_INTERRUPT_CHILD") != "1" {
		t.Skip("run by TestWindowsSuiteInterrupt")
	}
	setup(t)
	m := newSuiteMachine(t, windowsHost(), false)
	fmt.Printf("CHILD-RUN %s\nCHILD-WORK %s\n", m.remote.(sshRemote).r.RunID(), m.work)
	tree := buildTree(t)
	cfg := wintest.Config{
		Remote: m.remote, Lock: &wintest.FileLock{Path: windowsLockPath(), Wait: 3 * time.Hour}, AgentExe: m.agentExe, Work: m.work,
		Jobs: 2, BuildJobs: 2, BatchSize: 10, Timeout: 20 * time.Minute, ResultsPath: filepath.Join(m.work, "results.json"), Generation: "x", Target: "real PC",
		Build: func(ctx context.Context, it wintest.Item) (string, error) {
			data, err := os.ReadFile(tree)
			if err != nil {
				return "", err
			}
			p := filepath.Join(m.work, strings.ReplaceAll(it.Name, "/", "_")+".exe")
			fmt.Println("CHILD-BUILD")
			return p, os.WriteFile(p, data, 0o755)
		},
		Check: func(wintest.Item, wintest.Outcome) []string { return nil },
	}
	items := []wintest.Item{
		{Name: "x/one", Group: "x", Args: []string{"sleep"}, Key: "1"},
		{Name: "x/two", Group: "x", Args: []string{"sleep"}, Key: "2"},
	}
	rep, err := m.run(cfg, items)
	t.Fatalf("the run ended by itself: %v %v", rep, err)
}
