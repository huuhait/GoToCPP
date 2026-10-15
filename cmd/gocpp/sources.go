package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// hasSources reports whether the package lp has C or C++ files of its own
// (Phase 04, step 4.13), which gocpp build compiles and links in.
func hasSources(lp *load.Package) bool {
	return len(lp.CFiles) > 0 || len(lp.CXXFiles) > 0
}

// findCC returns the C compiler for the package's .c files that goes with
// the C++ compiler cxx (build.FindCC). With -asan, it warns when GOCPP_CC
// names a compiler of another family: the sanitizers of two compilers in
// one program are not supported.
func (d *driver) findCC(cxx build.CXX) (build.CXX, error) {
	cc, err := build.FindCC(d.ctx, cxx)
	if err == nil && d.flags.asan && cc.Family != cxx.Family {
		from := ""
		if os.Getenv("GOCPP_CC") != "" {
			from = ", GOCPP_CC"
		}
		fmt.Fprintf(d.stderr, "gocpp %s: warning: -asan with a C compiler (%s%s) of another family than the C++ compiler (%s) is not supported\n",
			d.name, cc.Name(), from, cxx.Name())
	}
	return cc, err
}

// sourceFailed prints the error of a C or C++ file of the package that
// didn't compile: an error in the user's code, so, like go build for a
// cgo file, the compiler's whole output under the package's line, with
// the package folder written as gocpp check writes it ("./a.c:3:5:
// error: ...").
func (d *driver) sourceFailed(pkg *check.Package, e *build.CompileError) {
	out := strings.TrimRight(string(e.Output), "\n")
	if out == "" {
		out = fmt.Sprintf("%s: %v (the compiler printed nothing)", e.Command, e.Err)
	}
	d.sourcesHeader(pkg)
	fmt.Fprintf(d.stderr, "%s\n", d.shortenDir(pkg, out))
}

// sourcesHeader prints the "# <package>" line above the output of the
// package's C and C++ files, once: warnings of some files and the error
// of another come under one line, as in go build.
func (d *driver) sourcesHeader(pkg *check.Package) {
	if !d.pkgLine {
		fmt.Fprintf(d.stderr, "# %s\n", pkg.Load.ImportPath)
		d.pkgLine = true
	}
}

// sourceWarnings returns the build.Builder.SourceWarnings of pkg: the
// warnings of its C and C++ files go to stderr like their errors, under
// the package's line, which comes once (sourcesHeader). The builder calls
// it for one file at a time, and before Program returns.
func (d *driver) sourceWarnings(pkg *check.Package) func([]byte) {
	return func(out []byte) {
		d.sourcesHeader(pkg)
		fmt.Fprintf(d.stderr, "%s\n", d.shortenDir(pkg, strings.TrimRight(string(out), "\n")))
	}
}

// shortenDir returns the compiler output out with the folder of pkg
// written as gocpp check writes it: "./" for the current folder.
func (d *driver) shortenDir(pkg *check.Package, out string) string {
	dir := pkg.Load.Dir + string(filepath.Separator)
	// A compiler writes a local header found through -iquote <dir> as
	// <dir>/./a.h: that is ./a.h, not ././a.h.
	out = strings.ReplaceAll(out, dir+"."+string(filepath.Separator), dir)
	short := strings.TrimSuffix(diag.ShortFileName(d.check.dir, dir+"x"), "x")
	if short == dir {
		return out
	}
	return strings.ReplaceAll(out, dir, short)
}
