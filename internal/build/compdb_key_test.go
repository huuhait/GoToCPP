package build

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The cache knows a compile_commands.json by its folder with the links resolved: the same file
// reached through a linked folder has one record, and a file that is itself a link has the record
// of the link (not of what it points to).
func TestCompDBKeyResolvesFolderLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links")
	}
	real := t.TempDir()
	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(real, linked); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	a := compDBRecord(cache, filepath.Join(real, "compile_commands.json"))
	b := compDBRecord(cache, filepath.Join(linked, "compile_commands.json"))
	if a != b {
		t.Errorf("one file, two records: %s and %s", a, b)
	}
	file := filepath.Join(real, "other.json")
	if err := os.Symlink(filepath.Join(real, "compile_commands.json"), file); err != nil {
		t.Fatal(err)
	}
	if compDBRecord(cache, file) == a {
		t.Errorf("a link to the file has the record of the file")
	}
}
