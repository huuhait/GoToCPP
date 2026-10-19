//go:build unix

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ^C while the preprocessor of gocpp bindgen runs a slow compiler: the command says it was
// interrupted, ends with the signal's status, and the compiler is gone.
func TestBindgenInterrupt(t *testing.T) {
	inHeaders(t)
	requireClang(t)
	real, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("no clang++")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "slow.pid")
	fake := filepath.Join(dir, "slow-clang++")
	script := "#!/bin/sh\nfor a in \"$@\"; do\n\tcase \"$a\" in\n\t-dD) echo $$ > '" + pidFile + "'; exec sleep 60;;\n\tesac\ndone\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_CLANG", fake)
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- run([]string{"bindgen", "c_basic.h"}, strings.NewReader(""), &stdout, &stderr) }()
	var pid string
	for i := 0; i < 600 && pid == ""; i++ {
		if b, err := os.ReadFile(pidFile); err == nil && strings.TrimSpace(string(b)) != "" {
			pid = strings.TrimSpace(string(b))
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if pid == "" {
		t.Fatal("the slow compiler did not start")
	}
	syscall.Kill(os.Getpid(), syscall.SIGINT)
	select {
	case code := <-done:
		defer exitSignal.Store(nil)
		if code != 128+int(syscall.SIGINT) || stdout.Len() != 0 || stderr.String() != "gocpp bindgen: interrupted\n" {
			t.Errorf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
		}
	case <-time.After(60 * time.Second):
		t.Fatal("bindgen did not stop")
	}
	// the compiler (a process of its own group) is gone
	for i := 0; i < 100; i++ {
		if err := exec.Command("kill", "-0", pid).Run(); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("the compiler (pid %s) is still running", pid)
}
