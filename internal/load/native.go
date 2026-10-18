package load

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
)

// Native functions (Phase 05 step 5.7). A function of a standard package that is written in C++
// has no body in the .gp file and a doc comment line
//
//	//gocpp:native gp::errors::assign
//	func assign(err error, target any) bool
//
// which names the C++ function that the calls become: a function of the runtime (declared in a
// header that gocpp/rt.hpp includes), called with the arguments in their C++ types, as the
// package's other functions are. It is the one //gocpp: directive that comes after the package
// clause, because it belongs to a function. A function without a body and without the directive
// is a declaration stub (stdlib/README.md): it type-checks and cannot run.
//
// Only the standard library has native functions (the loader reports the directive anywhere
// else), and only plain functions: no methods, no generic functions.

// nativePrefix starts the directive.
const nativePrefix = "//gocpp:native"

// cxxQualified is a C++ function name the directive may name: qualified names of the runtime,
// optionally starting at the global namespace.
var cxxQualified = regexp.MustCompile(`^(::)?[A-Za-z_][A-Za-z0-9_]*(::[A-Za-z_][A-Za-z0-9_]*)*$`)

// isNativeComment reports whether the comment is a //gocpp:native line (the name is exactly
// "native", so //gocpp:nativex is an unknown directive).
func isNativeComment(c *ast.Comment) bool {
	rest, ok := strings.CutPrefix(c.Text, nativePrefix)
	return ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t')
}

// NativeFunc returns the C++ name that the //gocpp:native directive in the doc comment of fd
// gives, and whether fd is a valid native function: a plain function without a body and with
// one directive that names a C++ function. (ParseDirectives reports the invalid ones; the
// callers get the result of a package without errors.)
func NativeFunc(fd *ast.FuncDecl) (cxx string, ok bool) {
	cxx, msg, _, found := nativeOf(fd)
	return cxx, found && msg == ""
}

// nativeOf finds the directive in the doc comment of fd. found says whether there is one; msg
// is the error to report, at the position at, if the directive or the function is not right.
func nativeOf(fd *ast.FuncDecl) (cxx, msg string, at token.Pos, found bool) {
	if fd.Doc == nil {
		return "", "", token.NoPos, false
	}
	for _, c := range fd.Doc.List {
		if !isNativeComment(c) {
			continue
		}
		at = c.Slash
		if found {
			return "", "more than one //gocpp:native directive on a function", at, true
		}
		found = true
		args := strings.Fields(strings.TrimPrefix(c.Text, nativePrefix))
		if i := slicesIndexComment(args); i >= 0 {
			args = args[:i] // "// comment" at the end of the line, as the other directives allow
		}
		if len(args) != 1 {
			return "", "usage: //gocpp:native C++function", at, true
		}
		if !cxxQualified.MatchString(args[0]) {
			return "", "//gocpp:native: " + args[0] + " is not the name of a C++ function (like gp::errors::assign)", at, true
		}
		cxx = args[0]
	}
	switch {
	case !found:
	case fd.Body != nil:
		return "", "//gocpp:native is for a function without a body", at, true
	case fd.Recv != nil:
		return "", "//gocpp:native is for a function, not a method", at, true
	case fd.Type.TypeParams != nil:
		return "", "//gocpp:native is for a function that is not generic", at, true
	}
	return cxx, "", at, found
}

// slicesIndexComment returns the index of the first argument that starts a "//" comment, or -1.
func slicesIndexComment(args []string) int {
	for i, a := range args {
		if strings.HasPrefix(a, "//") {
			return i
		}
	}
	return -1
}

// nativeComments returns the //gocpp:native comments of f, which come after the package clause.
func nativeComments(f *ast.File) []*ast.Comment {
	var out []*ast.Comment
	for _, g := range f.Comments {
		for _, c := range g.List {
			if c.Slash > f.Package && isNativeComment(c) {
				out = append(out, c)
			}
		}
	}
	return out
}

// checkNative reports the //gocpp:native comments of f that are not valid: one that is not in
// the doc comment of a function, and the mistakes nativeOf finds.
func checkNative(fset *token.FileSet, f *ast.File, errs *diag.List) {
	cs := nativeComments(f)
	if len(cs) == 0 {
		return
	}
	owned := make(map[*ast.Comment]bool)
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		_, msg, at, found := nativeOf(fd)
		if !found {
			continue
		}
		for _, c := range fd.Doc.List {
			if isNativeComment(c) {
				owned[c] = true
			}
		}
		if msg != "" {
			errs.Errorf(fset.Position(at), "%s", msg)
		}
	}
	for _, c := range cs {
		if !owned[c] {
			errs.Errorf(fset.Position(c.Slash), "//gocpp:native must be in the doc comment of a function")
		}
	}
}

// checkNativeStd reports the native functions of a package that is not part of the standard
// library.
func checkNativeStd(fset *token.FileSet, f *ast.File, errs *diag.List) {
	for _, c := range nativeComments(f) {
		errs.Errorf(fset.Position(c.Slash), "//gocpp:native is only for the standard library")
	}
}

// IsStub reports whether p is a declaration stub: a standard package with a function without a
// body that is not native (stdlib/README.md). A stub type-checks and is never lowered; a package
// whose bodyless functions are all native is a real package.
func (p *Package) IsStub() bool {
	if !p.Std {
		return false
	}
	for _, f := range p.Files {
		if f.AST == nil {
			continue
		}
		for _, d := range f.AST.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body == nil {
				if _, native := NativeFunc(fd); !native {
					return true
				}
			}
		}
	}
	return false
}
