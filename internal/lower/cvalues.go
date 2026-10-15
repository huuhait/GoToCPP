package lower

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// C/C++ values in gocpp code (Phase 04 README §2.5): the part of step 4.4 that the C type
// translator and the names need to be tested.
//
//	cstdio.printf("%d\n", 5)            ::printf("%d\n", 5);       (a call as a statement)
//	var n C.int = cstdio.puts("x")      int n = ::puts("x");       (an initializer of a C type)
//	var v std.vector[int]               ::std::vector<int64_t> v{}; (the zero value: value-initialized)
//	x := cmath.sqrt(2.0)                auto x = ::sqrt(2.0);
//	C.int(n), C.ulonglong(n)            gp::cgo::as<int>(n), gp::cgo::as<unsigned long long>(n)
//	std.string("hi")                    ::std::string("hi")        (a constructor)
//	int(cmath.floor(x))                 gp::cgo::as<int64_t>(::floor(x)) (a conversion to a gocpp
//	                                    type, gp_cgoh.As[int](...) after the check's rewrite)
//	return n, f(v) (C types)            return n; ::f(v)
//
// An untyped constant where a C/C++ value takes it (an argument of a C/C++ function, the value of
// a variable or a result of a C/C++ type, a template argument) is a plain C literal (cConstant):
// "x" is a string literal there, not a gp::string, and 5 an int.
//
// Every opaque expression goes through opaqueExpr: what it doesn't translate yet (operators,
// member access on C/C++ values with gp::deref, composite literals, range) is reported as "not
// supported yet", never written as broken C++.

// varType returns the C++ type of the variable v (also a parameter, a result or a struct field):
// its gocpp type, or, for a type with C/C++ parts, the C++ of the type expression it was declared
// with; auto for a local declared from a C/C++ value without a type (x := lib.f()).
func (g *gen) varType(v *types.Var, pos token.Pos) cppir.Type {
	if complete(v.Type()) {
		return g.cxxType(v.Type(), pos)
	}
	if e := g.pkg.Opaque.TypeExpr(v); e != nil {
		return g.typeExpr(e)
	}
	if e := g.sigTypes()[v]; e != nil {
		return g.typeExpr(e)
	}
	if v.IsField() || v.Parent() == nil || v.Parent() == g.pkg.Types.Scope() {
		g.unsupported(pos, "package variables of C/C++ values without a type ("+v.Name()+"): write its type, var "+v.Name()+" T = ...")
		return typeName("gp_unsupported")
	}
	return typeName("auto")
}

// sigTypes returns the type expressions of the parameters and results of the package's
// functions and func literals, also those without a name (which have no object of their own for
// Opaque.TypeExpr), for the ones whose types have C/C++ parts.
func (g *gen) sigTypes() map[*types.Var]ast.Expr {
	if g.sigExprs != nil {
		return g.sigExprs
	}
	g.sigExprs = make(map[*types.Var]ast.Expr)
	add := func(list *ast.FieldList, vars *types.Tuple) {
		if list == nil {
			return
		}
		i := 0
		for _, f := range list.List {
			n := max(len(f.Names), 1)
			for range n {
				if i < vars.Len() && !complete(vars.At(i).Type()) {
					g.sigExprs[vars.At(i)] = f.Type
				}
				i++
			}
		}
	}
	for _, f := range g.pkg.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			var ft *ast.FuncType
			var sig *types.Signature
			switch x := n.(type) {
			case *ast.FuncDecl:
				if fn, ok := g.info.Defs[x.Name].(*types.Func); ok {
					ft, sig = x.Type, fn.Signature()
				}
			case *ast.FuncLit:
				sig, _ = g.info.TypeOf(x).(*types.Signature)
				ft = x.Type
			}
			if sig != nil {
				add(ft.Params, sig.Params())
				add(ft.Results, sig.Results())
			}
			return true
		})
	}
	return g.sigExprs
}

// opaqueExpr returns the C++ of the opaque expression e (check.Opaque): a C/C++ value whose type
// go/types doesn't know.
func (fg *funcGen) opaqueExpr(e ast.Expr) cppir.Expr {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return fg.expr(x.X)
	case *ast.Ident:
		if v, ok := fg.g.info.Uses[x].(*types.Var); ok && fg.isBoxed(v) {
			fg.g.unsupported(x.Pos(), "C/C++ values in closures, defers and behind & (step 4.8)")
			return placeholder
		}
		return fg.ident(x)
	case *ast.CallExpr:
		return fg.call(x)
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		if r, ok := fg.g.cRefOf(x); ok {
			return fg.g.cValue(r, x)
		}
		if sel, ok := x.(*ast.SelectorExpr); ok && fg.g.info.Selections[sel] != nil {
			// A field of a C/C++ type of a gocpp struct, or a method value or expression of a
			// gocpp type with C/C++ parameters or results (structs.go, methods.go).
			return fg.selector(sel)
		}
	}
	fg.g.unsupported(e.Pos(), "this operation on C/C++ values (step 4.4)")
	return placeholder
}

// cValue returns the C++ of the C/C++ name r, written as e, used as a value.
func (g *gen) cValue(r cRef, e ast.Expr) cppir.Expr {
	switch r.kind {
	case interop.Namespace:
		g.userError(e.Pos(), "%s is a C++ namespace, not a value", r.src)
		return placeholder
	case interop.Type, interop.Template:
		g.userError(e.Pos(), "%s is a C/C++ type, not a value", r.src)
		return placeholder
	}
	return r.x
}

// cCall returns the C++ of the call c if it calls a C/C++ name (a function, a function
// template's instance, a macro, a constructor or a C type's conversion), and false if it
// doesn't.
func (fg *funcGen) cCall(c *ast.CallExpr) (cppir.Expr, bool) {
	r, ok := fg.g.cRefOf(c.Fun)
	if !ok {
		return nil, false
	}
	if r.x == placeholder {
		return placeholder, true // reported
	}
	args := fg.cArgs(c.Args)
	switch r.kind {
	case interop.Namespace:
		fg.g.userError(c.Fun.Pos(), "%s is a C++ namespace, not a function", r.src)
		return placeholder, true
	case interop.Type:
		if n, ok := r.x.(*cppir.Name); ok && n.Args == nil && isBuiltinC(n.Name) && len(args) == 1 {
			// C.int(n): a conversion, with Go's results where a C++ cast is undefined
			// behavior (a float out of range: gp::cgo::as, gocpp/interop.hpp).
			return templateCall("gp::cgo::as", []cppir.Node{typeName(n.Name)}, args[0]), true
		}
	}
	if c.Ellipsis.IsValid() {
		fg.g.unsupported(c.Ellipsis, "... in a call of a C/C++ function")
		return placeholder, true
	}
	return &cppir.Call{Fun: r.x, Args: args}, true
}

// isCCall reports whether e is a call of a C/C++ function (not of a gocpp function with C/C++
// results).
func (fg *funcGen) isCCall(e ast.Expr) bool {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || !fg.g.pkg.Opaque.IsOpaque(c) {
		return false
	}
	_, ok = fg.g.cRefOf(c.Fun)
	return ok
}

// isBuiltinC reports whether the C++ name s is a C type of cBuiltins.
func isBuiltinC(s string) bool {
	for _, b := range cBuiltins {
		if b == s {
			return true
		}
	}
	return false
}

// cArgs returns the C++ of the arguments of a call of a C/C++ function: an untyped constant is a
// plain C literal (cConstant), nil is nullptr, and a gocpp value has its gocpp type.
func (fg *funcGen) cArgs(list []ast.Expr) []cppir.Expr {
	xs := make([]cppir.Expr, len(list))
	for i, a := range list {
		xs[i] = fg.cValueOf(a)
	}
	return xs
}

// cValueOf returns the C++ of e where a C/C++ value takes it (cArgs).
func (fg *funcGen) cValueOf(e ast.Expr) cppir.Expr {
	tv := fg.g.info.Types[e]
	switch {
	case tv.Value != nil:
		return fg.g.cConstant(tv.Value, tv.Type, e.Pos())
	case tv.IsNil():
		return lit("nullptr")
	}
	return fg.exactExpr(e)
}

// asConversion returns the C++ of the conversion conv of a C/C++ value to a gocpp type
// (gp_cgoh.As[T](x) after the check's rewrite): gp::cgo::as<T>(x) for a number or a bool, which
// is Go's conversion also where a static_cast is undefined behavior (a float out of range).
func (fg *funcGen) asConversion(c *ast.CallExpr, conv *check.Conversion) cppir.Expr {
	if conv.Kind != check.AsCall || len(c.Args) != 1 {
		fg.g.unsupported(c.Pos(), "this conversion of a C/C++ value (step 4.4)")
		return placeholder
	}
	to := fg.g.info.TypeOf(c)
	b := basicOf(to)
	if b == nil || b.Info()&(types.IsNumeric|types.IsBoolean) == 0 || b.Info()&types.IsComplex != 0 || isNewtype(to) {
		fg.g.unsupported(c.Pos(), "conversions of C/C++ values to "+types.TypeString(to, types.RelativeTo(fg.g.pkg.Types))+" (steps 4.4 and 4.9)")
		return placeholder
	}
	return templateCall("gp::cgo::as", []cppir.Node{fg.g.cxxType(to, c.Pos())}, fg.expr(c.Args[0]))
}

// markAllCValues runs markCValues over the package's functions and package variables.
func (g *gen) markAllCValues() {
	for _, f := range g.pkg.Load.Files {
		for _, d := range f.AST.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if fn, ok := g.info.Defs[d.Name].(*types.Func); ok && d.Body != nil {
					g.markCValues(d.Body, fn.Signature())
				}
			case *ast.GenDecl:
				g.markCValues(d, nil)
			}
		}
	}
}

// markCValues finds the untyped constants in n that are the values of C/C++ variables and
// results (var n C.int = 5, n = 5, return 5 from a func with a C/C++ result), which are C literals
// there (cConstant): go/types doesn't convert them, as it has no type for the variable. (nil is
// nullptr there anyway.) sig is
// the signature of the function that n is the body of (nil for a package variable's value).
func (g *gen) markCValues(n ast.Node, sig *types.Signature) {
	mark := func(e ast.Expr, cTarget bool) {
		if tv, ok := g.info.Types[e]; ok && cTarget && tv.Value != nil {
			if g.cConsts == nil {
				g.cConsts = make(map[ast.Expr]bool)
			}
			g.cConsts[ast.Unparen(e)] = true
		}
	}
	isC := func(e ast.Expr) bool {
		if g.pkg.Opaque.IsOpaque(e) {
			return true
		}
		if id, ok := e.(*ast.Ident); ok {
			if v, ok := g.info.ObjectOf(id).(*types.Var); ok {
				return !complete(v.Type())
			}
		}
		return false
	}
	ast.Inspect(n, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) == len(s.Rhs) {
				for i, v := range s.Rhs {
					mark(v, isC(s.Lhs[i]))
				}
			}
		case *ast.ValueSpec:
			for i, v := range s.Values {
				if i < len(s.Names) && len(s.Names) == len(s.Values) {
					mark(v, isC(s.Names[i]))
				}
			}
		case *ast.ReturnStmt:
			if sig != nil && sig.Results().Len() == len(s.Results) {
				for i, v := range s.Results {
					mark(v, !complete(sig.Results().At(i).Type()))
				}
			}
		case *ast.FuncLit:
			lit, _ := g.info.TypeOf(s).(*types.Signature)
			if lit != nil {
				g.markCValues(s.Body, lit)
			}
			return false
		}
		return true
	})
}
