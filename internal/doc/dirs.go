// The folder scan in this file follows dirs.go of cmd/internal/doc of Go
// 1.25.14 (Copyright 2015 The Go Authors. All rights reserved. Use of that
// source code is governed by a BSD-style license that can be found in
// internal/doc/LICENSE).
//
// gocpp: the code roots are not GOROOT and GOPATH or the output of go list
// but the embedded standard library, the main module (the folder with a
// gocpp.mod file) and the modules it requires (load.ListModules: replaced
// folders, vendor/, the module cache); the scan reads them through fs.FS;
// and it does not use a goroutine: the next folder is scanned when Next
// asks for it.

package doc

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// A root is a tree of packages: the standard library, or a module.
type root struct {
	importPath string // "" for the standard library, else the module path
	dir        string // the folder for messages and positions
	fsys       fs.FS
	inModule   bool
	std        bool
}

// A Dir is a folder that holds packages, with the import path it would have.
type dirEntry struct {
	importPath string
	rel        string // in root.fsys
	dir        string
	root       *root
}

// moduleState is what the module of the current folder gives.
type moduleState struct {
	main  *load.Module
	deps  []*load.Dependency
	roots []*root

	// unavailable are the required modules that have no folder (not
	// downloaded, a missing replacement).
	unavailable []*load.Dependency
}

// modules finds the main module and the modules it requires, once.
func (r *runner) modules() *moduleState {
	if r.mods != nil {
		return r.mods
	}
	ms := &moduleState{}
	r.mods = ms
	list := load.ListModules(load.Config{Dir: r.dir}, r.dir)
	if list.Main == nil || list.Main.Path == "" {
		return ms
	}
	ms.main = list.Main
	ms.roots = append(ms.roots, &root{importPath: list.Main.Path, dir: list.Main.Dir, fsys: load.DirFS(list.Main.Dir), inModule: true})
	// go list -m all: the main module first, then the others sorted by path.
	deps := slices.Clone(list.Deps)
	slices.SortStableFunc(deps, func(a, b *load.Dependency) int { return strings.Compare(a.Path, b.Path) })
	for _, d := range deps {
		if d.Dir == "" {
			ms.unavailable = append(ms.unavailable, d) // d.Err says why
			continue
		}
		ms.deps = append(ms.deps, d)
		ms.roots = append(ms.roots, &root{importPath: d.Path, dir: d.Dir, fsys: load.DirFS(d.Dir), inModule: true})
	}
	return ms
}

// codeRoots returns the roots to search: the standard library first, then
// the main module, then the required modules.
func (r *runner) codeRoots() []*root {
	std := &root{dir: r.stdDir, fsys: r.stdFS, std: true}
	return append([]*root{std}, r.modules().roots...)
}

// Dirs scans the roots for folders with .gp files, breadth first in lexical
// order, as go doc does. It scans a folder when Next first needs it, and
// keeps what it found.
type dirsScan struct {
	roots  []*root
	warn   io.Writer // where unreadable folders are told (nil: nowhere)
	hist   []dirEntry
	offset int

	// The scan state: the root number, the folders of this level of the
	// current root that are not scanned yet, and those of the next level.
	ri    int
	this  []string
	next  []string
	begun bool
}

func newDirsScan(roots []*root, warn io.Writer) *dirsScan {
	return &dirsScan{roots: roots, warn: warn}
}

// Reset puts the scan back at the beginning.
func (d *dirsScan) Reset() { d.offset = 0 }

// Next returns the next folder of the scan; false when the scan is done.
func (d *dirsScan) Next() (dirEntry, bool) {
	for d.offset >= len(d.hist) {
		if !d.scanOne() {
			return dirEntry{}, false
		}
	}
	e := d.hist[d.offset]
	d.offset++
	return e, true
}

// scanOne scans one folder, and reports whether the scan is not done.
func (d *dirsScan) scanOne() bool {
	for len(d.this) == 0 {
		switch {
		case len(d.next) > 0:
			d.this, d.next = d.next, nil
		case !d.begun && d.ri < len(d.roots):
			d.begun = true
			d.this = []string{"."}
		default:
			if d.begun {
				d.ri++
				d.begun = false
			}
			if d.ri >= len(d.roots) {
				return false
			}
		}
	}
	rt := d.roots[d.ri]
	rel := d.this[0]
	d.this = d.this[1:]
	entries, err := fs.ReadDir(rt.fsys, rel)
	if err != nil {
		if d.warn != nil { // like go doc, which logs it and goes on
			fmt.Fprintf(d.warn, "gocpp doc: open %s: %s\n", filepath.Join(rt.dir, filepath.FromSlash(rel)), errorText(err))
		}
		return true
	}
	hasFiles := false
	for _, ent := range entries {
		name := ent.Name()
		if !ent.IsDir() {
			if !hasFiles && strings.HasSuffix(name, ".gp") {
				hasFiles = true
			}
			continue
		}
		// The go tool ignores folders starting with ., _, or named "testdata".
		if name[0] == '.' || name[0] == '_' || name == "testdata" {
			continue
		}
		// In a module, skip vendor folders and stop at the boundary of another module.
		if rt.inModule {
			if name == "vendor" {
				continue
			}
			if fi, err := fs.Stat(rt.fsys, path.Join(rel, name, "gocpp.mod")); err == nil && !fi.IsDir() {
				continue
			}
		}
		d.next = append(d.next, path.Join(rel, name))
	}
	if hasFiles {
		importPath := rt.importPath
		if rel != "." {
			if importPath != "" {
				importPath += "/"
			}
			importPath += rel
		}
		d.hist = append(d.hist, dirEntry{importPath: importPath, rel: rel, dir: path.Join(filepath.ToSlash(rt.dir), rel), root: rt})
	}
	return true
}

// importPath finds the package with the full import path p: in the standard
// library, else in the main module or in a required module.
func (r *runner) importPath(p string) (*Package, error) {
	if filepath.IsAbs(p) {
		return r.importDir(p)
	}
	if p == "" || p == "." || !fs.ValidPath(p) || strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("invalid import path %q", p)
	}
	if hasGp(r.stdFS, p) {
		return r.env.readPackage(r.stdFS, p, path.Join(r.stdDir, p), p, true)
	}
	if stdlib.IsStd(p) {
		return nil, fmt.Errorf("package %s is not available yet in gocpp's standard library", p)
	}
	var best *root
	for _, rt := range r.modules().roots {
		if (p == rt.importPath || strings.HasPrefix(p, rt.importPath+"/")) && (best == nil || len(rt.importPath) > len(best.importPath)) {
			best = rt
		}
	}
	if best != nil {
		rel := strings.TrimPrefix(strings.TrimPrefix(p, best.importPath), "/")
		if rel == "" {
			rel = "."
		}
		if fi, err := fs.Stat(best.fsys, rel); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("package %s is not in module %s (%s)", p, best.importPath, best.dir)
		}
		return r.env.readPackage(best.fsys, rel, filepath.ToSlash(filepath.Join(best.dir, filepath.FromSlash(rel))), p, false)
	}
	if first, _, _ := strings.Cut(p, "/"); strings.Contains(first, ".") {
		for _, d := range r.modules().unavailable {
			if p == d.Path || strings.HasPrefix(p, d.Path+"/") {
				return nil, fmt.Errorf("package %s: module %s is required but not available: %s", p, d.Path, d.Err)
			}
		}
		return nil, fmt.Errorf("no required module provides package %s (add it to gocpp.mod with gocpp get)", p)
	}
	return nil, fmt.Errorf("package %s is not in std", p)
}

// hasGp reports whether the folder rel of fsys holds a .gp file.
func hasGp(fsys fs.FS, rel string) bool {
	entries, err := fs.ReadDir(fsys, rel)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return !e.IsDir() && strings.HasSuffix(e.Name(), ".gp") })
}

// importDir finds the package in the folder dir (build.ImportDir). Its import
// path comes from the module that holds the folder, "." if there is none.
func (r *runner) importDir(dir string) (*Package, error) {
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(r.dir, dir)
	}
	dir = filepath.Clean(dir)
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot find package in %s: %v", dir, errorText(err))
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", dir)
	}
	importPath := "."
	bestLen := -1
	for _, rt := range r.modules().roots {
		rel, err := filepath.Rel(rt.dir, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || len(rt.dir) <= bestLen {
			continue
		}
		bestLen = len(rt.dir)
		importPath = rt.importPath
		if rel != "." {
			importPath += "/" + filepath.ToSlash(rel)
		}
	}
	return r.env.readPackage(load.DirFS(dir), ".", filepath.ToSlash(dir), importPath, false)
}
