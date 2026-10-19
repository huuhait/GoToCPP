package cache

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestRetryReadSharingViolation: another program has the file open without sharing (a formatter or a compiler that writes
// it, a virus scanner that looks at it): the read fails with ERROR_SHARING_VIOLATION for a moment, and the server reads
// again instead of making the failure a fact about the file (Phase 14 step 14.5, ADR-096).
func TestRetryReadSharingViolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.gp")
	if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_WRITE, 0 /* no sharing: a writer that excludes readers */, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(path); err == nil {
		t.Fatal("the file can be read while it is open without sharing: the test makes no violation")
	}
	go func() {
		time.Sleep(60 * time.Millisecond)
		syscall.CloseHandle(h)
	}()
	src, err := retryRead(func() ([]byte, error) { return os.ReadFile(path) })
	if err != nil || string(src) != "package a\n" {
		t.Errorf("retryRead = %q, %v", src, err)
	}
}
