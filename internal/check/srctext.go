package check

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/load"
)

// SourceText returns e as the user wrote it: std.vector, cstdio.printf, C.cstr, not the C.std.vector
// and C.printf of the loader's rewrite or the gp_cgoh.Cstr of check's rewrite. The uses of header
// packages and namespaces and the cgo helpers get their names back while it prints. It does not
// change the AST (the language server calls it while other requests read the same tree).
func (p *Package) SourceText(e ast.Expr) string {
	p.sourceUsesOnce.Do(p.collectSourceUses)
	return p.UserText(types.ExprString(p.restoreNames(e)))
}

// collectSourceUses finds the selectors of the loader's rewrite, by the selector.
func (p *Package) collectSourceUses() {
	p.sourceUses = make(map[*ast.SelectorExpr]*load.CppUse)
	if p.Load == nil {
		return
	}
	for _, f := range p.Load.Files {
		if f.Rewrite == nil {
			continue
		}
		for _, u := range f.Rewrite.Uses {
			if (u.Kind == load.HeaderUse || u.Kind == load.NamespaceUse) && !u.Bare {
				p.sourceUses[u.Sel] = u
			}
		}
	}
}

// restoreNames returns e, or a copy of it where the rewritten names have their user spelling.
// Only the nodes on the way to a rewritten name are copied.
func (p *Package) restoreNames(e ast.Expr) ast.Expr {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		if u := p.sourceUses[x]; u != nil {
			return &ast.SelectorExpr{X: u.Ident, Sel: x.Sel}
		}
		if inner := p.restoreNames(x.X); inner != x.X {
			return &ast.SelectorExpr{X: inner, Sel: x.Sel}
		}
	case *ast.CallExpr:
		return p.restoreCall(x)
	case *ast.ParenExpr:
		if inner := p.restoreNames(x.X); inner != x.X {
			return &ast.ParenExpr{Lparen: x.Lparen, X: inner, Rparen: x.Rparen}
		}
	case *ast.StarExpr:
		if inner := p.restoreNames(x.X); inner != x.X {
			return &ast.StarExpr{Star: x.Star, X: inner}
		}
	case *ast.UnaryExpr:
		if inner := p.restoreNames(x.X); inner != x.X {
			return &ast.UnaryExpr{OpPos: x.OpPos, Op: x.Op, X: inner}
		}
	case *ast.BinaryExpr:
		left, right := p.restoreNames(x.X), p.restoreNames(x.Y)
		if left != x.X || right != x.Y {
			return &ast.BinaryExpr{X: left, OpPos: x.OpPos, Op: x.Op, Y: right}
		}
	case *ast.IndexExpr:
		base, index := p.restoreNames(x.X), p.restoreNames(x.Index)
		if base != x.X || index != x.Index {
			return &ast.IndexExpr{X: base, Lbrack: x.Lbrack, Index: index, Rbrack: x.Rbrack}
		}
	case *ast.IndexListExpr:
		base := p.restoreNames(x.X)
		indices, changed := p.restoreList(x.Indices)
		if base != x.X || changed {
			return &ast.IndexListExpr{X: base, Lbrack: x.Lbrack, Indices: indices, Rbrack: x.Rbrack}
		}
	case *ast.SliceExpr:
		copied := *x
		copied.X = p.restoreNames(x.X)
		changed := copied.X != x.X
		for _, part := range []struct{ from, to *ast.Expr }{
			{&x.Low, &copied.Low}, {&x.High, &copied.High}, {&x.Max, &copied.Max},
		} {
			if *part.from != nil {
				*part.to = p.restoreNames(*part.from)
				changed = changed || *part.to != *part.from
			}
		}
		if changed {
			return &copied
		}
	case *ast.TypeAssertExpr:
		inner := p.restoreNames(x.X)
		typ := x.Type
		if typ != nil {
			typ = p.restoreNames(typ)
		}
		if inner != x.X || typ != x.Type {
			return &ast.TypeAssertExpr{X: inner, Lparen: x.Lparen, Type: typ, Rparen: x.Rparen}
		}
	case *ast.CompositeLit:
		if x.Type != nil {
			if typ := p.restoreNames(x.Type); typ != x.Type {
				return &ast.CompositeLit{Type: typ, Lbrace: x.Lbrace, Elts: x.Elts, Rbrace: x.Rbrace}
			}
		}
	case *ast.ArrayType:
		if x.Elt != nil {
			if elt := p.restoreNames(x.Elt); elt != x.Elt {
				return &ast.ArrayType{Lbrack: x.Lbrack, Len: x.Len, Elt: elt}
			}
		}
	case *ast.MapType:
		key, value := p.restoreNames(x.Key), p.restoreNames(x.Value)
		if key != x.Key || value != x.Value {
			return &ast.MapType{Map: x.Map, Key: key, Value: value}
		}
	}
	return e
}

// restoreCall restores the call c: a cgo helper call gets its C.name back.
func (p *Package) restoreCall(c *ast.CallExpr) ast.Expr {
	args, changed := p.restoreList(c.Args)
	fun := p.restoreNames(c.Fun)
	if name := p.CgoHelper(c); name != "" {
		sel := ast.Unparen(c.Fun).(*ast.SelectorExpr)
		fun = &ast.SelectorExpr{
			X:   &ast.Ident{NamePos: sel.X.Pos(), Name: "C"},
			Sel: &ast.Ident{NamePos: sel.Sel.Pos(), Name: name},
		}
	}
	if fun == c.Fun && !changed {
		return c
	}
	return &ast.CallExpr{Fun: fun, Lparen: c.Lparen, Args: args, Ellipsis: c.Ellipsis, Rparen: c.Rparen}
}

// restoreList restores each expression of list; changed says whether a copy was made.
func (p *Package) restoreList(list []ast.Expr) ([]ast.Expr, bool) {
	var out []ast.Expr
	for i, e := range list {
		restored := p.restoreNames(e)
		if restored != e && out == nil {
			out = append([]ast.Expr(nil), list...)
		}
		if out != nil {
			out[i] = restored
		}
	}
	if out == nil {
		return list, false
	}
	return out, true
}

// IsC reports whether the name x is the package C (C in C.int, and the C that the loader's rewrite
// put in C.printf).
func (p *Package) IsC(x ast.Expr) bool {
	id, ok := x.(*ast.Ident)
	if !ok || p.Info == nil {
		return false
	}
	pn, ok := p.Info.Uses[id].(*types.PkgName)
	return ok && pn.Imported().Path() == "C"
}

// IsCTypeExpr reports whether the type expression e names a C/C++ type: C.int, lib.Obj,
// std.vector[int], or a pointer to one.
func (p *Package) IsCTypeExpr(e ast.Expr) bool {
	p.sourceUsesOnce.Do(p.collectSourceUses)
	switch x := ast.Unparen(e).(type) {
	case *ast.StarExpr:
		return p.IsCTypeExpr(x.X)
	case *ast.SelectorExpr:
		if p.IsC(x.X) || p.sourceUses[x] != nil {
			return true
		}
		return p.IsCTypeExpr(x.X)
	case *ast.IndexExpr:
		return p.IsCTypeExpr(x.X)
	case *ast.IndexListExpr:
		return p.IsCTypeExpr(x.X)
	}
	return false
}

// DeclaredTypeExpr returns the type as the program wrote it for the variable or field e, when it
// is a variable or a field that is declared with a type (var end *C.wchar_t), else nil. A
// parameter, and a variable whose type comes from its value, have none here.
func (p *Package) DeclaredTypeExpr(e ast.Expr) ast.Expr {
	var v *types.Var
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		v, _ = p.Info.Uses[x].(*types.Var)
	case *ast.SelectorExpr:
		if s := p.Info.Selections[x]; s != nil && s.Kind() == types.FieldVal {
			v, _ = s.Obj().(*types.Var)
		}
	}
	if v == nil {
		return nil
	}
	return p.Opaque.TypeExpr(v.Origin())
}
