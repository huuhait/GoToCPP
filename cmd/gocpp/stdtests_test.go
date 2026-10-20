package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
)

// overlayOf returns an overlay of two in-memory trees.
func overlayOf(base, extra fstest.MapFS) *testOverlayFS {
	return &testOverlayFS{base: base, extra: extra}
}

func entryNames(entries []fs.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestOverlayReadsFileOfBase(t *testing.T) {
	overlay := overlayOf(
		fstest.MapFS{"strings/strings.gp": {Data: []byte("base")}},
		fstest.MapFS{},
	)

	data, err := fs.ReadFile(overlay, "strings/strings.gp")

	if err != nil || string(data) != "base" {
		t.Fatalf("ReadFile = %q, %v; want base", data, err)
	}
}

func TestOverlayReadsFileOfExtra(t *testing.T) {
	overlay := overlayOf(
		fstest.MapFS{},
		fstest.MapFS{"strings/strings_test.gp": {Data: []byte("extra")}},
	)

	data, err := fs.ReadFile(overlay, "strings/strings_test.gp")

	if err != nil || string(data) != "extra" {
		t.Fatalf("ReadFile = %q, %v; want extra", data, err)
	}
}

func TestOverlayExtraShadowsBaseFile(t *testing.T) {
	overlay := overlayOf(
		fstest.MapFS{"strings/a.gp": {Data: []byte("base")}},
		fstest.MapFS{"strings/a.gp": {Data: []byte("extra")}},
	)

	data, err := fs.ReadFile(overlay, "strings/a.gp")

	if err != nil || string(data) != "extra" {
		t.Fatalf("ReadFile = %q, %v; want extra", data, err)
	}
}

func TestOverlayOpenOfMissingFileFails(t *testing.T) {
	overlay := overlayOf(fstest.MapFS{}, fstest.MapFS{})

	_, err := overlay.Open("strings/none.gp")

	if !os.IsNotExist(err) {
		t.Fatalf("Open = %v; want a not-exist error", err)
	}
}

func TestOverlayOpenOfFolderComesFromBase(t *testing.T) {
	overlay := overlayOf(
		fstest.MapFS{"strings/a.gp": {Data: []byte("base")}},
		fstest.MapFS{"strings/b_test.gp": {Data: []byte("extra")}},
	)

	file, err := overlay.Open("strings")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()

	if err != nil || !info.IsDir() {
		t.Fatalf("Stat = %v, %v; want a folder", info, err)
	}
}

func TestOverlayListsFilesOfBothTreesSorted(t *testing.T) {
	overlay := overlayOf(
		fstest.MapFS{"strings/b.gp": {}, "strings/d.gp": {}},
		fstest.MapFS{"strings/a_test.gp": {}, "strings/c_test.gp": {}},
	)

	entries, err := overlay.ReadDir("strings")

	want := []string{"a_test.gp", "b.gp", "c_test.gp", "d.gp"}
	if got := entryNames(entries); err != nil || !slices.Equal(got, want) {
		t.Fatalf("ReadDir = %v, %v; want %v", got, err, want)
	}
}

func TestOverlayListsFileOfBothTreesOnce(t *testing.T) {
	overlay := overlayOf(
		fstest.MapFS{"strings/a.gp": {}},
		fstest.MapFS{"strings/a.gp": {}},
	)

	entries, err := overlay.ReadDir("strings")

	if got := entryNames(entries); err != nil || !slices.Equal(got, []string{"a.gp"}) {
		t.Fatalf("ReadDir = %v, %v; want one a.gp", got, err)
	}
}

func TestOverlayListsFolderThatOnlyExtraHas(t *testing.T) {
	overlay := overlayOf(
		fstest.MapFS{},
		fstest.MapFS{"strings/a_test.gp": {}},
	)

	entries, err := overlay.ReadDir("strings")

	if got := entryNames(entries); err != nil || !slices.Equal(got, []string{"a_test.gp"}) {
		t.Fatalf("ReadDir = %v, %v; want a_test.gp", got, err)
	}
}

func TestOverlayListsFolderThatOnlyBaseHas(t *testing.T) {
	overlay := overlayOf(
		fstest.MapFS{"strings/a.gp": {}},
		fstest.MapFS{},
	)

	entries, err := overlay.ReadDir("strings")

	if got := entryNames(entries); err != nil || !slices.Equal(got, []string{"a.gp"}) {
		t.Fatalf("ReadDir = %v, %v; want a.gp", got, err)
	}
}

func TestOverlayListingOfMissingFolderFails(t *testing.T) {
	overlay := overlayOf(fstest.MapFS{}, fstest.MapFS{})

	_, err := overlay.ReadDir("nothing")

	if !os.IsNotExist(err) {
		t.Fatalf("ReadDir = %v; want a not-exist error", err)
	}
}

func TestStdlibFSWithoutVariableIsEmbeddedLibrary(t *testing.T) {
	t.Setenv(stdTestsEnv, "")

	if _, ok := stdlibFS().(*testOverlayFS); ok {
		t.Fatal("stdlibFS is an overlay without " + stdTestsEnv)
	}
}

func TestStdlibFSWithVariableIsOverlay(t *testing.T) {
	t.Setenv(stdTestsEnv, t.TempDir())

	if _, ok := stdlibFS().(*testOverlayFS); !ok {
		t.Fatal("stdlibFS is not an overlay with " + stdTestsEnv)
	}
}

func TestRunDirOfPackageWithFolderIsThatFolder(t *testing.T) {
	pkg := t.TempDir()
	t.Setenv(stdTestsEnv, t.TempDir())

	if got := testRunDir(pkg, "strings"); got != pkg {
		t.Fatalf("testRunDir = %q; want %q", got, pkg)
	}
}

func TestRunDirOfStandardPackageIsFolderOfItsTests(t *testing.T) {
	tests := t.TempDir()
	own := filepath.Join(tests, "encoding", "json")
	if err := os.MkdirAll(own, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(stdTestsEnv, tests)

	if got := testRunDir(filepath.Join(tests, "no-such-folder"), "encoding/json"); got != own {
		t.Fatalf("testRunDir = %q; want %q", got, own)
	}
}

func TestRunDirWithoutAnyFolderIsEmpty(t *testing.T) {
	t.Setenv(stdTestsEnv, t.TempDir())

	if got := testRunDir(filepath.Join(t.TempDir(), "none"), "strings"); got != "" {
		t.Fatalf("testRunDir = %q; want empty", got)
	}
}

func TestRunDirWithoutVariableIsEmpty(t *testing.T) {
	t.Setenv(stdTestsEnv, "")

	if got := testRunDir(filepath.Join(t.TempDir(), "none"), "strings"); got != "" {
		t.Fatalf("testRunDir = %q; want empty", got)
	}
}
