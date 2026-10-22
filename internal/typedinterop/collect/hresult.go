package collect

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/check"
)

// KeepsHResult reports whether the program does not take the result of the call x, one of a function
// that returns an HRESULT, as an error (ADR-137): it drops it (a statement, defer or go) or converts
// it to a number (int32(f()), C.long(f())), which is how it gets the HRESULT itself, with its success
// codes. parentOf gives the parent of a node of the package p.
func KeepsHResult(p *check.Package, parentOf func(ast.Node) ast.Node, x *ast.CallExpr) bool {
	var n ast.Node = x
	parent := parentOf(n)
	for {
		paren, ok := parent.(*ast.ParenExpr)
		if !ok {
			break
		}
		n = paren
		parent = parentOf(n)
	}
	switch q := parent.(type) {
	case *ast.ExprStmt, *ast.DeferStmt, *ast.GoStmt:
		return true
	case *ast.AssignStmt:
		// _ = f() drops it on purpose, as a statement does
		return q.Tok == token.ASSIGN && len(q.Lhs) == 1 && len(q.Rhs) == 1 && ast.Unparen(q.Rhs[0]) == ast.Unparen(n.(ast.Expr)) && isBlankIdent(q.Lhs[0])
	case *ast.CallExpr:
		if len(q.Args) != 1 || ast.Unparen(q.Args[0]) != ast.Unparen(n.(ast.Expr)) {
			return false
		}
		if cv := p.Conversion(q); cv != nil {
			return cv.Kind == check.AsCall && numberType(p, cv.Type)
		}
		if tv, ok := p.Info.Types[q.Fun]; ok && tv.IsType() {
			return numberType(p, q.Fun) // int32(f()), before the rewrite of the conversions
		}
		// C.long(f()) is the call of a C name that the checker leaves as it is.
		return cNumberType(p, q.Fun)
	}
	return false
}

func isBlankIdent(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// cNumbers are the arithmetic types of cgo's C package.
var cNumbers = map[string]bool{
	"char": true, "schar": true, "uchar": true, "short": true, "ushort": true, "int": true, "uint": true,
	"long": true, "ulong": true, "longlong": true, "ulonglong": true, "float": true, "double": true,
	"size_t": true, "ssize_t": true, "intptr_t": true, "uintptr_t": true, "int8_t": true, "uint8_t": true,
	"int16_t": true, "uint16_t": true, "int32_t": true, "uint32_t": true, "int64_t": true, "uint64_t": true,
	"wchar_t": true, "ptrdiff_t": true,
}

// cNumberType reports whether e is C.long, C.uint and the like.
func cNumberType(p *check.Package, e ast.Expr) bool {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	return ok && p.IsC(sel.X) && cNumbers[sel.Sel.Name]
}

// numberType reports whether the type expression t is a number of Go or a type of C/C++: what a
// program converts an HRESULT to when it wants the code.
func numberType(p *check.Package, t ast.Expr) bool {
	if tv, ok := p.Info.Types[t]; ok && tv.Type != nil && complete(tv.Type) {
		if b, ok := types.Unalias(tv.Type).Underlying().(*types.Basic); ok {
			return b.Info()&types.IsNumeric != 0 && b.Info()&types.IsComplex == 0
		}
	}
	return p.Opaque.IsOpaque(t)
}

// ParentsOf returns the parent of each node of the files of the package p.
func ParentsOf(p *check.Package) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	for _, f := range p.Load.Files {
		var stack []ast.Node
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				parents[n] = stack[len(stack)-1]
			}
			stack = append(stack, n)
			return true
		})
	}
	return parents
}
