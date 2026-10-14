// The pattern matching in this file follows the go command of Go 1.25.14
// closely: matchLocalDirs and walkPkgs in cmd/go/internal/modload,
// Match.MatchDirs in cmd/go/internal/search, and MatchPattern and
// TreeCanMatchPattern in cmd/internal/pkgpattern. Copyright 2017 The Go
// Authors. All rights reserved. Use of that source code is governed by a
// BSD-style license that can be found in internal/syntax/LICENSE.
//
// gocpp: the loader (internal/load) doesn't match patterns with "..." yet,
// so gocpp check does it here and gives the loader the folders and import
// paths that match. Phase 06 (gocpp list) can move this into the loader.

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// A patternMatch is what the patterns of gocpp check matched.
type patternMatch struct {
	// patterns are the patterns for load.Load: the patterns without "...",
	// as they are, and the folders and import paths that the patterns
	// with "..." match.
	patterns []string

	// wildDirs and wildPaths hold the packages that a pattern with "..."
	// matched: the absolute folders of the local patterns, the import
	// paths of the others. explicitDirs and explicitPaths hold the
	// packages that a pattern without "..." names.
	wildDirs, explicitDirs   map[string]bool
	wildPaths, explicitPaths map[string]bool

	// files says that the patterns are a list of .gp files.
	files bool

	// wild lists the patterns with "..." that have no errors, and
	// byPattern what each one matched (absolute folders or import paths),
	// for the warning "matched no packages".
	wild      []string
	byPattern map[string][]string

	// errors are the errors of the patterns ("pattern ./x/...: lstat
	// ./x/: no such file or directory"): like go list's, they stop gocpp
	// check before the type checking. warnings are printed first
	// ("warning: ignoring symlink x").
	errors, warnings []string
}

// matchPatterns matches the patterns of gocpp check, with cwd as the
// current folder.
//
// A local pattern with "..." (./..., ./geom/..., ../x/...) walks its folder
// like the go command does: it leaves out folders whose names start with
// "." or "_", testdata folders, the folders of other modules (with a
// gocpp.mod of their own) and the packages in vendor folders, and it takes
// each folder that has a .gp file. The folder must be in the main module,
// the module of cwd. Another pattern with "..." (..., example.com/app/...)
// matches the import paths of the packages of the main module and of the
// standard library. A pattern without "..." goes to the loader as it is.
//
// The go command also leaves out a folder with no Go files to build (only
// tests, or only files that the build constraints exclude). Only the
// loader knows that, so gocpp check drops such packages after loading
// (load.Package.NoFiles, see droppedPackages).
func matchPatterns(cwd string, patterns []string) *patternMatch {
	m := &patternMatch{
		wildDirs: make(map[string]bool), explicitDirs: make(map[string]bool),
		wildPaths: make(map[string]bool), explicitPaths: make(map[string]bool),
		byPattern: make(map[string][]string),
	}
	// Like the go command, a pattern ending in .gp is a list of files
	// only when one of them is a file; else a folder named x.gp is a
	// folder.
	m.files = slices.ContainsFunc(patterns, func(p string) bool {
		fi, err := os.Stat(p)
		return strings.HasSuffix(p, ".gp") && err == nil && !fi.IsDir()
	})
	for _, pat := range patterns {
		switch {
		case m.files:
			m.patterns = append(m.patterns, pat)
		case !strings.Contains(pat, "..."):
			if isLocal(pat) {
				m.explicitDirs[absIn(cwd, pat)] = true
				pat = loaderDir(pat)
			} else {
				m.explicitPaths[pat] = true
			}
			m.patterns = append(m.patterns, pat)
		case isLocal(pat):
			m.matchLocal(cwd, pat)
		default:
			m.matchImportPaths(cwd, pat)
		}
	}
	return m
}

// loaderDir returns the folder name dir for the loader, which takes a
// first pattern that ends in .gp for a list of files: a folder named x.gp
// gets a separator at its end.
func loaderDir(dir string) string {
	if strings.HasSuffix(dir, ".gp") {
		return dir + string(filepath.Separator)
	}
	return dir
}

// matchLocal matches the local pattern pat with "...", like
// matchLocalDirs and Match.MatchDirs of the go command.
func (m *patternMatch) matchLocal(cwd, pat string) {
	i := strings.Index(pat, "...")
	dir := filepath.Dir(filepath.Clean(pat[:i+3]))
	absDir := absIn(cwd, dir)
	main, here := load.FindModule(cwd), load.FindModule(absDir)
	if main == nil || here == nil || here.Dir != main.Dir {
		m.errors = append(m.errors, fmt.Sprintf("pattern %s: directory prefix %s does not contain main module or its selected dependencies",
			pat, diag.ShortPath(cwd, absDir)))
		return
	}
	var matched []string

	// filepath.Clean removes "./" prefixes (and ".\" on Windows). We need
	// to preserve these, since they are meaningful in MatchPattern and in
	// the names of the folders.
	cleanPattern := filepath.Clean(pat)
	local := strings.HasPrefix(pat, "./") || (os.PathSeparator == '\\' && strings.HasPrefix(pat, `.\`))
	prefix := ""
	if cleanPattern != "." && local {
		prefix = "./"
		cleanPattern = "." + string(os.PathSeparator) + cleanPattern
	}
	match := matchPattern(filepath.ToSlash(cleanPattern))
	i = strings.Index(cleanPattern, "...")
	root, _ := filepath.Split(cleanPattern[:i])
	// If root is a symlink to a folder, follow it: a separator at the end
	// makes the walk do that.
	if root == "" || !os.IsPathSeparator(root[len(root)-1]) {
		root += string(filepath.Separator)
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // likely a permission error, which could interfere with matching
		}
		if !d.IsDir() {
			return nil
		}
		top := p == root
		if top {
			p = filepath.Clean(p)
		}
		// Avoid .foo, _foo, and testdata folder trees, but not "." or "..".
		_, elem := filepath.Split(p)
		dot := strings.HasPrefix(elem, ".") && elem != "." && elem != ".."
		if dot || strings.HasPrefix(elem, "_") || elem == "testdata" {
			return filepath.SkipDir
		}
		if !top && isFile(filepath.Join(p, "gocpp.mod")) {
			return filepath.SkipDir // another module
		}
		name := prefix + filepath.ToSlash(p)
		if !match(name) || !hasGpFile(p) {
			return nil
		}
		m.patterns = append(m.patterns, loaderDir(filepath.FromSlash(name)))
		m.wildDirs[absIn(cwd, p)] = true
		matched = append(matched, absIn(cwd, p))
		return nil
	})
	if err != nil {
		m.errors = append(m.errors, fmt.Sprintf("pattern %s: %v", pat, err))
		return
	}
	m.wild = append(m.wild, pat)
	m.byPattern[pat] = matched
}

// matchImportPaths matches the pattern pat with "..." that is not local
// against the import paths of the packages of the main module (the
// module of cwd) and of the standard library, like the walkPkgs of the go
// command.
func (m *patternMatch) matchImportPaths(cwd, pat string) {
	match, treeCanMatch := matchPattern(pat), treeCanMatchPattern(pat)
	nerr := len(m.errors)
	var matched []string
	add := func(name string) {
		if match(name) {
			matched = append(matched, name)
			if !m.wildPaths[name] {
				m.wildPaths[name] = true
				m.patterns = append(m.patterns, name)
			}
		}
	}
	if mod := load.FindModule(cwd); mod != nil && mod.Path != "" {
		// If the root itself is a symlink to a folder, follow it.
		root := mod.Dir + string(filepath.Separator)
		err := filepath.WalkDir(root, func(pkgDir string, d fs.DirEntry, err error) error {
			if err != nil {
				m.errors = append(m.errors, fmt.Sprintf("pattern %s: %v", pat, err))
				return nil
			}
			want := true
			elem := ""
			if pkgDir != root {
				// Avoid .foo, _foo, and testdata subfolder trees.
				_, elem = filepath.Split(pkgDir)
				if strings.HasPrefix(elem, ".") || strings.HasPrefix(elem, "_") || elem == "testdata" {
					want = false
				}
			}
			name := path.Join(mod.Path, filepath.ToSlash(pkgDir[len(root):]))
			if !treeCanMatch(name) {
				want = false
			}
			if !d.IsDir() {
				if d.Type()&fs.ModeSymlink != 0 && want {
					if target, err := os.Stat(pkgDir); err == nil && target.IsDir() {
						m.warnings = append(m.warnings, "warning: ignoring symlink "+pkgDir)
					}
				}
				return nil
			}
			if !want {
				return filepath.SkipDir
			}
			// Stop at module boundaries.
			if pkgDir != root && isFile(filepath.Join(pkgDir, "gocpp.mod")) {
				return filepath.SkipDir
			}
			if hasGpFile(pkgDir) {
				add(name)
			}
			if elem == "vendor" {
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			m.errors = append(m.errors, fmt.Sprintf("pattern %s: %v", pat, err))
		}
	}
	for _, name := range stdPackages() {
		add(name)
	}
	if len(m.errors) == nerr {
		m.wild = append(m.wild, pat)
		m.byPattern[pat] = matched
	}
}

// stdPackages returns the import paths of the packages of the standard
// library, sorted.
func stdPackages() []string {
	var paths []string
	fs.WalkDir(stdlib.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".gp") {
			if dir := path.Dir(p); !slices.Contains(paths, dir) {
				paths = append(paths, dir)
			}
		}
		return nil
	})
	slices.Sort(paths)
	return paths
}

// isLocal reports whether the pattern pat names a folder or a file by its
// path, like build.IsLocalImport plus absolute paths in the go command.
func isLocal(pat string) bool {
	return pat == "." || pat == ".." || strings.HasPrefix(pat, "./") || strings.HasPrefix(pat, "../") ||
		filepath.IsAbs(pat) || (filepath.Separator == '\\' && (strings.HasPrefix(pat, `.\`) || strings.HasPrefix(pat, `..\`)))
}

// absIn returns p as an absolute, clean path, relative to dir if it is
// relative.
func absIn(dir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(dir, p)
}

func isFile(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && !fi.IsDir()
}

// hasGpFile reports whether the folder dir has a .gp file whose name
// doesn't start with "." or "_". go's ImportDir leaves out a folder
// without such a file (NoGoError), and so does the walk.
func hasGpFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true // the loader reports the error
	}
	return slices.ContainsFunc(entries, func(e fs.DirEntry) bool {
		name := e.Name()
		return !e.IsDir() && strings.HasSuffix(name, ".gp") && !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_")
	})
}

// matchPattern returns a function that reports whether a name (a slash
// path) matches the pattern, which uses "..." as a wildcard for any
// string: MatchPattern of the go command. The "..." doesn't match through
// a vendor folder (x/vendor/y), and x/... also matches x.
func matchPattern(pattern string) func(name string) bool {
	// The strategy for the trailing /... is to nest it in an explicit ?
	// expression. The strategy for the vendor exclusion is to change the
	// unmatchable vendor strings to a disallowed code point (vendorChar)
	// and to use "(anything but that codepoint)*" as the implementation of
	// the ... wildcard.
	const vendorChar = "\x00"
	if strings.Contains(pattern, vendorChar) {
		return func(string) bool { return false }
	}
	re := regexp.QuoteMeta(pattern)
	wild := `[^` + vendorChar + `]*`
	re = replaceVendor(re, vendorChar)
	switch {
	case strings.HasSuffix(re, `/`+vendorChar+`/\.\.\.`):
		re = strings.TrimSuffix(re, `/`+vendorChar+`/\.\.\.`) + `(/vendor|/` + vendorChar + `/\.\.\.)`
	case re == vendorChar+`/\.\.\.`:
		re = `(/vendor|/` + vendorChar + `/\.\.\.)`
	}
	if strings.HasSuffix(re, `/\.\.\.`) {
		re = strings.TrimSuffix(re, `/\.\.\.`) + `(/\.\.\.)?`
	}
	re = strings.ReplaceAll(re, `\.\.\.`, wild)
	reg := regexp.MustCompile(`^` + re + `$`)
	return func(name string) bool {
		if strings.Contains(name, vendorChar) {
			return false
		}
		return reg.MatchString(replaceVendor(name, vendorChar))
	}
}

// replaceVendor returns the result of replacing non-trailing vendor path
// elements in x with repl.
func replaceVendor(x, repl string) string {
	if !strings.Contains(x, "vendor") {
		return x
	}
	elem := strings.Split(x, "/")
	for i := 0; i < len(elem)-1; i++ {
		if elem[i] == "vendor" {
			elem[i] = repl
		}
	}
	return strings.Join(elem, "/")
}

// treeCanMatchPattern returns a function that reports whether a name or
// any name below it can match the pattern: TreeCanMatchPattern of the go
// command.
func treeCanMatchPattern(pattern string) func(name string) bool {
	wildCard := false
	if i := strings.Index(pattern, "..."); i >= 0 {
		wildCard = true
		pattern = pattern[:i]
	}
	return func(name string) bool {
		return len(name) <= len(pattern) && hasPathPrefix(pattern, name) ||
			wildCard && strings.HasPrefix(name, pattern)
	}
}

// hasPathPrefix reports whether the slash path s begins with the elements
// of prefix.
func hasPathPrefix(s, prefix string) bool {
	switch {
	case len(s) == len(prefix):
		return s == prefix
	case len(s) > len(prefix):
		if prefix != "" && prefix[len(prefix)-1] == '/' {
			return strings.HasPrefix(s, prefix)
		}
		return s[len(prefix)] == '/' && s[:len(prefix)] == prefix
	}
	return false
}
