package lower

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// References to the other packages of the program (Phase 06 step 6.2): pkg.Name, a qualified
// identifier of a gocpp package that the Program lowers. Its C++ name is the name that the
// package chose, fully qualified with its namespace (::gpkg_geom::New): the namespaces are flat
// (ADR-010), so no name of the current package can hide it. Constants are folded by go/types
// (expr.go), types are structOf's and ifaceOf's, methods are members of their structs.

// qualifiedObj returns the object that x, pkg.Name, names when pkg is a gocpp package that the
// Program lowers, else nil.
func (g *gen) qualifiedObj(x *ast.SelectorExpr) types.Object {
	id, ok := x.X.(*ast.Ident)
	if !ok {
		return nil
	}
	pn, ok := g.info.Uses[id].(*types.PkgName)
	if !ok || g.foreign(pn.Imported()) == nil {
		return nil
	}
	return g.info.Uses[x.Sel]
}

// qualifiedFunc returns the function that x, pkg.F, names, or nil if it is no function of a
// package of the program.
func (g *gen) qualifiedFunc(x *ast.SelectorExpr) *types.Func {
	fn, _ := g.qualifiedObj(x).(*types.Func)
	return fn
}

// qualified returns the C++ of the qualified identifier pkg.Name that names obj: a package
// variable, or a function as a value.
func (fg *funcGen) qualified(obj types.Object, pos token.Pos) cppir.Expr {
	switch obj := obj.(type) {
	case *types.Var:
		return cxxName(fg.cname(obj, pos))
	case *types.Func:
		return fg.funcRef(obj, pos)
	}
	fg.g.unsupported(pos, "the qualified name "+obj.Name())
	return placeholder
}
