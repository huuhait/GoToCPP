package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// File selection (Phase 09 README, section 2.1). A package's files are the
// ones go/build picks for the target: the file name suffixes (_linux.go,
// _amd64.go) and //go:build lines decide, with GOOS and GOARCH of the
// target, the tag purego, no cgo, and the release tags up to the pinned Go.
//
// gocpp has no Go assembly, so it must take the pure-Go variant of every
// function that Go writes in assembly on some architectures. Go's packages
// choose by architecture (internal/bytealg, math, hash/crc32: "//go:build
// !amd64 && !arm64 ..." for the generic Go file) or by the tag purego
// (crypto, math/big). So the default architecture of the tool is "generic":
// a name that is none of Go's, for which every architecture-specific file
// is out and every "no assembly here" file is in, together with purego.
// A function that still has no body after that has no pure-Go variant (a
// finding, port.go).

// A Skip is a file that was not taken, and why.
type Skip struct {
	File   string
	Reason string
}

// A fileSel is a file that was taken, and for which systems.
type fileSel struct {
	Name string   // the .go name in Go's source
	For  []string // GOOS values
}

// Selection is the result of the file selection for a package.
type Selection struct {
	Pkg     string
	Name    string    // the package name
	Taken   []fileSel // sorted by name
	Skipped []Skip    // sorted by name; tests are counted, not listed
	Tests   int       // _test.go files (ported by step 9.10, not here)
	Asm     []string  // assembly files that the constraints keep: a blocker
	Imports []string  // import paths of the taken files, sorted
}

// goContext returns the go/build context of one system.
func goContext(pin Pin, goos string, opts Options) *build.Context {
	ctxt := &build.Context{
		GOOS:        goos,
		GOARCH:      opts.Arch,
		Compiler:    "gc",
		CgoEnabled:  false,
		BuildTags:   slices.Clone(opts.Tags),
		ReleaseTags: nil,
	}
	for i := 1; i <= pin.minor(); i++ {
		ctxt.ReleaseTags = append(ctxt.ReleaseTags, fmt.Sprintf("go1.%d", i))
	}
	return ctxt
}

// selectPackage returns the files of pkg for each system of opts, merged.
func selectPackage(root string, pin Pin, pkg string, opts Options) (*Selection, error) {
	dir := filepath.Join(root, "src", filepath.FromSlash(pkg))
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("package %s: no folder %s in Go %s", pkg, "src/"+pkg, pin.Version)
	}
	sel := &Selection{Pkg: pkg}
	taken := make(map[string][]string)
	skipped := make(map[string]string)
	imports := make(map[string]bool)
	var noGo []string
	for _, goos := range opts.OS {
		ctxt := goContext(pin, goos, opts)
		p, err := ctxt.ImportDir(dir, 0)
		var noGoErr *build.NoGoError
		switch {
		case errors.As(err, &noGoErr):
			noGo = append(noGo, goos)
			continue
		case err != nil:
			return nil, fmt.Errorf("package %s (GOOS=%s): %v", pkg, goos, err)
		}
		if sel.Name == "" {
			sel.Name = p.Name
		} else if sel.Name != p.Name {
			return nil, fmt.Errorf("package %s: the package name is %s for GOOS=%s and %s for another", pkg, p.Name, goos, sel.Name)
		}
		for _, f := range p.GoFiles {
			taken[f] = append(taken[f], goos)
		}
		for _, f := range p.Imports {
			imports[f] = true
		}
		sel.Asm = append(sel.Asm, p.SFiles...)
		sel.Tests = len(p.TestGoFiles) + len(p.XTestGoFiles)
		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range files {
			name := e.Name()
			if e.IsDir() || slices.Contains(p.GoFiles, name) || strings.HasSuffix(name, "_test.go") {
				continue
			}
			reason, ok := skipReason(ctxt, dir, name, pin, opts, goos)
			if !ok {
				continue
			}
			if _, seen := skipped[name]; !seen {
				skipped[name] = reason
			}
		}
	}
	if len(taken) == 0 {
		return nil, fmt.Errorf("package %s: no Go files for GOOS=%s GOARCH=%s tags=%s", pkg, strings.Join(noGo, ","), opts.Arch, strings.Join(opts.Tags, ","))
	}
	for name, goos := range taken {
		delete(skipped, name) // taken for one system: not skipped
		sel.Taken = append(sel.Taken, fileSel{Name: name, For: goos})
	}
	slices.SortFunc(sel.Taken, func(a, b fileSel) int { return strings.Compare(a.Name, b.Name) })
	for name, reason := range skipped {
		sel.Skipped = append(sel.Skipped, Skip{File: name, Reason: reason})
	}
	slices.SortFunc(sel.Skipped, func(a, b Skip) int { return strings.Compare(a.File, b.File) })
	sel.Imports = sortedKeys(imports)
	slices.Sort(sel.Asm)
	sel.Asm = slices.Compact(sel.Asm)
	return sel, nil
}

// sortedKeys returns the keys of a set, sorted.
func sortedKeys(m map[string]bool) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// skipReason says why go/build left a file out of a package for the system
// goos. ok is false for a file that is not a source of the package at all.
func skipReason(ctxt *build.Context, dir, name string, pin Pin, opts Options, goos string) (reason string, ok bool) {
	switch ext := filepath.Ext(name); ext {
	case ".go":
	case ".s", ".S":
		return "assembly: gocpp has no Go assembly, the pure-Go variant is taken", true
	case ".c", ".h", ".cc", ".cpp", ".m":
		return "C/C++ source: not ported", true
	default:
		return "", false
	}
	src, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "unreadable: " + err.Error(), true
	}
	// Does the name alone exclude the file (_windows.go, _amd64.go)?
	byName := *ctxt
	byName.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("package x\n")), nil }
	if match, _ := byName.MatchFile(dir, name); !match {
		return fmt.Sprintf("the file name excludes GOOS=%s GOARCH=%s", goos, opts.Arch), true
	}
	if line := buildLine(src); line != "" {
		return line + " is false", true
	}
	if importsC(src) {
		return "cgo: gocpp has no cgo (import \"C\")", true
	}
	return "excluded by the build constraints", true
}

// buildLine returns the //go:build line of a Go source, or "".
func buildLine(src []byte) string {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return ""
	}
	for _, g := range f.Comments {
		if g.Pos() >= f.Package {
			break
		}
		for _, c := range g.List {
			if strings.HasPrefix(c.Text, "//go:build ") {
				return c.Text
			}
		}
	}
	return ""
}

// importsC reports whether a Go source imports "C".
func importsC(src []byte) bool {
	f, err := parser.ParseFile(token.NewFileSet(), "", bytes.Clone(src), parser.ImportsOnly)
	if err != nil {
		return false
	}
	for _, imp := range f.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == "C" {
			return true
		}
	}
	return false
}

// packageExists reports whether the tree has a folder for the package.
func packageExists(root, pkg string) bool {
	st, err := os.Stat(filepath.Join(root, "src", filepath.FromSlash(pkg)))
	return err == nil && st.IsDir()
}
