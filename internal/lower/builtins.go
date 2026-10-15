package lower

import (
	"go/ast"

	"github.com/huuhait/gocpp/internal/cppir"
)

// minMax returns the C++ of the builtin min or max (name) whose arguments are not all constants
// (go/types folds a constant call, and fg.expr writes its value): gp::min<T>(x, y, ...), with
// T the C++ type of the result, which is the type of each argument (a constant argument has
// that type too: go/types converts it), so gp::min<int64_t>(x, 1) (gocpp/minmax.hpp has Go's
// rules for NaN and -0). The arguments are ordered like the arguments of a call (order.go).
func (fg *funcGen) minMax(c *ast.CallExpr, name string) cppir.Expr {
	t := fg.g.info.TypeOf(c)
	return templateCall("gp::"+name, []cppir.Node{fg.g.cxxType(t, c.Pos())}, fg.exprs(c.Args, false)...)
}
