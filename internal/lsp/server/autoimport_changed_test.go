package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// TestIndexChanged: a changed .gp file makes the module index old only when
// the package name of its folder is not what the index has, in the module
// that the walk would find the file in.
func TestIndexChanged(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) string {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write("a/gocpp.mod", "module example.com/a\n")
	write("a/p/p.gp", "package p\n")
	write("a/testdata/t/t.gp", "package t\n")
	write("a/_hidden/h.gp", "package h\n")
	write("a/nested/gocpp.mod", "module example.com/nested\n")
	write("a/nested/n.gp", "package n\n")
	write("b/gocpp.mod", "module example.com/b\n")
	write("b/q/q.gp", "package q\n")
	ix := newImportIndex(context.Background(), nil)
	modA := &load.Module{Path: "example.com/a", Dir: filepath.Join(root, "a")}
	modB := &load.Module{Path: "example.com/b", Dir: filepath.Join(root, "b")}
	for _, m := range []*load.Module{modA, modB} {
		if _, err := ix.moduleWait(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	}
	changed := func(rel string, typ protocol.FileChangeType) bool {
		before := ix.generation()
		ix.changed([]protocol.FileEvent{{URI: protocol.URIFromPath(filepath.Join(root, filepath.FromSlash(rel))), Type: typ}})
		return ix.generation() != before
	}
	for _, rel := range []string{"a/p/p.gp", "b/q/q.gp"} {
		if changed(rel, protocol.Changed) {
			t.Errorf("an unchanged save of %s made the indexes old", rel)
		}
	}
	// Folders and files that the walk skips.
	write("a/testdata/t/t.gp", "package other\n")
	write("a/_hidden/h.gp", "package other\n")
	write("a/nested/n.gp", "package other\n")
	for _, rel := range []string{"a/testdata/t/t.gp", "a/_hidden/h.gp", "a/nested/n.gp"} {
		if changed(rel, protocol.Changed) {
			t.Errorf("a change of %s, which the walk does not look at, made the indexes old", rel)
		}
	}
	// A package clause that is broken, then fixed.
	write("a/p/p.gp", "pckage p\n")
	if !changed("a/p/p.gp", protocol.Changed) {
		t.Error("a broken package clause did not make the index old")
	}
	ix.moduleWait(context.Background(), modA)
	write("a/p/p.gp", "package p\n")
	if !changed("a/p/p.gp", protocol.Changed) {
		t.Error("a fixed package clause did not make the index old")
	}
	ix.moduleWait(context.Background(), modA)
	if changed("a/p/p.gp", protocol.Changed) {
		t.Error("the save after the fix made the index old")
	}
	// A module whose index is being made: a change of a file in it makes it
	// again (the walk may have read the file), a file of another module does not.
	ix.mu.Lock()
	m := ix.mods[modA.Dir]
	m.done = make(chan struct{}) // as if the walk were still running
	ix.mu.Unlock()
	if !changed("a/p/p.gp", protocol.Changed) {
		t.Error("a change during the walk did not make the index old")
	}
	if changed("b/q/q.gp", protocol.Changed) {
		t.Error("a file of another module during the walk made the index old")
	}
}
