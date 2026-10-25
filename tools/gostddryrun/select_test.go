package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func fakeContext(t *testing.T, goos, goarch string) (*TagSet, string) {
	t.Helper()
	d, err := readToolDefaults(fakeGOROOT)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := d.tagSet(goos, goarch, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tags, filepath.Join(fakeGOROOT, "src", "example")
}

func TestSelectDirLinux(t *testing.T) {
	tags, dir := fakeContext(t, "linux", "amd64")
	files, err := selectDir(newContext(fakeGOROOT, tags, "gocpp"), dir)
	if err != nil {
		t.Fatal(err)
	}
	wantGo := []string{"example.go", "gocpp_only.go", "imports.go", "linux.go", "purego_only.go", "release.go", "tooltag.go"}
	if strings.Join(files.GoFiles, ",") != strings.Join(wantGo, ",") {
		t.Errorf("GoFiles = %v, want %v", files.GoFiles, wantGo)
	}
	if strings.Join(files.SFiles, ",") != "asm.s" {
		t.Errorf("SFiles = %v, want [asm.s]", files.SFiles)
	}
	if files.Tests != 1 {
		t.Errorf("Tests = %d, want 1", files.Tests)
	}
	// cgo.go has //go:build cgo, so it is out of the gocpp context (CGO off).
	for _, name := range []string{"cgo.go", "gc_only.go", "notooltag.go", "windows.go"} {
		if !excluded(files, name) {
			t.Errorf("%s is not in the excluded list: %v", name, files.Excluded)
		}
	}
}

func TestSelectDirWindows(t *testing.T) {
	tags, dir := fakeContext(t, "windows", "amd64")
	files, err := selectDir(newContext(fakeGOROOT, tags, "gocpp"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(files.GoFiles, "windows.go") || contains(files.GoFiles, "linux.go") {
		t.Errorf("windows GoFiles = %v", files.GoFiles)
	}
}

func TestDroppedAdded(t *testing.T) {
	tags, dir := fakeContext(t, "linux", "amd64")
	dropped, added, err := droppedAdded(fakeGOROOT, tags, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(dropped, ",") != "gc_only.go" {
		t.Errorf("dropped = %v, want [gc_only.go]", dropped)
	}
	if strings.Join(added, ",") != "gocpp_only.go" {
		t.Errorf("added = %v, want [gocpp_only.go]", added)
	}
}

func TestCgoSelected(t *testing.T) {
	tags, dir := fakeContext(t, "linux", "amd64")
	cgo, err := cgoSelected(fakeGOROOT, tags, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cgo, ",") != "cgo.go" {
		t.Errorf("cgo files = %v, want [cgo.go]", cgo)
	}
}

func TestClosure(t *testing.T) {
	tags, _ := fakeContext(t, "linux", "amd64")
	pkgs, set, err := closure(newContext(fakeGOROOT, tags, "gocpp"), fakeGOROOT, []string{"example"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pkgs, ",") != "example,example/dep" {
		t.Errorf("closure = %v, want [example example/dep]", pkgs)
	}
	if !set["example/dep"] {
		t.Errorf("set = %v", set)
	}
}

func TestNoFilesUnderGocpp(t *testing.T) {
	tags, _ := fakeContext(t, "linux", "amd64")
	files, err := selectDir(newContext(fakeGOROOT, tags, "gocpp"), filepath.Join(fakeGOROOT, "src", "example", "gconly"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files.GoFiles) != 0 {
		t.Errorf("gconly GoFiles = %v, want none under gocpp", files.GoFiles)
	}
	dropped, _, err := droppedAdded(fakeGOROOT, tags, filepath.Join(fakeGOROOT, "src", "example", "gconly"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(dropped, ",") != "gc.go" {
		t.Errorf("gconly dropped = %v, want [gc.go]", dropped)
	}
}

func excluded(files *DirFiles, name string) bool {
	for _, e := range files.Excluded {
		if e.Name == name {
			return true
		}
	}
	return false
}
