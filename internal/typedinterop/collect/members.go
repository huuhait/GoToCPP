package collect

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/check"
)

// Promoted members of embedded C++ classes (step 12.9, ADR-098).
//
//	type S struct{ std.vector[int] }
//	s.push_back(1)     the member call on s.vector: sel(arg&(std::vector<int64_t>)|push_back)
//
// A selector that the checker took as a member of an embedded C++ class (check.Package.PromotedPath,
// PromotedField) is a member of that class's value: the object is an lvalue of the field's C++
// type (std::declval<std::vector<int64_t>&>()), as for a variable of that type. The class itself
// is a reference too (the type of the embedded field), found under the selector's name node
// (Set.RefOf(sel.Sel)): the typed lowering reads the kind of the object (a class, a pointer to
// one) from it.

// classIR returns the C++ type of the embedded field whose type expression is e: the expression
// itself, the C++ type behind a named type or alias declared over one (type Vec std.vector[int]),
// and a pointer to either.
func (c *collector) classIR(e ast.Expr, depth int) Expr {
	if depth > 8 {
		return &Unknown{Why: "type declarations nested too deep"}
	}
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		if tn, ok := c.info.Uses[x].(*types.TypeName); ok {
			if te := c.op.TypeExpr(tn); te != nil {
				return c.classIR(te, depth+1)
			}
		}
	case *ast.StarExpr:
		return &Star{X: c.classIR(x.X, depth+1)}
	}
	return c.typeIR(e)
}

// promotedReceiver returns the object of the member access sel when the checker took sel as a
// member of an embedded C++ class: an lvalue of the embedded field's C++ type. nil when sel is
// not such a selector. A class whose type expression is not in this package (the struct is
// declared in another package) gives an Unknown part: the reference is listed and not probed.
func (c *collector) promotedReceiver(sel *ast.SelectorExpr) Expr {
	f := c.p.PromotedField(sel)
	if f == nil {
		return nil
	}
	cls := c.classOfField(f)
	c.registerClass(sel.Sel, cls)
	// The object is a place unless the struct is a temporary (f().size()).
	lv := c.isLValue(sel.X)
	if tv, ok := c.info.Types[sel.X]; ok && tv.Type != nil {
		if _, ptr := types.Unalias(tv.Type).Underlying().(*types.Pointer); ptr {
			lv = true
		}
	}
	return &Arg{T: cls, LValue: lv}
}

// classOfField returns the C++ type of the embedded field f: its type expression, or, when the field's
// type is a type of this package declared over a C++ type (type Named std.string), that declaration.
func (c *collector) classOfField(f *types.Var) Expr {
	if te := c.op.TypeExpr(f.Origin()); te != nil {
		return c.classIR(te, 0)
	}
	t, star := types.Unalias(f.Type()), false
	if p, ok := t.(*types.Pointer); ok {
		t, star = types.Unalias(p.Elem()), true
	}
	if n, ok := t.(*types.Named); ok {
		if te := c.op.TypeExpr(n.Obj()); te != nil {
			x := c.classIR(te, 0)
			if star {
				x = &Star{X: x}
			}
			return x
		}
	}
	return &Unknown{Why: "embedded C++ field " + f.Name() + " of another package"}
}

// registerClass makes (or finds) the reference of the C++ class x and lets the node n stand for it.
func (c *collector) registerClass(n ast.Node, x Expr) {
	if isUnknown(x) {
		return
	}
	key := x.String()
	r := c.set.byKey[key]
	if r == nil {
		r = &Ref{Kind: kindOf(x), Key: key, Expr: x, Standalone: true}
		r.Incomplete = incompleteOf(x)
		c.set.byKey[key] = r
		c.set.Refs = append(c.set.Refs, r)
	}
	c.set.byNode[n] = r
}

// MemberSet makes the set of references that answer the queries of check's second pass for
// promoted members: for each query the member of an lvalue of the class, X.Name, as a standalone
// reference (decltype((declval<C&>().Name)): the compiler says what the name is by what it says
// about the access), and for each class an access to operator-> (a class that has one and no member
// of the name is not decided). The result is the set, and the references of the queries (nil where
// the class has no C++ spelling) and of the arrows, by query index. The set is probed as any
// other.
func MemberSet(p *check.Package, fset *token.FileSet, qs []check.MemberQuery) (set *Set, member, arrow []*Ref) {
	c := newCollector(p, fset)
	if c == nil {
		return NewSet(), make([]*Ref, len(qs)), make([]*Ref, len(qs))
	}
	var refs []*Ref
	member, arrow = make([]*Ref, len(qs)), make([]*Ref, len(qs))
	classes := make(map[ast.Expr]Expr)
	for i, q := range qs {
		cls, ok := classes[q.Class]
		if !ok {
			cls = c.classIR(q.Class, 0)
			classes[q.Class] = cls
		}
		if isUnknown(cls) {
			continue
		}
		obj := &Arg{T: cls, LValue: true}
		member[i] = &Ref{Expr: &Select{X: obj, Name: q.Name}}
		arrow[i] = &Ref{Expr: &Select{X: obj, Name: "operator->"}}
		refs = append(refs, member[i], arrow[i])
	}
	set = NewSet(refs...)
	for i := range qs {
		if member[i] != nil {
			member[i] = set.Ref(member[i].Expr.String())
			arrow[i] = set.Ref(arrow[i].Expr.String())
		}
	}
	return set, member, arrow
}
