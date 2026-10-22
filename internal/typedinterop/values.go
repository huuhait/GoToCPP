package typedinterop

import (
	"go/ast"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Typed returns what the probe says about the expression e (the node the collector made a
// reference of) and the probe's result, or nil: not probed, failed, recovered (clang printed an
// error for it), or opaque in the foreign view. The lowering, the interop checker and the
// language server all read the typed view through it.
func (r *Report) Typed(e ast.Expr) (*ftype.Value, *probe.Result) {
	if r == nil || r.Set == nil || r.Table == nil {
		return nil, nil
	}
	ref := r.Set.RefOf(e)
	if ref == nil {
		if p, ok := e.(*ast.ParenExpr); ok {
			return r.Typed(p.X)
		}
		return nil, nil
	}
	res := r.Table.Result(ref.ID)
	if res == nil || !res.OK || res.Recovered != "" || ref.ID >= len(r.Values) {
		return nil, nil
	}
	v := r.Values[ref.ID]
	if v.Kind == ftype.Opaque || v.C == nil {
		return nil, nil
	}
	return &v, res
}

// MemberObject returns the expression whose typed view is the object of the member access sel:
// sel.X, or, for a member that a struct takes from an embedded C++ class (check/promoted.go,
// cppmembers.go), the name sel.Sel, which the collector made the reference of the embedded field's
// class of (collect/members.go). The C++ of the object is the path of the embedded fields, so its
// type is that of the field, not sel.X's.
func (r *Report) MemberObject(p *check.Package, sel *ast.SelectorExpr) ast.Expr {
	if r != nil && r.Set != nil && p.PromotedPath(sel) != nil && r.Set.RefOf(sel.Sel) != nil {
		return sel.Sel
	}
	return sel.X
}
