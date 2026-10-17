package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/lower"
)

// This file reports the errors of the C++ compiler and of the linker in a
// package that uses C or C++ (Phase 04 README §2.11, step 4.17). There, a
// C++ error that comes from a .gp line is an error of the program: a wrong
// argument to a C function, a C++ template that doesn't accept the user's
// type. gocpp prints it like its own errors, at the .gp line, with the
// first error's message and the source line:
//
//	./main.gp:14: C++: no member named 'push_bak' in 'std::vector<long>'
//		nums.push_bak(4)
//
// The rest of the compiler's output (notes, template backtraces) is left
// out; -cpperrors=full shows it all.

// cppErrorsMode is the value of the -cpperrors flag: "short" (the default)
// or "full".
type cppErrorsMode string

func (m *cppErrorsMode) String() string { return string(*m) }

func (m *cppErrorsMode) Set(s string) error {
	if s != "short" && s != "full" {
		return fmt.Errorf("want short or full")
	}
	*m = cppErrorsMode(s)
	return nil
}

// full reports whether -cpperrors=full is set.
func (m cppErrorsMode) full() bool { return m == "full" }

// pkgUsesCpp reports whether pkg uses C or C++: a file imports a header or
// uses C (the loader adds import "C" to such a file).
func pkgUsesCpp(pkg *check.Package) bool {
	for _, f := range pkg.Load.Files {
		if f.Info != nil && len(f.Info.Includes) > 0 {
			return true
		}
		if f.AST != nil {
			for _, spec := range f.AST.Imports {
				if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == "C" {
					return true
				}
			}
		}
	}
	return false
}

// cppLineSet holds the .gp lines that have C or C++ code, by file (the
// absolute name, as #line writes it): only an error there can be the
// user's (see cppLines).
type cppLineSet map[string]map[int]bool

func (s cppLineSet) add(file string, line int) {
	if s[file] == nil {
		s[file] = make(map[int]bool)
	}
	s[file][line] = true
}

func (s cppLineSet) has(l cxxLoc) bool { return s[filepath.Clean(l.file)][l.line] }

// cppLines returns the .gp lines of pkg that have C or C++ code: each line
// of an expression that uses C or a header package or namespace (after the
// loader's rewrite, a selector on C) or that is opaque (check.Opaque: a
// value of a C/C++ type), the first line of the statement around it (or of
// the declaration, outside functions), and the header import lines. Nearly
// every generated line of C++ has a #line of its .gp line, so the line of
// an error alone can't tell a bug of gocpp from an error of the user; an
// error on a line without C or C++ code is gocpp's.
func cppLines(fset *token.FileSet, pkg *check.Package) cppLineSet {
	set := make(cppLineSet)
	for _, f := range pkg.Load.Files {
		if f.Info != nil {
			for _, inc := range f.Info.Includes {
				if inc.Pos.IsValid() {
					p := fset.Position(inc.Pos)
					set.add(filepath.Clean(p.Filename), p.Line)
				}
			}
		}
		if f.AST == nil {
			continue
		}
		var stack []ast.Node
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			e, ok := n.(ast.Expr)
			if !ok || !isCppExpr(pkg, e) {
				return true
			}
			// The parts of e are on its lines: Inspect doesn't go into
			// them, and calls no f(nil) for e.
			stack = stack[:len(stack)-1]
			from, to := fset.Position(e.Pos()), fset.Position(e.End())
			file := filepath.Clean(from.Filename)
			for l := from.Line; l <= to.Line; l++ {
				set.add(file, l)
			}
			for i := len(stack) - 1; i >= 0; i-- {
				switch stack[i].(type) {
				case ast.Stmt, ast.Spec, ast.Decl:
					set.add(file, fset.Position(stack[i].Pos()).Line)
					return false
				}
			}
			return false
		})
	}
	return set
}

// isCppExpr reports whether the expression e of pkg is C or C++ code: a
// selector on C (C.int, C.std.vector, a header package's function after
// the loader's rewrite), a cgo helper (C.GoString, which check rewrote),
// an opaque expression, or a gocpp value with a C/C++ part (a struct with
// a C/C++ field, which fmt prints through C++: step 4.12).
func isCppExpr(pkg *check.Package, e ast.Expr) bool {
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "C" || pkg.IsCgoHelper(sel) {
			return true
		}
	}
	if pkg.Opaque.IsOpaque(e) {
		return true
	}
	if pkg.Info == nil {
		return false
	}
	tv, ok := pkg.Info.Types[e]
	return ok && tv.IsValue() && tv.Type != nil && lower.HasC(tv.Type)
}

// A cxxLoc is a place in a file that the compiler names, without the
// column: with #line, the column of a .gp line is the column in the C++.
type cxxLoc struct {
	file string // as the compiler prints it; "" when the message has no place
	line int
}

// A cxxDiag is one error of the compiler, with the places that lead to it.
type cxxDiag struct {
	loc  cxxLoc
	msg  string   // without "error: "
	inst []cxxLoc // the template instantiations that lead to it, the innermost first
	mac  []cxxLoc // gcc: the expansions of the macro that it is in, the innermost first
	incl []cxxLoc // the #include lines that lead to its file, the innermost first
}

var (
	// ansiRx matches the color codes (SGR, erase in line) and the
	// hyperlinks (OSC 8) that a compiler writes with -fdiagnostics-color
	// or -fdiagnostics-urls.
	ansiRx = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]|\x1b\\][^\x1b\a]*(?:\x1b\\\\|\a)")
	// diagLineRx matches "file:line:col: kind: message" (the column may be
	// missing).
	diagLineRx = regexp.MustCompile(`^(.+?):(\d+):(?:\d+:)? (fatal error|error|warning|note|remark): (.*)$`)
	// toolErrorRx matches an error without a place: "cc1plus: error: ...".
	toolErrorRx = regexp.MustCompile(`^(\S+): (?:fatal )?error: (.*)$`)
	// includeRx matches the include stack before a diagnostic: "In file
	// included from a.h:3:" (clang and gcc, which ends the line with ","
	// when more follow) and gcc's "                 from b.h:5:".
	includeRx = regexp.MustCompile(`^(In file included from|\s+from) (.+?):(\d+)(?::\d+)?[,:]$`)
	// contextRx matches gcc's line that starts the context of the errors
	// after it: "a.h: In instantiation of '...':", "x.gp: In function
	// '...':", "At global scope:".
	contextRx = regexp.MustCompile(`^(?:.+?: )?(?:In [a-z][a-z ]* |At global scope)`)
	// frameRx matches a frame of gcc's instantiation context: "a.h:5:3:
	// required from '...'", "x.gp:7:9:   required from here".
	frameRx = regexp.MustCompile(`^(.+?):(\d+):(?:\d+:)?\s{2,}\S`)
	// instNoteRx matches clang's note for an instantiation frame: "in
	// instantiation of function template specialization '...' requested
	// here", "in instantiation of default argument ... required here".
	instNoteRx = regexp.MustCompile(`(?:requested|required) here$`)
	// macroNoteRx matches gcc's note at the use of a macro whose expansion
	// has the error: "in expansion of macro 'PX'". (clang reports such an
	// error at the use itself.)
	macroNoteRx = regexp.MustCompile(`^in expansion of macro `)
	// staticAssertRx matches the message of a failed static_assert, gcc's
	// and clang's (with the condition), and gives the static_assert's own
	// message.
	staticAssertRx = regexp.MustCompile(`^static assertion failed(?: due to requirement '.*?')?: (.*)$`)
)

// cleanCompilerLine removes the color codes and a carriage return from a
// line of the compiler's output.
func cleanCompilerLine(line string) string {
	return ansiRx.ReplaceAllString(strings.TrimSuffix(line, "\r"), "")
}

// parseCXXErrors reads the errors in the output of gcc or clang, with the
// places that lead to each: gcc writes the instantiation context before
// the errors (and keeps it for the errors that follow, until the next
// context line), clang writes it as notes after the error.
func parseCXXErrors(out []byte) []cxxDiag {
	var (
		diags []cxxDiag
		incl  []cxxLoc // the include stack read since the last diagnostic
		ctx   []cxxLoc // gcc's instantiation context
		cur   = -1     // the error that the notes after it belong to, or -1
	)
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := cleanCompilerLine(sc.Text())
		if m := includeRx.FindStringSubmatch(line); m != nil {
			if strings.HasPrefix(m[1], "In") {
				incl = nil
			}
			n, _ := strconv.Atoi(m[3])
			incl = append(incl, cxxLoc{m[2], n})
			continue
		}
		if m := diagLineRx.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])
			loc := cxxLoc{m[1], n}
			switch m[3] {
			case "error", "fatal error":
				diags = append(diags, cxxDiag{loc: loc, msg: m[4], inst: slices.Clone(ctx), incl: incl})
				cur = len(diags) - 1
				incl = nil
			case "note":
				switch {
				case cur < 0:
				case instNoteRx.MatchString(m[4]):
					diags[cur].inst = append(diags[cur].inst, loc)
				case macroNoteRx.MatchString(m[4]):
					diags[cur].mac = append(diags[cur].mac, loc)
				}
			default: // a warning: the notes after it are its own
				cur, incl = -1, nil
			}
			continue
		}
		if m := toolErrorRx.FindStringSubmatch(line); m != nil {
			diags = append(diags, cxxDiag{msg: m[2]})
			cur, incl = len(diags)-1, nil
			continue
		}
		if contextRx.MatchString(line) && strings.HasSuffix(line, ":") {
			ctx = nil
			continue
		}
		if m := frameRx.FindStringSubmatch(line); m != nil && !strings.HasPrefix(line, " ") {
			n, _ := strconv.Atoi(m[2])
			ctx = append(ctx, cxxLoc{m[1], n})
		}
	}
	return diags
}

// A cxxClassifier decides which errors of the generated C++ are the
// user's (see mapError).
type cxxClassifier struct {
	generated string     // the generated C++ file
	include   string     // the runtime's include folder (build.CompileError.Include), or ""
	user      cppLineSet // the .gp lines with C or C++ code (cppLines)
}

func (c *cxxClassifier) isGP(file string) bool { return strings.HasSuffix(file, ".gp") }

func (c *cxxClassifier) isGenerated(file string) bool {
	return file != "" && filepath.Clean(file) == filepath.Clean(c.generated)
}

// isRuntime reports whether file is a header of gocpp's runtime, and, if
// so, whether it is gocpp/interop.hpp: the helpers at the boundary between
// gocpp and C/C++ (gp::cgo::as, case_eq, gp::deref, gp::range, ...), which
// get the user's C/C++ values and whose static_asserts are written for
// users.
func (c *cxxClassifier) isRuntime(file string) (runtime, interop bool) {
	if c.include == "" || file == "" {
		return false, false
	}
	rel, err := filepath.Rel(c.include, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false, false
	}
	return true, filepath.ToSlash(rel) == "gocpp/interop.hpp"
}

// mapError returns the .gp line that the error d comes from, if it is an
// error of the user's code. That .gp line must have C or C++ code
// (cppLines), and:
//   - the error is at that line (through #line);
//   - or it is in a header, in a macro that the line uses (gcc's note "in
//     expansion of macro" at the line);
//   - or it is in a header that the line instantiated a template of: the
//     innermost .gp frame of the instantiations, when no frame inside it
//     is gocpp's (the generated C++ without a .gp line, or the runtime);
//   - or, for the first error only, it is in a header (not the runtime's)
//     that the line, a header import, includes, without instantiations: a
//     header that doesn't compile as C++. (A later error has the include
//     stack too, but it is most likely a follow-on of an earlier one.)
//
// An error in the generated lines without a .gp line, in the runtime other
// than gocpp/interop.hpp, or at a .gp line without C or C++ code, is
// gocpp's. The other frames that are .gp lines come back in from, for the
// report.
func (c *cxxClassifier) mapError(d cxxDiag, first bool) (at cxxLoc, from []cxxLoc, ok bool) {
	file := d.loc.file
	switch rt, interop := c.isRuntime(file); {
	case file == "" || c.isGenerated(file):
		return cxxLoc{}, nil, false
	case c.isGP(file):
		return d.loc, gpFrames(d.inst, d.loc), c.user.has(d.loc)
	case rt && !interop:
		return cxxLoc{}, nil, false
	}
	for _, f := range d.mac {
		if c.isGP(f.file) {
			return f, gpFrames(d.inst, f), c.user.has(f)
		}
	}
	for i, f := range d.inst {
		if c.isGP(f.file) {
			return f, gpFrames(d.inst[i+1:], f), c.user.has(f)
		}
		if rt, interop := c.isRuntime(f.file); c.isGenerated(f.file) || rt && !interop {
			return cxxLoc{}, nil, false
		}
	}
	if rt, _ := c.isRuntime(file); first && !rt && len(d.inst) == 0 {
		for _, f := range d.incl {
			if c.isGP(f.file) {
				return f, nil, c.user.has(f)
			}
		}
	}
	return cxxLoc{}, nil, false
}

// gocppPlace reports whether the place of an error is gocpp's code: the
// generated lines without a .gp line, a .gp line without C or C++ code,
// gocpp's runtime outside gocpp/interop.hpp, or no place at all.
func (c *cxxClassifier) gocppPlace(l cxxLoc) bool {
	rt, interop := c.isRuntime(l.file)
	return l.file == "" || c.isGenerated(l.file) || c.isGP(l.file) && !c.user.has(l) || rt && !interop
}

// gpFrames returns the .gp frames of frames, without at and repeats.
func gpFrames(frames []cxxLoc, at cxxLoc) []cxxLoc {
	var gp []cxxLoc
	for _, f := range frames {
		if strings.HasSuffix(f.file, ".gp") && f != at && !slices.Contains(gp, f) {
			gp = append(gp, f)
		}
	}
	return gp
}

// userMessage returns the message of the error d for the report: with
// ASCII quotes (gcc writes ‘’ in a UTF-8 locale), and, for a static_assert
// of gocpp/interop.hpp, only the static_assert's own message, without its
// "gocpp: ".
func (c *cxxClassifier) userMessage(d cxxDiag) string {
	msg := strings.NewReplacer("‘", "'", "’", "'").Replace(d.msg)
	if _, interop := c.isRuntime(d.loc.file); interop {
		if m := staticAssertRx.FindStringSubmatch(msg); m != nil {
			msg = strings.TrimPrefix(m[1], "gocpp: ")
		}
	}
	if m := cstrRx.FindStringSubmatch(msg); m != nil {
		if m[1] != "" {
			msg += cstrHint
		} else {
			msg += charPtrHint
		}
	}
	return msg
}

// cstrRx matches gcc's and clang's error for a gocpp string passed where C wants a C string,
// cstdio.puts(s): a gp::string converts only to std::string_view (Phase 04 README §2.6). The
// group is "const " for a const char*.
var cstrRx = regexp.MustCompile(`^(?:cannot convert|no viable conversion from) 'gp::string' to '(const )?char ?\*'`)

// cstrHint is the hint after cstrRx's errors for a const char*, charPtrHint for a char*, which
// C.cstr's const char* doesn't give.
const (
	cstrHint    = " (a gocpp string is no C string: pass C.cstr(s), or C.CString(s) for a copy that C keeps)"
	charPtrHint = " (a gocpp string is no C string: a char* parameter gets a copy that C may change, C.CString(s), freed with C.free)"
)

// A cppError is an error of the user's code that the C++ compiler or the
// linker found, at a .gp line.
type cppError struct {
	at   cxxLoc
	msg  string
	from []cxxLoc // more .gp lines of the template backtrace
}

// userCXXErrors returns the errors of the compiler's output out that are
// the user's (cxxClassifier.mapError), one per .gp line (the first one of
// that line), or ok false when the first error is gocpp's: then it is an
// internal error. The errors after the first that are not the user's are
// dropped as follow-ons of the earlier ones (most are: a template that
// fails on the user's type fails in more places); others counts those of
// them that are in gocpp's code (gocppPlace), which may be a bug of gocpp.
func (c *cxxClassifier) userCXXErrors(out []byte) (errs []cppError, others int, ok bool) {
	diags := parseCXXErrors(out)
	if len(diags) == 0 {
		return nil, 0, false
	}
	seen := make(map[cxxLoc]bool)
	for i, d := range diags {
		at, from, mapped := c.mapError(d, i == 0)
		if !mapped {
			if i == 0 {
				return nil, 0, false
			}
			if c.gocppPlace(d.loc) {
				others++
			}
			continue
		}
		if seen[at] {
			continue
		}
		seen[at] = true
		errs = append(errs, cppError{at: at, msg: c.userMessage(d), from: from})
	}
	return errs, others, true
}

// printCppErrors prints errs like gocpp check prints errors: under the
// package's line, sorted by place, at most 10 (then "too many errors"),
// each with its source line and the other .gp lines that lead to it.
func (d *driver) printCppErrors(pkg *check.Package, errs []cppError) {
	var l diag.List
	lines := make(map[string][]string)
	for _, e := range errs {
		var related []diag.Related
		if e.at.file != "" {
			if _, ok := lines[e.at.file]; !ok {
				src, _ := os.ReadFile(e.at.file)
				lines[e.at.file] = strings.Split(string(src), "\n")
			}
			if src := lines[e.at.file]; e.at.line >= 1 && e.at.line <= len(src) {
				if s := strings.TrimSpace(src[e.at.line-1]); s != "" {
					related = append(related, diag.Related{Msg: s})
				}
			}
		}
		for _, f := range e.from {
			related = append(related, diag.Related{Pos: token.Position{Filename: f.file, Line: f.line}, Msg: "from here"})
		}
		l.Add(diag.Diagnostic{Pos: token.Position{Filename: e.at.file, Line: e.at.line}, Severity: diag.Error, Msg: e.msg, Related: related})
	}
	fmt.Fprintf(d.stderr, "# %s\n", pkg.Load.ImportPath)
	p := diag.Printer{Dir: d.check.dir}
	p.Print(d.stderr, &l)
}

// cppCompileFailed reports the error of a compile of the generated C++ of
// a package that uses C or C++, when the errors are the user's
// (userCXXErrors). It returns false, and prints nothing, when they are
// gocpp's: then the caller reports an internal error.
func (d *driver) cppCompileFailed(pkg *check.Package, e *build.CompileError, src string) bool {
	if !pkgUsesCpp(pkg) {
		return false
	}
	c := &cxxClassifier{generated: src, include: e.Include, user: cppLines(d.check.fset, pkg)}
	errs, others, ok := c.userCXXErrors(e.Output)
	if !ok {
		return false
	}
	if d.flags.cppErrors.full() {
		fmt.Fprintf(d.stderr, "# %s\n%s\n", pkg.Load.ImportPath, strings.TrimRight(d.shortenDir(pkg, string(e.Output)), "\n"))
		return true
	}
	for i := range errs {
		errs[i].msg = "C++: " + errs[i].msg
	}
	d.printCppErrors(pkg, errs)
	switch {
	case others == 1:
		fmt.Fprintln(d.stderr, "(and 1 more error in gocpp's code: -cpperrors=full shows it)")
	case others > 1:
		fmt.Fprintf(d.stderr, "(and %d more errors in gocpp's code: -cpperrors=full shows them)\n", others)
	}
	return true
}

// An undefinedRef is a symbol that the linker didn't find, and the .gp
// line that uses it (from the debug information) if the linker names one.
type undefinedRef struct {
	sym string
	at  cxxLoc
}

var (
	// gnuUndefinedRx matches GNU ld's error (gold's too): "/a/main.gp:6:
	// (.text+0xa): undefined reference to `f(int)'", or "main.o:(.text+0x5):
	// undefined reference to `f'" without the debug information.
	gnuUndefinedRx = regexp.MustCompile("undefined reference to [`'‘](.*)['’]$")
	// gnuPlaceRx matches the .gp place at the start of GNU ld's error.
	gnuPlaceRx = regexp.MustCompile(`^(?:\S+: )?(.+\.gp):(\d+):`)
	// lldUndefinedRx matches the error of lld and mold, "ld.lld: error:
	// undefined symbol: f(int)", whose places follow on ">>> referenced
	// by" lines.
	lldUndefinedRx = regexp.MustCompile(`undefined symbol: (.+)$`)
	// lldPlaceRx matches the place of a ">>> referenced by main.gp:6
	// (/a/main.gp:6)" line: the absolute name in parentheses if it is
	// there, else the first.
	lldPlaceRx = regexp.MustCompile(`^>>> referenced by (?:.*\((.+\.gp):(\d+)\)|(.+?\.gp):(\d+))`)
	// gocppSymbolRx matches the symbols of gocpp's runtime (gp::) and of
	// the generated C++ (the namespaces gpkg_<package>).
	gocppSymbolRx = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(?:gp::|gpkg_)`)
)

// parseUndefined reads the undefined symbols in the linker's output out,
// with the .gp lines that use them.
func parseUndefined(out []byte) []undefinedRef {
	var refs []undefinedRef
	last := -1 // lld: the symbol that the ">>> referenced by" lines belong to
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := cleanCompilerLine(sc.Text())
		if m := gnuUndefinedRx.FindStringSubmatch(line); m != nil {
			r := undefinedRef{sym: m[1]}
			if p := gnuPlaceRx.FindStringSubmatch(line); p != nil {
				n, _ := strconv.Atoi(p[2])
				r.at = cxxLoc{p[1], n}
			}
			refs, last = append(refs, r), -1
			continue
		}
		if m := lldUndefinedRx.FindStringSubmatch(line); m != nil {
			refs = append(refs, undefinedRef{sym: strings.TrimSpace(m[1])})
			last = len(refs) - 1
			continue
		}
		if m := lldPlaceRx.FindStringSubmatch(line); m != nil && last >= 0 {
			file, n := m[1], m[2]
			if file == "" {
				file, n = m[3], m[4]
			}
			line, _ := strconv.Atoi(n)
			r := refs[last]
			if r.at.file != "" {
				refs = append(refs, r) // another place of the same symbol
				last = len(refs) - 1
			}
			refs[last].at = cxxLoc{file, line}
			continue
		}
		if !strings.HasPrefix(line, ">>>") {
			last = -1
		}
	}
	return refs
}

// cppLinkFailed reports a link that failed because symbols are missing
// that the user's headers declare (a library or a C/C++ file that the
// program doesn't link), in a package that uses C or C++ or has C/C++
// files or link flags: an error of the program, at the .gp lines that use
// the symbols, with a hint. It returns false, and prints nothing, when the
// link failed for another reason, a missing symbol is gocpp's, or the
// linker names no .gp line for a use.
func (d *driver) cppLinkFailed(pkg *check.Package, e *build.CompileError, env build.EnvFlags) bool {
	if !pkgUsesCpp(pkg) && !hasSources(pkg.Load) && len(pkg.Flags.Link) == 0 && len(env.LDFlags) == 0 {
		return false
	}
	refs := parseUndefined(e.Output)
	if len(refs) == 0 {
		return false
	}
	for _, r := range refs {
		// A use that the linker can't place at a .gp line (the debug
		// information has none) is reported with the linker's own lines.
		if gocppSymbolRx.MatchString(r.sym) || r.at.file == "" {
			return false
		}
	}
	it := "it"
	if len(refs) > 1 {
		it = "them"
	}
	var hints []string
	if d.files && folderHasSources(pkg.Load.Dir) {
		hints = append(hints, fmt.Sprintf("(the folder has C or C++ files, and gocpp %s of named .gp files leaves them out: gocpp %s . builds them too)", d.name, d.name))
	} else {
		hints = append(hints, fmt.Sprintf("(the headers declare %s, but the program links no definition: add the library with //gocpp:link or //gocpp:pkg-config, or the C or C++ file that defines %s to the package)", it, it))
	}
	if len(pkg.Flags.Link) > 0 {
		hints = append(hints, fmt.Sprintf("(the link flags of the package's //gocpp: directives: %s)", strings.Join(pkg.Flags.Link, " ")))
	}
	if len(env.LDFlags) > 0 {
		hints = append(hints, fmt.Sprintf("(GOCPP_LDFLAGS: %s)", strings.Join(env.LDFlags, " ")))
	}
	if hasSources(pkg.Load) {
		hints = append(hints, fmt.Sprintf("(the package's C and C++ files: %s)", strings.Join(slices.Concat(pkg.Load.CFiles, pkg.Load.CXXFiles), " ")))
	}
	if d.flags.cppErrors.full() {
		fmt.Fprintf(d.stderr, "# %s\n%s\n", pkg.Load.ImportPath, strings.TrimRight(d.shortenDir(pkg, string(e.Output)), "\n"))
	} else {
		var errs []cppError
		seen := make(map[undefinedRef]bool)
		for _, r := range refs {
			if !seen[r] {
				seen[r] = true
				errs = append(errs, cppError{at: r.at, msg: fmt.Sprintf("C++: undefined reference to '%s'", r.sym)})
			}
		}
		d.printCppErrors(pkg, errs)
	}
	for _, h := range hints {
		fmt.Fprintf(d.stderr, "\t%s\n", h)
	}
	return true
}

// folderHasSources reports whether the folder dir has C or C++ files
// (.c, .cc, .cpp, .cxx), whatever their build constraints say.
func folderHasSources(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		switch filepath.Ext(e.Name()) {
		case ".c", ".cc", ".cpp", ".cxx":
			if !e.IsDir() {
				return true
			}
		}
	}
	return false
}
