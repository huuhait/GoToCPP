package filelock

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The lock is between open files, also inside one process (flock and LockFileEx both treat two opens
// of a file as two holders), and a second process cannot take it: that part runs on Windows too,
// on the real PC (tests/windows_native_test.go runs this package's tests there).

func TestTryLock(t *testing.T) {
	name := filepath.Join(t.TempDir(), "lock")
	a, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if ok, err := TryLock(a); !ok || err != nil {
		t.Fatalf("first lock = %v, %v", ok, err)
	}
	if ok, err := TryLock(b); ok || err != nil {
		t.Fatalf("second lock = %v, %v; want busy", ok, err)
	}
	Unlock(a)
	if ok, err := TryLock(b); !ok || err != nil {
		t.Fatalf("lock after unlock = %v, %v", ok, err)
	}
	Unlock(b)
}

// TestLockHolder is the child of TestLockAcrossProcesses: it takes the lock and holds it until its
// standard input closes.
func TestLockHolder(t *testing.T) {
	name := os.Getenv("GOCPP_FILELOCK_HOLD")
	if name == "" {
		t.Skip("a helper of TestLockAcrossProcesses")
	}
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := TryLock(f); !ok || err != nil {
		os.Stdout.WriteString("busy\n")
		return
	}
	os.Stdout.WriteString("locked\n")
	buf := make([]byte, 1)
	os.Stdin.Read(buf)
}

func TestLockAcrossProcesses(t *testing.T) {
	name := filepath.Join(t.TempDir(), "lock")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHolder$")
	cmd.Env = append(os.Environ(), "GOCPP_FILELOCK_HOLD="+name)
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { in.Close(); cmd.Wait() })
	line := make([]byte, 7)
	n, _ := out.Read(line)
	if string(line[:n]) != "locked\n" {
		t.Fatalf("the child says %q", line[:n])
	}
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if ok, err := TryLock(f); ok || err != nil {
		t.Fatalf("lock while the child holds it = %v, %v", ok, err)
	}
	// The child ends (its handle closes, the system releases the lock): the lock is free, also
	// though the child never called Unlock.
	in.Close()
	cmd.Wait()
	if ok, err := TryLock(f); !ok || err != nil {
		t.Fatalf("lock after the holder ended = %v, %v", ok, err)
	}
}
