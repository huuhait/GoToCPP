//go:build unix

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestCheckProcess is not a test: TestCheckInterrupt runs the test binary
// with GOCPP_CHECK_PROCESS=1 to get gocpp check as a process of its own.
func TestCheckProcess(t *testing.T) {
	if os.Getenv("GOCPP_CHECK_PROCESS") != "1" {
		t.Skip("only run by TestCheckInterrupt")
	}
	exit(run([]string{"check", "."}, strings.NewReader(""), os.Stdout, os.Stderr)) // like main
}

// TestCheckInterrupt checks ^C while the namespace probe runs a slow
// compiler: gocpp check prints nothing and dies of SIGINT (on Linux; with
// exit status 130 elsewhere), like gocpp build and run, and the compiler
// is gone.
func TestCheckInterrupt(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "probe.pid")
	// A fake g++: what LookCXX asks, and a probe that waits.
	fake := filepath.Join(dir, "slow-c++")
	script := `#!/bin/sh
for a in "$@"; do
	case "$a" in
	--version) echo "slow-c++ 1.0"; exit 0;;
	-dM) echo "#define __GNUC__ 99"; exit 0;;
	-fsyntax-only) echo $$ > '` + pidFile + `'; exec sleep 60;;
	esac
done
exit 3
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// A "text file busy" start (golang/go#22315) is retried by gocpp
	// itself (build.Start).
	pkg := filepath.Join(dir, "m")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\nimport \"<vector>\"\n\nfunc main() { _ = std.vector[int]{} }\n",
	} {
		if err := os.WriteFile(filepath.Join(pkg, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckProcess$")
	cmd.Dir = pkg
	cmd.Env = append(os.Environ(), "GOCPP_CHECK_PROCESS=1", "GOCPP_CXX="+fake, "GOOS=", "GOARCH=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()

	var pid int
	for deadline := time.Now().Add(30 * time.Second); pid == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the probe didn't start; stderr:\n%s", stderr.String())
		}
		if data, err := os.ReadFile(pidFile); err == nil && bytes.HasSuffix(data, []byte("\n")) {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	checkSignaled(t, cmd, syscall.SIGINT)
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("after ^C: stdout %q, stderr %q; want no output", stdout.String(), stderr.String())
	}
	// The compiler (sleep, after exec) is gone: killed and waited for.
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		syscall.Kill(pid, syscall.SIGKILL)
		t.Errorf("the compiler process %d is still there after gocpp check exited (kill -0: %v)", pid, err)
	}
}
