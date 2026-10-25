package tests

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// A job with files (a Wine program: explorer.exe, started again after the prefix's one ended, inherits
// the program's pipes) does not wait for a helper process that keeps its output open, and still gets
// the output and the exit code; with pipes the same command is the problem the files avoid.
func TestJobFilesIgnoreHelpers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	c := &config{timeout: time.Minute}
	script := "sleep 30 & echo out; echo err >&2; exit 0"
	j := job{dir: t.TempDir(), name: "sh", args: []string{"-c", script}, show: "sh -c " + script}

	pipes := c.run(t, j)
	if pipes.problem == "" {
		t.Errorf("with pipes the helper's open output is no problem: %+v (the test does not show what files avoid)", pipes)
	}

	j.files = true
	j.helpers = map[string]bool{"sleep": true}
	start := time.Now()
	r := c.run(t, j)
	if r.problem != "" || r.exit != 0 || string(r.stdout) != "out\n" || string(r.stderr) != "err\n" {
		t.Errorf("with files: %+v, want exit 0, out and err, no problem", r)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("with files the job took %v", d)
	}
}

// A job with files still reports a process that it started and left running, unless it is a helper:
// the files only stop the pipes from making the run wait.
func TestJobFilesReportLeftovers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	c := &config{timeout: time.Minute}
	script := "sleep 30 > /dev/null 2>&1 & echo out"
	j := job{dir: t.TempDir(), name: "sh", args: []string{"-c", script}, show: "sh -c " + script, files: true}
	r := c.run(t, j)
	if !strings.Contains(r.problem, "kept running") || !strings.Contains(r.problem, "sleep 30") {
		t.Errorf("a child left running (output redirected): problem %q, want it to name the child", r.problem)
	}
	if string(r.stdout) != "out\n" {
		t.Errorf("stdout %q, want the output anyway", r.stdout)
	}
	j.helpers = map[string]bool{"sleep": true}
	if r := c.run(t, j); r.problem != "" {
		t.Errorf("the same child as a helper: problem %q, want none", r.problem)
	}
}

// A job with files that prints without end is stopped at the output limit, as with pipes, and not
// after it has filled the disk for the whole timeout.
func TestJobFilesOutputLimit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs yes")
	}
	c := &config{timeout: time.Minute}
	j := job{dir: t.TempDir(), name: "yes", show: "yes", files: true}
	start := time.Now()
	r := c.run(t, j)
	if !strings.Contains(r.problem, "printed more than") {
		t.Errorf("problem %q, want the output limit", r.problem)
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Errorf("it took %v to stop", d)
	}
	if len(r.stdout) > maxOutput {
		t.Errorf("kept %d bytes, the limit is %d", len(r.stdout), maxOutput)
	}
}

// A zombie in the group (a child that has exited, whose parent runs on without waiting for it) is not a
// process that runs: it is not reported, though its parent (here a helper) is in the group too.
func TestJobFilesIgnoreZombies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	c := &config{timeout: time.Minute}
	// The subshell forks `true`, which exits at once, and becomes sleep, which never waits for it.
	script := "(true & exec sleep 30) > /dev/null 2>&1 & echo out"
	j := job{dir: t.TempDir(), name: "sh", args: []string{"-c", script}, show: "sh -c " + script, files: true,
		helpers: map[string]bool{"sleep": true}}
	if r := c.run(t, j); r.problem != "" {
		t.Errorf("a zombie and a helper in the group: problem %q, want none", r.problem)
	}
}
