package load

import (
	"go/ast"
	"go/token"
	"regexp"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
)

// Extern functions (ADR-128). A function that is defined in C or C++ but declared by no header
// that the package imports (Dear ImGui's imgui_impl_win32.h keeps ImGui_ImplWin32_WndProcHandler
// inside "#if 0") is declared in the .gp file, as Go declares a function that assembly defines:
// with no body, and a doc comment line
//
//	//gocpp:extern [C|C++] [name]
//	func ImGui_ImplWin32_WndProcHandler(hwnd windows.HWND, msg windows.UINT,
//	    wparam windows.WPARAM, lparam windows.LPARAM) windows.LRESULT
//
// The arguments are the linkage ("C" is extern "C"; "C++", the default, is C++ linkage) and the
// C++ name when it differs from the Go name (a qualified name, ImGui::Foo, for a function of a
// namespace; a C function has a plain name). The generated header declares the function in the
// global namespace, and every call of it is a call of that C++ function, as a call through a
// header package is. Only plain functions are extern functions: no methods, no generic
// functions, no variadic ones. The types of the signature are checked by internal/check
// (externfuncs.go).
//
// It is the second //gocpp: directive that comes after the package clause, because it belongs
// to a function (the first is //gocpp:native of the standard library, native.go).

// externPrefix starts the directive.
const externPrefix = "//gocpp:extern"

// An ExternDecl is the meaning of the //gocpp:extern directive of one function.
type ExternDecl struct {
	// CLinkage says that the function is declared extern "C".
	CLinkage bool

	// Name is the C++ name without a leading "::": the Go name of the function, or the name
	// that the directive gives (ImGui::Foo).
	Name string
}

// Qualified returns the C++ name as it is written in a call: ::Name.
func (e ExternDecl) Qualified() string { return "::" + e.Name }

// externPlainName is the name of a C function, and externQualified a name that C++ linkage
// allows: names joined with ::, optionally from the global namespace.
var (
	externPlainName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	externQualified = regexp.MustCompile(`^(::)?[A-Za-z_][A-Za-z0-9_]*(::[A-Za-z_][A-Za-z0-9_]*)*$`)
)

// CxxKeywords are the keywords and alternative tokens of C++20. No C or C++ function has such a
// name, and an extern name is spelled as written, so it is an error (a Go function that has one
// of these names gets a _ suffix in C++, ADR-010, which a C++ function does not have).
var CxxKeywords = []string{
	"alignas", "alignof", "and", "and_eq", "asm", "auto", "bitand", "bitor", "bool", "break",
	"case", "catch", "char", "char16_t", "char32_t", "char8_t", "class", "co_await", "co_return",
	"co_yield", "compl", "concept", "const", "const_cast", "consteval", "constexpr", "constinit",
	"continue", "decltype", "default", "delete", "do", "double", "dynamic_cast", "else", "enum",
	"explicit", "export", "extern", "false", "float", "for", "friend", "goto", "if", "inline",
	"int", "long", "mutable", "namespace", "new", "noexcept", "not", "not_eq", "nullptr",
	"operator", "or", "or_eq", "private", "protected", "public", "register", "reinterpret_cast",
	"requires", "return", "short", "signed", "sizeof", "static", "static_assert", "static_cast",
	"struct", "switch", "template", "this", "thread_local", "throw", "true", "try", "typedef",
	"typeid", "typename", "union", "unsigned", "using", "virtual", "void", "volatile", "wchar_t",
	"while", "xor", "xor_eq",
}

// externNameProblem returns why the C++ name (without a leading ::) can not be the name of an
// extern function, or "" if it can: a segment that is a C++ keyword, or a namespace that holds
// the generated code (gp, std, gpkg_...) and so is not where a user's function is declared.
func externNameProblem(name string) string {
	segments := strings.Split(name, "::")
	for _, segment := range segments {
		if slices.Contains(CxxKeywords, segment) {
			return segment + " is a C++ keyword, so no C or C++ function has that name (an extern name " +
				"is spelled as written: a Go function with such a name gets a _ suffix, which a C++ " +
				"function does not have)"
		}
	}
	first := segments[0]
	if len(segments) > 1 && (first == "gp" || first == "std" || strings.HasPrefix(first, "gpkg_")) {
		return name + " is in the namespace " + first + " of gocpp's own code " +
			"(a function of a namespace of your own is declared there)"
	}
	return ""
}

// isExternComment reports whether the comment is a //gocpp:extern line (the name is exactly
// "extern": //gocpp:extern-c is another directive, and //gocpp:externx is unknown).
func isExternComment(c *ast.Comment) bool {
	rest, ok := strings.CutPrefix(c.Text, externPrefix)
	return ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t')
}

// ExternFunc returns the meaning of the //gocpp:extern directive in the doc comment of fd, and
// whether fd is a valid extern function: a plain function without a body and with one
// directive. (ParseDirectives reports the invalid ones; the callers get the result of a package
// without errors.)
func ExternFunc(fd *ast.FuncDecl) (ExternDecl, bool) {
	decl, msg, _, found := externOf(fd)
	return decl, found && msg == ""
}

// HasExternDirective reports whether the doc comment of fd has a //gocpp:extern line, valid or
// not.
func HasExternDirective(fd *ast.FuncDecl) bool {
	_, _, _, found := externOf(fd)
	return found
}

// externOf finds the directive in the doc comment of fd. found says whether there is one; msg
// is the error to report, at the position at, if the directive or the function is not right.
func externOf(fd *ast.FuncDecl) (decl ExternDecl, msg string, at token.Pos, found bool) {
	if fd.Doc == nil {
		return ExternDecl{}, "", token.NoPos, false
	}
	var args []string
	for _, c := range fd.Doc.List {
		if !isExternComment(c) {
			continue
		}
		at = c.Slash
		if found {
			return ExternDecl{}, "more than one //gocpp:extern directive on a function", at, true
		}
		found = true
		args = strings.Fields(strings.TrimPrefix(c.Text, externPrefix))
		if i := slicesIndexComment(args); i >= 0 {
			args = args[:i] // "// comment" at the end of the line, as the other directives allow
		}
	}
	if !found {
		return ExternDecl{}, "", token.NoPos, false
	}
	decl.Name = fd.Name.Name
	const usage = "usage: //gocpp:extern [C|C++] [name]"
	if len(args) > 0 && (args[0] == "C" || args[0] == "C++") {
		decl.CLinkage = args[0] == "C"
		args = args[1:]
	}
	switch {
	case len(args) > 1:
		return ExternDecl{}, usage, at, true
	case len(args) == 1:
		decl.Name = strings.TrimPrefix(args[0], "::")
		switch {
		case decl.CLinkage && !externPlainName.MatchString(args[0]):
			msg := "//gocpp:extern C: " + args[0] +
				" is not the name of a C function (a plain name: C has no namespaces)"
			return ExternDecl{}, msg, at, true
		case !externQualified.MatchString(args[0]):
			msg := "//gocpp:extern: " + args[0] +
				" is not the name of a C++ function (like add or ImGui::Begin)"
			return ExternDecl{}, msg, at, true
		}
	}
	if why := externNameProblem(decl.Name); why != "" {
		return ExternDecl{}, "//gocpp:extern: " + why, at, true
	}
	switch {
	case fd.Body != nil:
		return ExternDecl{}, "//gocpp:extern is for a function without a body", at, true
	case fd.Recv != nil:
		return ExternDecl{}, "//gocpp:extern is for a function, not a method", at, true
	case fd.Type.TypeParams != nil:
		return ExternDecl{}, "//gocpp:extern is for a function that is not generic", at, true
	case fd.Name.Name == "_" || fd.Name.Name == "init" || fd.Name.Name == "main":
		msg := "//gocpp:extern: a function named " + fd.Name.Name + " can not be extern"
		return ExternDecl{}, msg, at, true
	}
	return decl, "", at, true
}

// externComments returns the //gocpp:extern comments of f, which come after the package
// clause.
func externComments(f *ast.File) []*ast.Comment {
	var out []*ast.Comment
	for _, g := range f.Comments {
		for _, c := range g.List {
			if c.Slash > f.Package && isExternComment(c) {
				out = append(out, c)
			}
		}
	}
	return out
}

// checkExtern reports the //gocpp:extern comments of f that are not valid: one that is not in
// the doc comment of a function, and the mistakes externOf finds.
func checkExtern(fset *token.FileSet, f *ast.File, errs *diag.List) {
	cs := externComments(f)
	if len(cs) == 0 {
		return
	}
	owned := make(map[*ast.Comment]bool)
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		_, msg, at, found := externOf(fd)
		if !found {
			continue
		}
		for _, c := range fd.Doc.List {
			if isExternComment(c) {
				owned[c] = true
			}
		}
		if msg != "" {
			errs.Errorf(fset.Position(at), "%s", msg)
		}
	}
	for _, c := range cs {
		if !owned[c] {
			pos := fset.Position(c.Slash)
			errs.Errorf(pos, "//gocpp:extern must be in the doc comment of a function")
		}
	}
}

// checkExternStd reports the extern functions of a standard package: the standard library
// calls the runtime with //gocpp:native.
func checkExternStd(fset *token.FileSet, f *ast.File, errs *diag.List) {
	for _, c := range externComments(f) {
		pos := fset.Position(c.Slash)
		errs.Errorf(pos, "//gocpp:extern is not for the standard library (it uses //gocpp:native)")
	}
}

// ExternNames returns the names of the valid extern functions of f, in source order.
func ExternNames(f *ast.File) []string {
	var names []string
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body == nil && fd.Doc != nil {
			if _, ok := ExternFunc(fd); ok {
				names = append(names, fd.Name.Name)
			}
		}
	}
	return names
}

// IsBindgenView reports whether f is a view that gocpp bindgen wrote (ADR-092): a description of a
// header, with no bodies, that gocpp check accepts and nothing builds.
func IsBindgenView(f *ast.File) bool {
	for _, group := range f.Comments {
		if group.End() > f.Package {
			break
		}
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "// Code generated by gocpp bindgen from ") {
				return true
			}
		}
	}
	return false
}
