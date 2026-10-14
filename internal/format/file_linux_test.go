package format

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// TestBackupFileTooBig checks that backupFile removes a backup that it
// couldn't write to the end. A low limit on the size of files
// (RLIMIT_FSIZE, as with ulimit -f) makes the write fail with EFBIG. The
// SIGXFSZ signal that comes with it doesn't stop a Go program: the Go
// runtime ignores it. The limit is for the whole process, so the test sets
// it in a new run of the test binary, which runs only this test.
func TestBackupFileTooBig(t *testing.T) {
	const env = "GOCPP_TEST_BACKUP_DIR"
	if dir := os.Getenv(env); dir != "" {
		// The new run of the test binary.
		var limit syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		if limit.Max < 1024 {
			t.Skipf("the file size limit is %d bytes already", limit.Max)
		}
		old := limit
		limit.Cur = 1024
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		name, err := backupFile(filepath.Join(dir, "x.gp"), make([]byte, 4096), 0o600)
		// Set the old limit back at once: the test binary writes more files
		// before it exits, such as the coverage data of go test -cover.
		// Raising the soft limit up to the hard one needs no rights.
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(err, syscall.EFBIG) {
			t.Errorf("backupFile of 4096 bytes with a limit of 1024 bytes: %q, %v; want the error EFBIG", name, err)
		}
		return
	}

	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestBackupFileTooBig$", "-test.v")
	cmd.Env = append(os.Environ(), env+"="+dir)
	out, err := cmd.CombinedOutput()
	switch {
	case err != nil:
		t.Fatalf("the test in a new process: %v\n%s", err, out)
	case bytes.Contains(out, []byte("--- SKIP: TestBackupFileTooBig")):
		t.Skipf("the test in a new process skipped:\n%s", out)
	case !bytes.Contains(out, []byte("--- PASS: TestBackupFileTooBig")):
		t.Fatalf("the test in a new process didn't run:\n%s", out)
	}
	if names := dirNames(t, dir); len(names) != 0 {
		t.Errorf("after a backup that failed, the folder holds %q, want nothing", names)
	}
}
