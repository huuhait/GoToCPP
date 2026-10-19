// parseArgs, parseSymbol, findNextPackage and the loop of run follow main.go
// of cmd/internal/doc of Go 1.25.14 (Copyright 2015 The Go Authors. All
// rights reserved. Use of that source code is governed by a BSD-style
// license that can be found in internal/doc/LICENSE).
//
// gocpp: they return errors instead of calling log.Fatal, find packages
// with the runner (dirs.go), and know the header and namespace arguments.

package doc

import (
	"bytes"
	"context"
	"fmt"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/stdlib"
)

// A runner is one run of "gocpp doc".
type runner struct {
	w    io.Writer
	env  *Env
	opts Options

	dir    string // the absolute current folder
	stdFS  fs.FS
	stdDir string
	ctx    context.Context

	mods     *moduleState
	dirs     *dirsScan
	testOnly error               // the first partial-path match that had only _test.gp files
	found    map[string]dirEntry // the folders findNextPackage returned, by key
}

func newRunner(w io.Writer, env *Env, opts Options) *runner {
	if env == nil {
		env = &Env{}
	}
	r := &runner{w: w, env: env, opts: opts, stdFS: env.Stdlib, stdDir: env.StdlibDir, ctx: env.Context}
	if r.stdFS == nil {
		r.stdFS = stdlib.FS()
	}
	if r.stdDir == "" {
		r.stdDir = "$GOCPP/stdlib"
	}
	if r.ctx == nil {
		r.ctx = context.Background()
	}
	r.dir = env.Dir
	return r
}

// A target is what the arguments name.
type target struct {
	pkg      *Package
	hdr      *header // instead of pkg, for a header or a namespace
	userPath string  // what the user typed to find the package
	symbol   string  // the symbol, maybe with a .method
	more     bool    // there may be more folders worth looking at
}

// run finds what args name and writes its documentation.
func (r *runner) run(args []string) error {
	if r.dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		r.dir = wd
	}
	if !filepath.IsAbs(r.dir) {
		abs, err := filepath.Abs(r.dir)
		if err != nil {
			return err
		}
		r.dir = abs
	}
	r.dirs = newDirsScan(r.codeRoots(), r.env.Warn)
	var paths []string
	var symbol, method string
	// Loop until something is printed.
	for i := 0; ; i++ {
		t, err := r.parseArgs(args)
		if err != nil {
			return err
		}
		if i > 0 && !t.more { // Ignore the "more" bit on the first iteration.
			return failMessage(paths, symbol, method)
		}
		if t.hdr != nil {
			return r.headerDoc(t)
		}
		if t.pkg == nil {
			return fmt.Errorf("no such package: %s", t.userPath)
		}
		symbol, method, err = parseSymbol(t.symbol)
		if err != nil {
			return err
		}
		s, err := newSession(r.w, r.opts, t.pkg, t.userPath)
		if err != nil {
			return err
		}
		paths = append(paths, s.prettyPath())
		found, err := s.render(symbol, method)
		if ferr := s.flush(); ferr != nil && err == nil {
			err = ferr
		}
		if err != nil {
			return err
		}
		if found {
			return nil
		}
	}
}

// failMessage creates a nicely formatted error message when there is no
// result to show.
func failMessage(paths []string, symbol, method string) error {
	var b bytes.Buffer
	if len(paths) > 1 {
		b.WriteString("s")
	}
	b.WriteString(" ")
	for i, path := range paths {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(path)
	}
	if method == "" {
		return fmt.Errorf("no symbol %s in package%s", symbol, &b)
	}
	return fmt.Errorf("no method or field %s.%s in package%s", symbol, method, &b)
}

// parseArgs analyzes the arguments (if any) and returns the package it
// represents, the part of the argument the user used to identify the path
// (or "" if it's the current package) and the symbol (possibly with a
// .method) within that package. parseSymbol is used to analyze the symbol
// itself. The boolean final result reports whether it is possible that
// there may be more folders worth looking at. It will only be true if the
// package path is a partial match for some folder and there may be more
// matches. For example, if the argument is rand.Float64, we must scan both
// crypto/rand and math/rand to find the symbol, and the first call will
// return crypto/rand, true.
func (r *runner) parseArgs(args []string) (t target, err error) {
	wd := r.dir
	if len(args) == 0 {
		// Easy: current folder.
		pkg, err := r.importDir(wd)
		return target{pkg: pkg}, err
	}
	arg := args[0]
	// We have an argument. If it is a folder name beginning with . or ..,
	// use the absolute path name. This discriminates "./errors" from "errors"
	// if the current folder contains a non-standard errors package.
	if isDotSlash(arg) {
		arg = filepath.Join(wd, arg)
	}
	switch len(args) {
	default:
		return target{}, &UsageError{"too many arguments"}
	case 1:
		// Done below.
	case 2:
		if h, ok, err := r.headerArg(args[0]); ok {
			return target{hdr: h, userPath: args[0], symbol: args[1]}, err
		}
		// Package must be findable and importable.
		pkg, err := r.importPath(arg)
		if err == nil {
			return target{pkg: pkg, userPath: args[0], symbol: args[1]}, nil
		}
		for {
			packagePath, ok := r.findNextPackage(arg)
			if !ok {
				break
			}
			if pkg, err := r.importDirEntry(packagePath); err == nil {
				return target{pkg: pkg, userPath: arg, symbol: args[1], more: true}, nil
			}
		}
		if stdlib.IsStd(arg) { // keep the reason: it is not a typo
			return target{}, err
		}
		if r.testOnly != nil {
			return target{}, r.testOnly
		}
		return target{userPath: args[0], symbol: args[1]}, nil
	}
	// Usual case: one argument.
	if h, ok, err := r.headerArg(args[0]); ok {
		return target{hdr: h, userPath: args[0]}, err
	}
	// If it contains slashes, it begins with either a package path
	// or an absolute folder.
	// First, is it a complete package path as it is? If so, we are done.
	// This avoids confusion over package paths that have other
	// package paths as their prefix.
	pkg, importErr := r.importPath(arg)
	if importErr == nil {
		return target{pkg: pkg, userPath: arg}, nil
	}
	// Another disambiguator: If the argument starts with an upper
	// case letter, it can only be a symbol in the current folder.
	// Kills the problem caused by case-insensitive file systems
	// matching an upper case name as a package name.
	if !strings.ContainsAny(arg, `/\`) && token.IsExported(arg) {
		if pkg, err := r.importDir("."); err == nil {
			return target{pkg: pkg, symbol: arg}, nil
		}
	}
	// If it has a slash, it must be a package path but there is a symbol.
	// It's the last package path we care about.
	slash := strings.LastIndex(arg, "/")
	// There may be periods in the package path before or after the slash
	// and between a symbol and method.
	// Split the string at various periods to see what we find.
	// In general there may be ambiguities but this should almost always
	// work.
	var period int
	// slash+1: if there's no slash, the value is -1 and start is 0; otherwise
	// start is the byte after the slash.
	for start := slash + 1; start < len(arg); start = period + 1 {
		period = strings.Index(arg[start:], ".")
		symbol := ""
		if period < 0 {
			period = len(arg)
		} else {
			period += start
			symbol = arg[period+1:]
		}
		// Have we identified a package already?
		pkg, err := r.importPath(arg[0:period])
		if err == nil {
			return target{pkg: pkg, userPath: arg[0:period], symbol: symbol}, nil
		}
		// See if we have the basename or tail of a package, as in json for encoding/json
		// or ivy/value for robpike.io/ivy/value.
		pkgName := arg[:period]
		for {
			path, ok := r.findNextPackage(pkgName)
			if !ok {
				break
			}
			if pkg, err = r.importDirEntry(path); err == nil {
				return target{pkg: pkg, userPath: arg[0:period], symbol: symbol, more: true}, nil
			}
		}
		r.dirs.Reset() // Next iteration of for loop must scan all the folders again.
	}
	// If it has a slash, we've failed.
	if slash >= 0 {
		importErrStr := importErr.Error()
		if strings.Contains(importErrStr, arg[:period]) {
			return target{}, importErr
		}
		return target{}, fmt.Errorf("no such package %s: %s", arg[:period], importErrStr)
	}
	// std and ::ns.name are the C++ namespaces, not packages: a header says
	// which names they have.
	if h := r.namespaceArg(args[0]); h != nil {
		return target{hdr: h, userPath: args[0]}, nil
	}
	// Guess it's a symbol in the current folder.
	pkg, err = r.importDir(wd)
	if err != nil && r.testOnly != nil {
		return target{}, r.testOnly
	}
	return target{pkg: pkg, symbol: arg}, err
}

// importDirEntry reads the package of a folder that findNextPackage found:
// a folder entry of the scan, or an absolute folder given by the user.
func (r *runner) importDirEntry(key string) (pkg *Package, err error) {
	defer func() {
		// A folder with only test files matched the name: if nothing else
		// does, that is the reason to tell, not "no such package".
		if err != nil && strings.Contains(err.Error(), onlyTestFilesNote) && r.testOnly == nil {
			r.testOnly = err
		}
	}()
	if e, ok := r.found[key]; ok {
		return r.env.readPackage(e.root.fsys, e.rel, e.dir, e.importPath, e.root.std)
	}
	return r.importDir(key)
}

// dotPaths lists all the dotted paths legal on Unix-like and Windows-like
// file systems. We check them all, as the chance of error is minute and
// even on Windows people will use ./ sometimes.
var dotPaths = []string{
	`./`,
	`../`,
	`.\`,
	`..\`,
}

// isDotSlash reports whether the path begins with a reference to the local
// . or .. folder.
func isDotSlash(arg string) bool {
	if arg == "." || arg == ".." {
		return true
	}
	for _, dotPath := range dotPaths {
		if strings.HasPrefix(arg, dotPath) {
			return true
		}
	}
	return false
}

// parseSymbol breaks str apart into a symbol and method. Both may be
// missing or the method may be missing. If present, each must be a valid
// identifier (with the ! or # of gocpp's names).
func parseSymbol(str string) (symbol, method string, err error) {
	if str == "" {
		return
	}
	elem := strings.Split(str, ".")
	switch len(elem) {
	case 1:
	case 2:
		method = elem[1]
	default:
		return "", "", &UsageError{"too many periods in symbol specification"}
	}
	symbol = elem[0]
	return
}

// findNextPackage returns the next key that matches the (perhaps partial)
// package path pkg: the absolute folder, or the name of a folder of the scan
// (see importDirEntry). The boolean reports if any match was found.
func (r *runner) findNextPackage(pkg string) (string, bool) {
	if filepath.IsAbs(pkg) {
		if r.dirs.offset == 0 {
			r.dirs.offset = -1
			return pkg, true
		}
		return "", false
	}
	if pkg == "" || token.IsExported(pkg) { // Upper case symbol cannot be a package name.
		return "", false
	}
	pkg = path.Clean(pkg)
	pkgSuffix := "/" + pkg
	for {
		d, ok := r.dirs.Next()
		if !ok {
			return "", false
		}
		if d.importPath == pkg || strings.HasSuffix(d.importPath, pkgSuffix) {
			if r.found == nil {
				r.found = make(map[string]dirEntry)
			}
			key := d.root.dir + "|" + d.rel
			r.found[key] = d
			return key, true
		}
	}
}
