//go:build windows

package winpath

import (
	"os"
	"path/filepath"
	"testing"
)

// The real functions, on the real PC (tests/windows_native_test.go runs this package's tests there).

const japanese = "\u65e5\u672c"

func TestACPSafeReal(t *testing.T) {
	if !ACPSafe("C:\\Users\\Administrator\\a b.exe") {
		t.Error("an ASCII path is not safe")
	}
	if acp, _, _ := procGetACP.Call(); acp == cpUTF8 {
		t.Skip("the system code page is UTF-8: every path is safe")
	}
	if ACPSafe("C:\\Users\\" + japanese + "\\x") {
		t.Error("a Japanese name is safe in a Western code page")
	}
	if ACPSafe("x\U0001F600") {
		t.Error("an emoji is safe in a Western code page")
	}
}

func TestShortReal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "folder "+japanese+" with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := Short(filepath.Join(dir, "missing")); ok {
		t.Error("a short name for a file that is not there")
	}
	s, ok := Short(dir)
	if !ok {
		t.Skip("this volume makes no short names")
	}
	if !ACPSafe(s) || s == dir {
		t.Errorf("Short(%q) = %q", dir, s)
	}
	if fi, err := os.Stat(s); err != nil || !fi.IsDir() {
		t.Errorf("the short name %q does not name the folder: %v", s, err)
	}
	// A file to be made in the folder: the folder's short name and the new name.
	got := Compiler(filepath.Join(dir, "new.o"))
	if want := filepath.Join(s, "new.o"); got != want {
		t.Errorf("Compiler = %q, want %q", got, want)
	}
	if f, err := os.Create(got); err != nil {
		t.Errorf("the path cannot be created: %v", err)
	} else {
		f.Close()
	}
	if _, err := os.Stat(filepath.Join(dir, "new.o")); err != nil {
		t.Error("the short path is not the same folder")
	}
}
