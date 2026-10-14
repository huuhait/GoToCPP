package check

import (
	"go/ast"
	"go/token"
	"go/types"
)

// checkPrintSlices reports each slice argument of the builtins print and println (divergence
// D23 in docs/spec/language.md): Go prints a slice there as its length, its capacity and the
// address of its array, [3/4]0xc000012345, an address of Go's heap that gocpp can't give, so
// gocpp rejects it for good and suggests fmt.Println, which prints the elements.
//
// It also reports an array or a struct argument, which Go's compiler rejects too ("illegal
// types for operand: print"; go/types accepts any type there).
func checkPrintSlices(fset *token.FileSet, pkg *types.Package, files []*ast.File, info *types.Info,
	report func(pos token.Pos, err error)) {
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := ast.Unparen(call.Fun).(*ast.Ident)
			if !ok {
				return true
			}
			b, ok := info.Uses[id].(*types.Builtin)
			if !ok || b.Name() != "print" && b.Name() != "println" {
				return true
			}
			for _, a := range call.Args {
				t := info.TypeOf(a)
				if t == nil {
					continue
				}
				switch t.Underlying().(type) {
				case *types.Array, *types.Struct:
					report(a.Pos(), types.Error{Fset: fset, Pos: a.Pos(), Msg: "illegal types for operand: " + b.Name() + " " +
						types.ExprString(a) + " (" + types.TypeString(t, types.RelativeTo(pkg)) + "): " + b.Name() +
						" doesn't take arrays or structs, in Go too; use fmt.Println"})
					continue
				case *types.Slice:
				default:
					continue
				}
				report(a.Pos(), types.Error{Fset: fset, Pos: a.Pos(), Msg: "cannot " + b.Name() + " " +
					types.ExprString(a) + " (" + types.TypeString(t, types.RelativeTo(pkg)) + "): " + b.Name() +
					" prints a slice as [len/cap]0x<address>, which gocpp can't print; use fmt.Println"})
			}
			return true
		})
	}
}
