package main

// File selection with the constraints of tags.go.
//
// go/build.Context.ImportDir refuses a Compiler other than gc or gccgo, so the
// tool does not use it: it walks a package directory and asks
// Context.MatchFile, the go/build rule for one file name (the _GOOS/_GOARCH
// suffixes, //go:build and the old // +build lines, the compiler tag, the
// build, tool and release tags). That is the same rule the gocpp loader uses
// for .gp files (internal/load/resolve.go), and it needs no go command.

import (
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// newContext returns a go/build context with every constraint set explicitly:
// nothing comes from build.Default or from the toolchain that built the tool.
func newContext(goroot string, tags *TagSet, compiler string) *build.Context {
	return &build.Context{
		GOROOT:      goroot,
		GOOS:        tags.GOOS,
		GOARCH:      tags.GOARCH,
		Compiler:    compiler,
		CgoEnabled:  false,
		BuildTags:   slices.Clone(tags.BuildTags),
		ToolTags:    slices.Clone(tags.ToolTags),
		ReleaseTags: slices.Clone(tags.ReleaseTags),
	}
}

// DirFiles is the file selection of one package directory for one target.
type DirFiles struct {
	GoFiles  []string // selected .go files, sorted
	CgoFiles []string // selected .go files that import "C" (dropped: CGO_ENABLED=0)
	SFiles   []string // selected .s files (never used by the GC engine)
	Excluded []ExcludedFile
	Tests    int
}

// An ExcludedFile is a .go file that the constraints leave out.
type ExcludedFile struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// selectDir selects the files of dir under ctxt.
func selectDir(ctxt *build.Context, dir string) (*DirFiles, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := &DirFiles{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		switch {
		case strings.HasSuffix(name, "_test.go"):
			if ok, err := ctxt.MatchFile(dir, name); err == nil && ok {
				out.Tests++
			}
		case strings.HasSuffix(name, ".go"):
			ok, err := ctxt.MatchFile(dir, name)
			if err != nil {
				out.Excluded = append(out.Excluded, ExcludedFile{Name: name, Reason: err.Error()})
				continue
			}
			if !ok {
				out.Excluded = append(out.Excluded, ExcludedFile{Name: name, Reason: "build constraints"})
				continue
			}
			cgo, err := importsC(dir, name)
			if err != nil {
				return nil, err
			}
			if cgo {
				out.CgoFiles = append(out.CgoFiles, name)
			} else {
				out.GoFiles = append(out.GoFiles, name)
			}
		case strings.HasSuffix(name, ".s"):
			if ok, err := ctxt.MatchFile(dir, name); err == nil && ok {
				out.SFiles = append(out.SFiles, name)
			}
		}
	}
	sort.Strings(out.GoFiles)
	sort.Strings(out.CgoFiles)
	sort.Strings(out.SFiles)
	sort.Slice(out.Excluded, func(i, j int) bool { return out.Excluded[i].Name < out.Excluded[j].Name })
	return out, nil
}

// cgoSelected returns the files that a cgo-enabled build would compile for
// this target: the CgoFiles of the same context with CgoEnabled=true. The dry
// run drops them (CGO_ENABLED=0) and reports them, as Phase 16 README 2.7 says.
func cgoSelected(goroot string, tags *TagSet, dir string) ([]string, error) {
	ctxt := newContext(goroot, tags, "gocpp")
	ctxt.CgoEnabled = true
	files, err := selectDir(ctxt, dir)
	if err != nil {
		return nil, err
	}
	return files.CgoFiles, nil
}

// importsC reports whether the file imports "C".
func importsC(dir, name string) (bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
	if err != nil {
		return false, fmt.Errorf("%s: %v", filepath.Join(dir, name), err)
	}
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` {
			return true, nil
		}
	}
	return false, nil
}

// imports returns the import paths of the named files of dir.
func imports(dir string, files []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, name := range files {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", filepath.Join(dir, name), err)
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, err
			}
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// resolveImport returns the package path of imp in the GOROOT source tree when
// the package at from imports it, or "" when it is not a standard package
// (C, unsafe and builtin are not packages). The vendored copies under
// src/vendor are packages of their own ("vendor/golang.org/x/net/..."), as
// Phase 16 README 2.3 rule 4 says.
func resolveImport(goroot, from, imp string) string {
	switch imp {
	case "C", "unsafe", "builtin":
		return ""
	}
	exists := func(p string) bool {
		_, err := os.Stat(filepath.Join(goroot, "src", filepath.FromSlash(p)))
		return err == nil
	}
	if strings.HasPrefix(imp, "vendor/") {
		if exists(imp) {
			return imp
		}
		return ""
	}
	inVendor := from == "vendor" || strings.HasPrefix(from, "vendor/") || strings.Contains(from, "/vendor/")
	if inVendor {
		if v := path.Join("vendor", imp); exists(v) {
			return v
		}
		if exists(imp) {
			return imp
		}
		return ""
	}
	if exists(imp) {
		return imp
	}
	if v := path.Join("vendor", imp); exists(v) {
		return v
	}
	return ""
}

// closure walks the imports of roots and returns the packages that have Go
// files for the target, sorted, and the set of their paths.
func closure(ctxt *build.Context, goroot string, roots []string) ([]string, map[string]bool, error) {
	seen := map[string]bool{}
	var out []string
	queue := append([]string{}, roots...)
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if seen[pkg] {
			continue
		}
		seen[pkg] = true
		dir := filepath.Join(goroot, "src", filepath.FromSlash(pkg))
		files, err := selectDir(ctxt, dir)
		if err != nil {
			return nil, nil, fmt.Errorf("package %s: %v", pkg, err)
		}
		if len(files.GoFiles) == 0 {
			// Not buildable for this target (a _windows.go-only package on
			// linux, for example): it is not part of the closure.
			continue
		}
		out = append(out, pkg)
		imps, err := imports(dir, files.GoFiles)
		if err != nil {
			return nil, nil, err
		}
		for _, imp := range imps {
			if r := resolveImport(goroot, pkg, imp); r != "" && !seen[r] {
				queue = append(queue, r)
			}
		}
	}
	sort.Strings(out)
	set := make(map[string]bool, len(out))
	for _, p := range out {
		set[p] = true
	}
	return out, set, nil
}

// droppedAdded returns the .go files that the gocpp tag drops and adds
// compared with the gc tag, for one package directory.
func droppedAdded(goroot string, tags *TagSet, dir string) (dropped, added []string, err error) {
	gocpp, err := selectDir(newContext(goroot, tags, "gocpp"), dir)
	if err != nil {
		return nil, nil, err
	}
	gc, err := selectDir(newContext(goroot, tags, "gc"), dir)
	if err != nil {
		return nil, nil, err
	}
	selected := func(f *DirFiles) map[string]bool {
		m := map[string]bool{}
		for _, n := range f.GoFiles {
			m[n] = true
		}
		for _, n := range f.CgoFiles {
			m[n] = true
		}
		return m
	}
	a, b := selected(gocpp), selected(gc)
	for n := range b {
		if !a[n] {
			dropped = append(dropped, n)
		}
	}
	for n := range a {
		if !b[n] {
			added = append(added, n)
		}
	}
	sort.Strings(dropped)
	sort.Strings(added)
	return dropped, added, nil
}
