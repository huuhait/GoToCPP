package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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
//
// A compile that build.WatchPipes stopped (a header that is a named pipe) ends with the line
// "./a.c: open ./pipe.h: not a regular file: it is a named pipe ...", after what the compiler
// printed before it opened the pipe (a warning): the output alone would not say why it failed.
func (d *driver) sourceFailed(pkg *check.Package, e *build.CompileError) {
	out := withPipeLine(e, e.Source)
	if out == "" {
		out = fmt.Sprintf("%s: %v (the compiler printed nothing)", e.Command, e.Err)
	}
	d.sourcesHeader(pkg)
	fmt.Fprintf(d.stderr, "%s\n", d.shortenDir(pkg, out))
}

// withPipeLine returns the output of the failed compile e without its last newlines, then its
// pipeLine, if it has one.
func withPipeLine(e *build.CompileError, what string) string {
	out := strings.TrimRight(string(e.Output), "\n")
	line := pipeLine(e, what)
	switch {
	case line == "":
		return out
	case out == "":
		return line
	}
	return out + "\n" + line
}

// pipeLine returns "<what>: <the PipeError>" when build.WatchPipes stopped the compile e at a named
// pipe (what is the file that was compiled, or e.What when it is ""), or "".
func pipeLine(e *build.CompileError, what string) string {
	var pe *build.PipeError
	if !errors.As(e.Err, &pe) {
		return ""
	}
	if what == "" {
		what = e.What
	}
	return what + ": " + pe.Error()
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

// quotedIncludeRx matches a quoted #include line of a C or C++ file.
var quotedIncludeRx = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*include[ \t]*"([^"\n]+)"`)

// externCClashHints returns the hints for a link error that may come from a
// mismatch of linkage (step 4.16): gocpp wraps a local header in extern "C"
// for the generated code (externC, check's Package.ExternC: the include texts
// such as "./helpers.h"), and a C++ file of the package in dir includes the
// same header as C++, so what that file defines has C++ linkage and the calls
// from the generated code ask for C linkage. The hint, one per header, is
// worded as a possibility and printed only when it can be the cause, and
// conservatively (when unsure, no hint): an undefined symbol (undefined holds
// the names of the link error, mangled or not) appears as a whole word in a
// C++ file that has the stem of the header (helpers.cpp for helpers.h) or
// #includes it, and the file has no extern "C" at all (a file that wraps the
// include, or declares what it defines, is not the trap).
func externCClashHints(dir string, externC map[string]bool, cxxFiles, undefined []string) []string {
	var hdrs []string
	for t, wrapped := range externC {
		if h, err := strconv.Unquote(t); wrapped && err == nil && strings.HasPrefix(t, `"`) {
			hdrs = append(hdrs, h)
		}
	}
	slices.Sort(hdrs)
	var syms []*regexp.Regexp
	for _, u := range undefined {
		if name := symbolIdent(u); name != "" {
			syms = append(syms, regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`))
		}
	}
	var hints []string
	for _, h := range hdrs {
		stem := strings.TrimSuffix(path.Base(h), path.Ext(h))
		for _, f := range cxxFiles {
			text, err := os.ReadFile(filepath.Join(dir, f))
			if err != nil || strings.Contains(string(text), `extern "C"`) {
				continue
			}
			if !includesHeader(string(text), h) && strings.TrimSuffix(f, path.Ext(f)) != stem {
				continue
			}
			if !slices.ContainsFunc(syms, func(rx *regexp.Regexp) bool { return rx.Match(text) }) {
				continue
			}
			hints = append(hints, fmt.Sprintf("(%s parses as C, so gocpp includes it in extern \"C\" for the generated code; if %s includes it outside extern \"C\", what %[2]s defines has C++ linkage and the calls don't find it: add //gocpp:no-extern-c %[1]s, or #ifdef __cplusplus extern \"C\" guards to the header)", h, f))
			break
		}
	}
	return hints
}

// symbolIdent returns the name of the function in a linker symbol as the
// source writes it: "ns::f(int, int)" gives f, and a C name is itself.
func symbolIdent(sym string) string {
	if i := strings.IndexByte(sym, '('); i >= 0 {
		sym = sym[:i]
	}
	if i := strings.LastIndex(sym, "::"); i >= 0 {
		sym = sym[i+2:]
	}
	return strings.TrimSpace(sym)
}

// includesHeader reports whether the C++ text has a quoted #include of the
// header h (a path as the .gp file imports it, relative to the package
// folder, which is also where the file is): the same cleaned path, or the
// bare file name for a header in the package folder itself.
func includesHeader(text, h string) bool {
	for _, m := range quotedIncludeRx.FindAllStringSubmatch(text, -1) {
		inc := path.Clean(m[1])
		if inc == path.Clean(h) || (!strings.Contains(inc, "/") && path.Dir(path.Clean(h)) == "." && inc == path.Base(h)) {
			return true
		}
	}
	return false
}
