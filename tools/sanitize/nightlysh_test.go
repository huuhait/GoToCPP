//go:build linux

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These tests run the real tools/sanitize/nightly.sh in a throwaway git repository whose tools/sanitize is a
// stub (it exits with the status its arguments ask for, or sleeps), with SANITIZE_DEVSHELL empty so that no
// nix is needed. They test the script: the snapshot, the lock, the history, the exit status, the cleanup
// when a night is killed.

const stubTool = `package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	args := strings.Join(os.Args[1:], " ")
	if mark := os.Getenv("STUB_MARK"); mark != "" {
		os.WriteFile(mark, []byte(fmt.Sprint(os.Getpid())), 0o644)
		// a job that is asked to stop (TERM) says so, as the real tool cleans up when it is
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM)
		if strings.Contains(args, "ignoreterm") {
			signal.Ignore(syscall.SIGTERM)
		}
		go func() {
			<-ch
			os.WriteFile(mark+".term", nil, 0o644)
			os.Exit(143)
		}()
	}
	if cwd := os.Getenv("STUB_CWD"); cwd != "" {
		d, _ := os.Getwd()
		os.WriteFile(cwd, []byte(d), 0o644)
	}
	switch {
	case strings.Contains(args, "ignoreterm"): // a job that does not stop when asked: only KILL ends it
		time.Sleep(5 * time.Minute)
	case strings.Contains(args, "sleep"):
		time.Sleep(time.Minute)
	case strings.Contains(args, "bogus"):
		fmt.Fprintln(os.Stderr, "sanitize: unknown stage")
		os.Exit(2)
	case strings.Contains(args, "fail"):
		os.Exit(1)
	}
}
`

type nightlyEnv struct {
	repo, results, cache string
}

func newNightlyEnv(t *testing.T) *nightlyEnv { return newNightlyEnvWith(t, stubTool) }

// newNightlyEnvWith makes the repository with the given source for tools/sanitize/main.go.
func newNightlyEnvWith(t *testing.T, tool string) *nightlyEnv {
	t.Helper()
	for _, tool := range []string{"git", "go", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(root, "tools/sanitize/nightly.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	e := &nightlyEnv{repo: filepath.Join(dir, "repo"), results: filepath.Join(dir, "results"), cache: filepath.Join(dir, "cache")}
	writeTree(t, e.repo, map[string]string{"go.mod": "module stub\n\ngo 1.21\n", "tools/sanitize/main.go": tool})
	if err := os.WriteFile(filepath.Join(e.repo, "tools/sanitize/nightly.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "stub"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = e.repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return e
}

func (e *nightlyEnv) cmd(args ...string) *exec.Cmd {
	cmd := exec.Command("sh", append([]string{filepath.Join(e.repo, "tools/sanitize/nightly.sh")}, args...)...)
	cmd.Dir = e.repo
	cmd.Env = append(os.Environ(), "SANITIZE_RESULTS="+e.results, "SANITIZE_CACHE="+e.cache, "SANITIZE_DEVSHELL=", "SANITIZE_RUNNER=")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	return cmd
}

func (e *nightlyEnv) history(t *testing.T) string {
	b, _ := os.ReadFile(filepath.Join(e.results, "history.txt"))
	return string(b)
}

func (e *nightlyEnv) worktrees(t *testing.T) string {
	cmd := exec.Command("git", "worktree", "list")
	cmd.Dir = e.repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// nights lists the folders of the nights.
func (e *nightlyEnv) nights(t *testing.T) []string {
	ents, _ := os.ReadDir(e.results)
	var out []string
	for _, d := range ents {
		if d.IsDir() {
			out = append(out, d.Name())
		}
	}
	return out
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

func TestNightlyScriptExitStatusAndCleanup(t *testing.T) {
	commit := func(e *nightlyEnv) string {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = e.repo
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out))
	}
	for _, tt := range []struct {
		name string
		args []string
		want int
	}{
		{"a clean night", nil, 0},
		{"a failed stage", []string{"fail"}, 1},
		{"a usage error is 2, not go run's 1", []string{"-stages", "bogus"}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newNightlyEnv(t)
			cmd := e.cmd(tt.args...)
			err := cmd.Run()
			if got := exitCode(err); got != tt.want {
				t.Fatalf("exit %d, want %d", got, tt.want)
			}
			h := e.history(t)
			if !strings.Contains(h, " exit "+string(rune('0'+tt.want))+" "+commit(e)) {
				t.Errorf("history.txt = %q", h)
			}
			if strings.Count(e.worktrees(t), "\n") != 1 {
				t.Errorf("the snapshot is still a worktree:\n%s", e.worktrees(t))
			}
			nights := e.nights(t)
			if len(nights) != 1 {
				t.Fatalf("nights = %v", nights)
			}
			if _, err := os.Stat(filepath.Join(e.results, nights[0], "src")); err == nil {
				t.Error("the snapshot folder is still there")
			}
			if b, _ := os.ReadFile(filepath.Join(e.results, nights[0], "commit.txt")); strings.TrimSpace(string(b)) != commit(e) {
				t.Errorf("commit.txt = %q", b)
			}
		})
	}
}

// waitFor waits until the file exists (a bounded number of tries).
func waitFor(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 600; i++ {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s did not appear", path)
}

func processGone(pid string) bool {
	stat, err := os.ReadFile("/proc/" + pid + "/stat")
	return err != nil || strings.Contains(string(stat), ") Z ")
}

func TestNightlyScriptKilledNightCleansUp(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			e := newNightlyEnv(t)
			mark := filepath.Join(t.TempDir(), "mark")
			cmd := e.cmd("sleep")
			cmd.Env = append(cmd.Env, "STUB_MARK="+mark)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waitFor(t, mark) // the job runs, in the snapshot
			pidb, _ := os.ReadFile(mark)
			defer func() {
				// the script stopped the job with TERM (which the real tool answers by cleaning up), not with KILL
				if _, err := os.Stat(mark + ".term"); err != nil {
					t.Error("the job was not sent TERM")
				}
			}()
			if !strings.Contains(e.worktrees(t), "/src") {
				t.Errorf("no snapshot while the night runs:\n%s", e.worktrees(t))
			}
			// the whole group, as systemd or a terminal does
			syscall.Kill(-cmd.Process.Pid, sig)
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				if got := exitCode(err); got != 128+int(sig) {
					t.Errorf("exit %d, want %d", got, 128+int(sig))
				}
			case <-time.After(60 * time.Second):
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				t.Fatal("the script did not end after the signal")
			}
			if strings.Count(e.worktrees(t), "\n") != 1 {
				t.Errorf("the snapshot is still a worktree:\n%s", e.worktrees(t))
			}
			for _, n := range e.nights(t) {
				if _, err := os.Stat(filepath.Join(e.results, n, "src")); err == nil {
					t.Errorf("night %s kept its snapshot", n)
				}
			}
			if h := e.history(t); !strings.Contains(h, "killed by") || !strings.Contains(h, " exit "+string(rune('0'+(128+int(sig))/100))) {
				t.Errorf("history.txt = %q", h)
			}
			for i := 0; !processGone(string(pidb)) && i < 100; i++ {
				time.Sleep(50 * time.Millisecond)
			}
			if !processGone(string(pidb)) {
				syscall.Kill(mustAtoi(string(pidb)), syscall.SIGKILL)
				t.Errorf("the job (pid %s) still runs", pidb)
			}
		})
	}
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range strings.TrimSpace(s) {
		n = n*10 + int(c-'0')
	}
	return n
}

// TestNightlyScriptRemovesTheSnapshotOfAKilledNight: after a kill -9 (nothing runs any trap) the next
// night removes the snapshot and the worktree entry, and nothing else.
func TestNightlyScriptRemovesStaleSnapshots(t *testing.T) {
	e := newNightlyEnv(t)
	mark := filepath.Join(t.TempDir(), "mark")
	cmd := e.cmd("sleep")
	cmd.Env = append(cmd.Env, "STUB_MARK="+mark)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, mark)
	pidb, _ := os.ReadFile(mark)
	// The script and its job, with no trap running (a power cut). The job has a process group of its own (setsid),
	// so it is killed by its pid; as long as it ran, it would hold the night's lock, and rightly so.
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	syscall.Kill(mustAtoi(string(pidb)), syscall.SIGKILL)
	cmd.Wait()
	for i := 0; !processGone(string(pidb)) && i < 100; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(e.worktrees(t), "/src") {
		t.Fatalf("the kill -9 left no snapshot to clean:\n%s", e.worktrees(t))
	}
	// something of the user's that is not a snapshot must stay
	keep := filepath.Join(e.results, "20200101-000000", "src")
	writeTree(t, keep, map[string]string{"keep.txt": "mine"})
	// a src folder with a .git file that is not a worktree of this repository: never touched, not even removed
	foreign := filepath.Join(e.results, "20200202-000000", "src")
	writeTree(t, foreign, map[string]string{".git": "gitdir: /nowhere/else", "x.txt": "foreign"})
	other := filepath.Join(e.results, "notes.txt")
	os.WriteFile(other, []byte("notes"), 0o644)

	if out, err := e.cmd().CombinedOutput(); err != nil {
		t.Fatalf("the next night: %v\n%s\nhistory:\n%s", err, out, e.history(t))
	}
	if strings.Count(e.worktrees(t), "\n") != 1 {
		t.Errorf("the stale worktree entry is still there:\n%s", e.worktrees(t))
	}
	for _, n := range e.nights(t) {
		if n == "20200101-000000" || n == "20200202-000000" {
			continue
		}
		if _, err := os.Stat(filepath.Join(e.results, n, "src")); err == nil {
			t.Errorf("night %s kept its snapshot", n)
		}
	}
	if _, err := os.Stat(filepath.Join(foreign, "x.txt")); err != nil {
		t.Error("the script removed a src folder that git does not know as a worktree of this repository")
	}
	if h := e.history(t); !strings.Contains(h, "previous night killed hard, snapshot removed") {
		t.Errorf("history.txt does not say that the killed night was found: %q", h)
	}
	if _, err := os.Stat(filepath.Join(keep, "keep.txt")); err != nil {
		t.Error("the script removed a src folder that is not a git worktree")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("the script removed a file that is not a snapshot")
	}
}

func TestNightlyScriptSecondNightSkipsAndLeavesNoFolder(t *testing.T) {
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("no flock")
	}
	e := newNightlyEnv(t)
	if err := os.MkdirAll(e.results, 0o755); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(e.results, "lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if got := exitCode(e.cmd().Run()); got != 2 {
		t.Errorf("exit %d, want 2", got)
	}
	if n := e.nights(t); len(n) != 0 {
		t.Errorf("the night that did not run left folders: %v", n)
	}
	if h := e.history(t); !strings.Contains(h, "skipped: another night is running") {
		t.Errorf("history.txt = %q", h)
	}
	if strings.Count(e.worktrees(t), "\n") != 1 {
		t.Errorf("worktrees:\n%s", e.worktrees(t))
	}
}

// ---- the session and its signals (Linux: signals are sent to the test process itself)

func openTestSession(t *testing.T) (*session, chan int) {
	t.Helper()
	self, _ := os.Executable()
	var stderr bytes.Buffer
	cf := commonFlags{gocpp: self, work: t.TempDir(), jobs: 1}
	s, err := cf.open(&stderr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.close)
	exits := make(chan int, 4)
	s.exit = func(c int) { exits <- c }
	return s, exits
}

func TestInterruptStopsTheDeepStageCommands(t *testing.T) {
	s, _ := openTestSession(t)
	cmd := exec.CommandContext(context.Background(), "sleep", "60")
	inGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s.deep.track(cmd, true)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	s.interrupt()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		killGroup(cmd)
		t.Fatal("interrupt left the make command of a deep stage running")
	}
	// a command that starts after the interrupt is killed at once
	late := exec.CommandContext(context.Background(), "sleep", "60")
	inGroup(late)
	if err := late.Start(); err != nil {
		t.Fatal(err)
	}
	s.deep.track(late, true)
	lateDone := make(chan error, 1)
	go func() { lateDone <- late.Wait() }()
	select {
	case <-lateDone:
	case <-time.After(30 * time.Second):
		killGroup(late)
		t.Fatal("a command that started after the interrupt was not stopped")
	}
}

// TestSignalsInterruptTheSession sends the signals to the test process itself: with the handler of
// handleSignals they run interrupt (here: a stubbed exit); without it SIGTERM would end the test.
func TestSignalsInterruptTheSession(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			s, exits := openTestSession(t)
			work := s.work
			if err := syscall.Kill(os.Getpid(), sig); err != nil {
				t.Fatal(err)
			}
			select {
			case code := <-exits:
				if code != 130 {
					t.Errorf("exit code %d, want 130", code)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("the signal did not reach the handler")
			}
			if _, err := os.Stat(work); err != nil {
				t.Logf("(the caller's work folder %s is kept: %v)", work, err)
			}
		})
	}
}

func TestNightlyScriptRunsTheJobInTheSnapshot(t *testing.T) {
	e := newNightlyEnv(t)
	cwd := filepath.Join(t.TempDir(), "cwd")
	cmd := e.cmd()
	cmd.Env = append(cmd.Env, "STUB_CWD="+cwd)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(cwd)
	if !strings.HasPrefix(string(b), e.results+"/") || !strings.HasSuffix(string(b), "/src") {
		t.Errorf("the job ran in %q, want the snapshot under %s", b, e.results)
	}
	// uncommitted work is not tested; SANITIZE_LIVE=1 tests the live tree and records that it is dirty
	os.WriteFile(filepath.Join(e.repo, "dirty.txt"), []byte("x"), 0o644)
	live := e.cmd()
	live.Env = append(live.Env, "STUB_CWD="+cwd, "SANITIZE_LIVE=1")
	if err := live.Run(); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(cwd)
	if string(b) != e.repo {
		t.Errorf("SANITIZE_LIVE: the job ran in %q, want %q", b, e.repo)
	}
	if h := e.history(t); !strings.Contains(h, "(tree has uncommitted changes)") {
		t.Errorf("history.txt does not say the live tree was dirty: %q", h)
	}
}

func TestNightlyScriptToolThatDoesNotBuild(t *testing.T) {
	e := newNightlyEnvWith(t, "package main\n\nfunc main() { this does not compile }\n")
	cmd := e.cmd()
	out, err := cmd.CombinedOutput()
	if got := exitCode(err); got != 2 {
		t.Errorf("exit %d, want 2\n%s", got, out)
	}
	if h := e.history(t); !strings.Contains(h, " exit 2 ") || !strings.Contains(h, "could not build tools/sanitize") {
		t.Errorf("history.txt = %q", h)
	}
	if strings.Count(e.worktrees(t), "\n") != 1 {
		t.Errorf("the snapshot is still a worktree:\n%s", e.worktrees(t))
	}
}

// A job that ignores TERM is killed after the script's grace period (10 s); the script does not wait for it.
func TestNightlyScriptKillsAJobThatIgnoresTerm(t *testing.T) {
	e := newNightlyEnv(t)
	mark := filepath.Join(t.TempDir(), "mark")
	cmd := e.cmd("sleep", "ignoreterm")
	cmd.Env = append(cmd.Env, "STUB_MARK="+mark)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, mark)
	pidb, _ := os.ReadFile(mark)
	start := time.Now()
	syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if got := exitCode(err); got != 143 {
			t.Errorf("exit %d, want 143", got)
		}
		if took := time.Since(start); took < 5*time.Second {
			t.Errorf("the script ended after %v: it did not wait for the job to stop", took)
		}
	case <-time.After(90 * time.Second):
		syscall.Kill(mustAtoi(string(pidb)), syscall.SIGKILL)
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		t.Fatal("the script waits for a job that ignores TERM: no KILL")
	}
	for i := 0; !processGone(string(pidb)) && i < 100; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if !processGone(string(pidb)) {
		syscall.Kill(mustAtoi(string(pidb)), syscall.SIGKILL)
		t.Errorf("the job (pid %s) that ignores TERM still runs", pidb)
	}
	if strings.Count(e.worktrees(t), "\n") != 1 {
		t.Errorf("the snapshot is still a worktree:\n%s", e.worktrees(t))
	}
}

func TestNightlyScriptThroughASymlink(t *testing.T) {
	e := newNightlyEnv(t)
	link := filepath.Join(t.TempDir(), "nightly")
	if err := os.Symlink(filepath.Join(e.repo, "tools/sanitize/nightly.sh"), link); err != nil {
		t.Fatal(err)
	}
	cmd := e.cmd()
	cmd.Dir = t.TempDir() // not the repository
	cmd.Args = []string{"sh", link}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("through a symlink: %v\n%s", err, out)
	}
	if h := e.history(t); !strings.Contains(h, " exit 0 ") {
		t.Errorf("history.txt = %q", h)
	}
}
