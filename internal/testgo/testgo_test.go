package testgo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMinor(t *testing.T) {
	for version, want := range map[string]string{
		"go1.25.14":             "1.25",
		"go1.25":                "1.25",
		"go1.27.1\n":            "1.27",
		"go1.26rc1":             "1.26rc1",
		"go1.25.0 X:nocoverage": "1.25",
		"1.25.0":                "",
		"go1":                   "",
		"":                      "",
	} {
		if got := Minor(version); got != want {
			t.Errorf("Minor(%q) = %q, want %q", version, got, want)
		}
	}
}

func TestModuleMinor(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module m\n\ngo 1.26.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ModuleMinor(sub)
	if err != nil || got != "1.26" {
		t.Errorf("ModuleMinor = %q, %v; want 1.26", got, err)
	}
	if err := os.WriteFile(filepath.Join(sub, "go.mod"), []byte("module n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ModuleMinor(sub); err == nil {
		t.Error("a go.mod without a go line was accepted")
	}
}

func TestModuleMinorOfTheRepository(t *testing.T) {
	// The real go.mod is found from a folder of the repository.
	if got, err := ModuleMinor("."); err != nil || got == "" {
		t.Errorf("ModuleMinor(.) = %q, %v", got, err)
	}
}
