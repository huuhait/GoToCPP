// Package gpast says how gocpp's Phase 10 syntax (parser patches 4 to 6,
// FORK.md §4) looks in a stock go/ast tree, and has the functions to make
// and to recognize each form. The parser makes these nodes, the printer
// (internal/syntax/printer) prints them, and the passes of Phase 10 (the
// must and macro rewrites) read them. Nobody else should know the details.
//
// go/ast has no node for the new syntax, and its node types can't be
// extended, so each form is a stock node with a mark that Go code can't
// have:
//
//	F!(x), os.ReadFile!(p), parse![int](s)
//		A call. The name of the function (or the method after the dot) is
//		an *ast.Ident whose Name ends with "!": "F!", "ReadFile!". The
//		Ident's position and length are those of the source text, so
//		End() is right. The name must be the called function itself, not
//		the start of a longer selector: F![int].x(1) and a.F![int].x(1) are
//		errors. See IsMust.
//
//	func f!() T, func (s *S) Start!() { ... }
//		A *ast.FuncDecl whose Name ends with "!": "f!", "Start!".
//
//	#Name(...), #util.Dbg(x), #func(), #body()
//		A call whose first name is an *ast.Ident that starts with "#":
//		"#Name", "#util", "#func" (the # is in the source, so End() is
//		right). A macro use is always a call, and the name is the start of
//		the selector that is called: #a.b.c(x) and #a[int](x) are fine,
//		#a[int].b(x) is an error. See IsMacroUse.
//
//	macro Name(params) { ... }
//		A package-level *ast.FuncDecl whose Name starts with "#":
//		"#Name", so a use and its declaration have the same name.
//		FuncType.Func is the position of the word macro, so
//		FuncDecl.Pos() is too. The # is not in the source of the
//		declaration (macro Name), so Name.NamePos is the position of Name
//		and Name.End() is one too far: use DeclNameEnd. See IsMacroDecl.
//
//	macro M(msg lazy string)
//		The type of the parameter is an *ast.UnaryExpr with the Op
//		token.ILLEGAL (a mark that never comes from the scanner): its X is
//		the type string, and its OpPos is the position of the word lazy.
//		See Lazy.
//
//	#Trace()
//	func f() {}
//		A line of macros on top of a package-level func or type is a
//		*ast.GenDecl with the Tok token.ILLEGAL and one *ast.ValueSpec
//		with the one name _ (at the position of the #: a ValueSpec
//		without a name can't tell its Pos) and the call as its only value,
//		in File.Decls right before the func or type (or before the next
//		such line). See OnTop.
//		The doc comment of the declaration is above these lines, and the
//		parser makes it the Doc of the func (a type's Doc is in its
//		GenDecl, which is the one after the lines, as always).
//
// The stock go/types never sees these nodes: the rewrites of Phase 10 turn
// them into plain Go first. Things to know, because the tree is stock go/ast
// with marks:
//   - go/types does NOT reject a form that a pass forgot: it does not report
//     "undefined" for a name with a ! or a # (isValidName), so F!(x) and
//     #M(x) type-check silently; a lazy parameter gives "ILLEGALstring is not
//     a type"; an on-top line gives "invalid token ILLEGAL". A pass that
//     rewrites forms must call Leftovers on the tree before go/types and
//     fail if it finds any.
//   - ast.IsExported("#Assert") is false, so go/doc and ast.FilterFile treat
//     every macro as unexported. Code that needs the name of a macro (a
//     summary of a package, the exported check) must use MacroBase.
//   - A macro Name and a func Name don't clash in the tree (#Name and Name),
//     and neither do f! and f: the passes must report the redeclaration.
//   - The position of a macro declaration's name needs DeclNameEnd.
package gpast

import (
	"go/ast"
	"go/token"
	"strings"
)

// DeclNameEnd returns the end of the name of the func or macro declaration d
// in the source. For a macro, d.Name.End() is one too far: the name is
// "#Name" in the tree and "Name" in the source.
func DeclNameEnd(d *ast.FuncDecl) token.Pos {
	if IsMacroDecl(d) {
		return d.Name.NamePos + token.Pos(len(d.Name.Name)-1)
	}
	return d.Name.End()
}

// IsMust reports whether name is a name with a !, as the scanner makes it
// for F!( and F![: F!, f!.
func IsMust(name string) bool { return len(name) > 1 && strings.HasSuffix(name, "!") }

// MustBase returns the name without its !, and name if it has none.
func MustBase(name string) string {
	if IsMust(name) {
		return name[:len(name)-1]
	}
	return name
}

// IsMacroUse reports whether name is a name with a #, as the scanner makes
// it for #Name: #Assert, #func.
func IsMacroUse(name string) bool { return len(name) > 1 && strings.HasPrefix(name, "#") }

// MacroBase returns the name without its #, and name if it has none.
func MacroBase(name string) string {
	if IsMacroUse(name) {
		return name[1:]
	}
	return name
}

// IsMacroDecl reports whether d is a macro declaration (macro Name(...)),
// not a func.
func IsMacroDecl(d *ast.FuncDecl) bool { return d.Name != nil && IsMacroUse(d.Name.Name) }

// NewLazy returns the type of a lazy parameter, lazy typ, with the word
// lazy at pos.
func NewLazy(pos token.Pos, typ ast.Expr) *ast.UnaryExpr {
	return &ast.UnaryExpr{OpPos: pos, Op: token.ILLEGAL, X: typ}
}

// Lazy reports whether e is the type of a lazy parameter, and returns the
// type after the word lazy.
func Lazy(e ast.Expr) (typ ast.Expr, ok bool) {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.ILLEGAL {
		return u.X, true
	}
	return nil, false
}

// NewOnTop returns the declaration for the line of macros call, which
// starts at pos (the #).
func NewOnTop(pos token.Pos, call *ast.CallExpr) *ast.GenDecl {
	return &ast.GenDecl{
		TokPos: pos,
		Tok:    token.ILLEGAL,
		Specs: []ast.Spec{&ast.ValueSpec{
			Names:  []*ast.Ident{{NamePos: pos, Name: "_"}},
			Values: []ast.Expr{call},
		}},
	}
}

// OnTop returns the macro call of d if d is a line of macros on top of a
// declaration.
func OnTop(d ast.Decl) (call *ast.CallExpr, ok bool) {
	g, ok := d.(*ast.GenDecl)
	if !ok || g.Tok != token.ILLEGAL || len(g.Specs) != 1 {
		return nil, false
	}
	vs, ok := g.Specs[0].(*ast.ValueSpec)
	if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "_" || len(vs.Values) != 1 {
		return nil, false
	}
	call, ok = vs.Values[0].(*ast.CallExpr)
	return call, ok
}

// OnTopOf returns the macro calls on top of decls[i], in source order: the
// lines of macros right before it. It returns nil for a declaration with
// none, and for a line of macros itself.
func OnTopOf(decls []ast.Decl, i int) []*ast.CallExpr {
	if _, ok := OnTop(decls[i]); ok {
		return nil
	}
	start := i
	for start > 0 {
		if _, ok := OnTop(decls[start-1]); !ok {
			break
		}
		start--
	}
	calls := make([]*ast.CallExpr, 0, i-start)
	for _, d := range decls[start:i] {
		call, _ := OnTop(d)
		calls = append(calls, call)
	}
	if len(calls) == 0 {
		return nil
	}
	return calls
}

// Leftovers returns the nodes of the Phase 10 syntax in the tree n, in
// source order: a name with a ! or a # (an *ast.Ident), the type of a lazy
// parameter (an *ast.UnaryExpr), a line of macros on top (an *ast.GenDecl) and a
// macro declaration (an *ast.FuncDecl; its name is not reported again). The
// rewrites of Phase 10 turn each of them into plain Go, so a tree that goes
// to go/types must have none: go/types doesn't report most of them (see the
// package comment). A line of macros is reported as one node; what is in
// its call is not looked at.
func Leftovers(n ast.Node) []ast.Node {
	var found []ast.Node
	skip := make(map[*ast.Ident]bool)
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if !skip[n] && (IsMust(n.Name) || IsMacroUse(n.Name)) {
				found = append(found, n)
			}
		case *ast.UnaryExpr:
			if _, ok := Lazy(n); ok {
				found = append(found, n)
			}
		case *ast.GenDecl:
			if _, ok := OnTop(n); ok {
				found = append(found, n)
				return false
			}
		case *ast.FuncDecl:
			if IsMacroDecl(n) {
				skip[n.Name] = true
				found = append(found, n)
			}
		}
		return true
	})
	return found
}
