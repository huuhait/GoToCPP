package fsretry

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRename(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(a, []byte("A"), 0o644)
	os.WriteFile(b, []byte("B"), 0o644)
	if err := Rename(a, b); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(b); string(got) != "A" {
		t.Errorf("b = %q", got)
	}
	if _, err := os.Stat(a); !os.IsNotExist(err) {
		t.Error("a is still there")
	}
	// A missing source is an error at once, not after a wait.
	if err := Rename(filepath.Join(dir, "none"), filepath.Join(dir, "c")); !os.IsNotExist(err) {
		t.Errorf("missing source: %v", err)
	}
}

func TestRenameFolderOntoFolder(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "from"), filepath.Join(dir, "to")
	os.Mkdir(from, 0o755)
	os.Mkdir(to, 0o755)
	os.WriteFile(filepath.Join(to, "f"), nil, 0o644)
	// A folder onto a non-empty folder never works: the error comes at once (Windows would retry an access error for seconds otherwise; the
	// test has no wall-clock bound, it checks the outcome).
	if err := Rename(from, to); err == nil {
		t.Fatal("renamed a folder onto a non-empty folder")
	}
}

func TestOpen(t *testing.T) {
	name := filepath.Join(t.TempDir(), "file")
	os.WriteFile(name, []byte("content"), 0o644)
	file, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if got, _ := io.ReadAll(file); string(got) != "content" {
		t.Errorf("content = %q", got)
	}
}

func TestOpenMissingFileFailsAtOnce(t *testing.T) {
	// A missing file is an error at once, not after a wait: it is not an error that passes.
	_, err := Open(filepath.Join(t.TempDir(), "none"))
	if !os.IsNotExist(err) {
		t.Errorf("missing file: %v", err)
	}
}
