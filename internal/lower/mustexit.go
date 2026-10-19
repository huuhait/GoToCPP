package lower

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/load"
)

// mustExit returns the C++ of gp_must.Exit(err), the call that internal/check's lowering of
// func main!() and func init!() puts in the real main and init: gp::must_exit(err), which
// prints "<program>: <error>" and ends the program with the code 1 when err is not nil
// (runtime/include/gocpp/main.hpp). It is nil for any other call.
func (fg *funcGen) mustExit(c *ast.CallExpr) cppir.Expr {
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok || len(c.Args) != 1 || sel.Sel.Name != "Exit" {
		return nil
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}
	pn, ok := fg.g.info.Uses[id].(*types.PkgName)
	if !ok || pn.Imported().Path() != load.MustPath {
		return nil
	}
	return call("gp::must_exit", fg.expr(c.Args[0]))
}
