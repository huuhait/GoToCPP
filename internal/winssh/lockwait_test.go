//go:build linux

package winssh

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/pclock"
)

// TestMain turns the binary into the holder of the PC lock when WINSSH_HOLD names the lock file.
func TestMain(m *testing.M) {
	if path := os.Getenv("WINSSH_HOLD"); path != "" {
		release, err := pclock.Acquire(context.Background(), pclock.Options{Path: path, Wait: time.Second})
		if err != nil {
			os.Stdout.WriteString("error " + err.Error() + "\n")
			os.Exit(1)
		}
		os.Stdout.WriteString("held\n")
		bufio.NewReader(os.Stdin).ReadString('\n')
		release()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// A run that waits for the PC lock gives up at once when the harness is stopped (Ctrl-C), with a clear error.
func TestNewStopsWhileWaitingForTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pc.lock")
	t.Setenv("GOCPP_WINDOWS_LOCK", path)
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "WINSSH_HOLD="+path)
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { in.Write([]byte("\n")); cmd.Wait() }()
	if line, _ := bufio.NewReader(out).ReadString('\n'); strings.TrimSpace(line) != "held" {
		t.Fatalf("holder said %q", line)
	}
	saved, savedCancel := stopCtx, interrupt
	defer func() { stopCtx, interrupt = saved, savedCancel }()
	stopCtx, interrupt = context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := New("user@example.invalid")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("New returned before the stop: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	start := time.Now()
	Interrupt()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "stopped while waiting for the Windows machine") {
			t.Errorf("err = %v", err)
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("took %v", d)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("New did not give up after Interrupt")
	}
	// later calls fail at once too
	if _, err := New("user@example.invalid"); err == nil {
		t.Error("New after Interrupt succeeded")
	}
}
