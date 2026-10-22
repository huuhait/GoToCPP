package check

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/diag"
)

// The values of const declarations that are not constants.
//
// go/types has no type for a value that comes from C/C++, and it reports nothing for a constant
// expression that holds one, so `const d = x + 1` with `var x C.int` would pass the check and
// fail in the lowering. Go's own rule is the answer, as for any value that is not a constant:
//
//	./main.gp:10:12: x + 1 (C/C++ value) is not constant
//
// A value is not constant when it is opaque (Opaque) and holds a variable (a local, a package
// variable of this package or of another one, a func value) or a call of a Go function or method. The arguments of
// len, cap and unsafe.Sizeof are not looked into (they can be constant for a variable), and the
// call of a C/C++ name is not judged here: the check does not know whether lib.f(1) calls a
// function or converts to a type (the lowering's kind probe does), and opaque mode has the gate
// of the C names in constant expressions for it (featCConstants).

// notConstants adds the error "(C/C++ value) is not constant" for the value of each const
// declaration of the package that holds a variable or a Go call, and remembers the values
// (Package.notConst) so that the gate of the C names in constant expressions leaves them to it.
// It runs on the result of the opaque pass (op.result), only when go/types has no error that
// could leave types incomplete.
func (op *opaquePass) notConstants(fset *token.FileSet, p *Package) {
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			decl, ok := n.(*ast.GenDecl)
			if !ok || decl.Tok != token.CONST {
				return true
			}
			for _, spec := range decl.Specs {
				for _, v := range spec.(*ast.ValueSpec).Values {
					if !op.result.exprs[v] || !op.notConstant(v) {
						continue
					}
					if p.notConst == nil {
						p.notConst = make(map[ast.Expr]bool)
					}
					p.notConst[v] = true
					p.Errors.Add(diag.Diagnostic{
						Pos: fset.Position(v.Pos()),
						End: fset.Position(v.End()),
						Msg: p.SourceText(v) + " (C/C++ value) is not constant",
					})
				}
			}
			return false
		})
	}
}

// notConstant reports whether the opaque value v of a const declaration holds a variable or a
// call of a Go function.
func (op *opaquePass) notConstant(v ast.Expr) bool {
	bad := false
	ast.Inspect(v, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if obj, ok := op.info.Uses[x].(*types.Var); ok && !obj.IsField() {
				bad = true
			}
		case *ast.CallExpr:
			if op.sizeOnly(x) {
				return false // len(a), cap(a), unsafe.Sizeof(x): the arguments are not evaluated
			}
			if goCall(op.info, x) {
				bad = true
			}
		}
		return !bad
	})
	return bad
}

// sizeOnly reports whether the call evaluates none of its arguments when they are values of a
// type that go/types has no size for: the builtins len, cap, unsafe.Sizeof, Alignof and Offsetof
// (Check rewrote the Sizeof of a C/C++ value to the cgoh helper, unsafeHelperOf).
func (op *opaquePass) sizeOnly(call *ast.CallExpr) bool {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		if b, ok := op.info.Uses[fun].(*types.Builtin); ok {
			return b.Name() == "len" || b.Name() == "cap"
		}
	case *ast.SelectorExpr:
		if unsafeHelperOf(fun, op.info) == "Size" {
			return true
		}
		if b, ok := op.info.Uses[fun.Sel].(*types.Builtin); ok {
			switch b.Name() {
			case "Sizeof", "Alignof", "Offsetof":
				return true
			}
		}
	}
	return false
}

// goCall reports whether the call is the call of a function or a method of Go (a *types.Func), not
// a conversion, a builtin, the call of a C/C++ name or of a func value (the value is a variable,
// which notConstant reports itself).
func goCall(info *types.Info, call *ast.CallExpr) bool {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	default:
		return false
	}
	_, ok := info.Uses[id].(*types.Func)
	return ok
}
