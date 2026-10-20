package pclock

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The lock is between processes: TestMain turns this test binary into the holder when PCLOCK_HOLD names a file.
func TestMain(m *testing.M) {
	if path := os.Getenv("PCLOCK_HOLD"); path != "" {
		release, err := Acquire(context.Background(), Options{Path: path, Wait: time.Second})
		if err != nil {
			os.Stdout.WriteString("error " + err.Error() + "\n")
			os.Exit(1)
		}
		os.Stdout.WriteString("held\n")
		bufio.NewReader(os.Stdin).ReadString('\n') // until the parent says release (or dies)
		release()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// hold starts a process that holds the lock until stop is called.
func hold(t *testing.T, path string) (stop func()) {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "PCLOCK_HOLD="+path)
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(out).ReadString('\n')
	if strings.TrimSpace(line) != "held" {
		t.Fatalf("the holder said %q", line)
	}
	return func() { in.Write([]byte("\n")); cmd.Wait() }
}

func TestAnotherProcessExcludes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "pc.lock")
	stop := hold(t, path)
	if _, err := Acquire(context.Background(), Options{Path: path, Wait: 300 * time.Millisecond, Poll: 20 * time.Millisecond}); err == nil || !strings.Contains(err.Error(), "in use by another run") {
		t.Fatalf("a lock held by another process: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, Options{Path: path, Poll: 20 * time.Millisecond}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a lock with a context: %v", err)
	}
	got := make(chan error, 1)
	go func() {
		rel, err := Acquire(context.Background(), Options{Path: path, Wait: 10 * time.Second, Poll: 10 * time.Millisecond})
		if err == nil {
			rel()
		}
		got <- err
	}()
	time.Sleep(100 * time.Millisecond)
	stop()
	if err := <-got; err != nil {
		t.Errorf("the waiting lock after the release: %v", err)
	}
}

// Inside one process the lock is counted: two holders at once do not deadlock, and the file is free after the last release.
func TestCountedInsideOneProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pc.lock")
	a, err := Acquire(context.Background(), Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Acquire(context.Background(), Options{Path: path, Wait: time.Second})
	if err != nil {
		t.Fatalf("the second holder of one process: %v", err)
	}
	a()
	// still held by b: another process cannot take it
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "PCLOCK_HOLD="+path)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "in use by another run") {
		t.Errorf("after one release of two the lock must still be held: %q", out)
	}
	b()
	b() // a second release changes nothing
	stop := hold(t, path)
	stop()
}

func TestEnvDefaults(t *testing.T) {
	t.Setenv("GOCPP_WINDOWS_LOCK", "")
	if PathFromEnv() != DefaultPath {
		t.Error(PathFromEnv())
	}
	t.Setenv("GOCPP_WINDOWS_LOCK", "/x/y.lock")
	if PathFromEnv() != "/x/y.lock" {
		t.Error(PathFromEnv())
	}
	t.Setenv("GOCPP_WINDOWS_LOCK_WAIT", "5m")
	if waitFromEnv() != 5*time.Minute {
		t.Error(waitFromEnv())
	}
	t.Setenv("GOCPP_WINDOWS_LOCK_WAIT", "bad")
	if waitFromEnv() != 3*time.Hour {
		t.Error(waitFromEnv())
	}
}

func TestCancelledContextNeverAcquires(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 50; i++ {
		rel, err := Acquire(ctx, Options{Path: filepath.Join(t.TempDir(), "pc.lock")})
		if err == nil {
			rel()
			t.Fatal("a cancelled context acquired the lock")
		}
	}
}
