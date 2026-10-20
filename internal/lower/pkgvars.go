package lower

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Package variables (step 3.16): which ones C++ initializes at compile time (constLit), and
// the ones larger than 128 KiB that gp_var_init builds in place (largeInit). decl.go has the
// rest: packageVars declares them, initFuncs writes gp_var_init and gp_init_funcs.
//
// Every package variable is constinit (packageVars), so C++ runs no dynamic initializer of its
// own, and no static initializer can read another one before gp_var_init has run (the static
// initialization order fiasco can't happen, also when Phase 06 writes one C++ file per .gp file).

// constLit returns the C++ initializer of a package variable whose initializer (init) is a
// composite literal that C++ can evaluate at compile time: an array or a struct literal whose
// values are number and bool constants or nested literals of the same kind, positional or
// keyed, and whose type is plain (plainType: numbers, also complex ones, bools, and arrays and
// structs of them). The variable is then constinit with it (packageVars), and gp_var_init
// doesn't set it. The outer literal is a braced list: garr{1, 2, 3}, pt{1, 2}, pt{.Y = 2},
// and an empty one is the zero value, pt{}. Others (a string, a pointer, a slice or a map anywhere in the type, any
// value that is not a constant) return nil: gp::string and gp::ptr are not literal types in
// C++ (their destructors are not constexpr), so gp::array_keyed can't build an array of them
// at compile time. So does a variable larger than 128 KiB, which largeInit builds in place:
// clang++ evaluates gp::array_keyed at compile time only for arrays of up to 1 << 20 elements
// ("evaluated array bound 2097152 exceeds the limit"), and a large literal of constants would
// put the whole value into the program file.
func (g *gen) constLit(init *types.Initializer) cppir.Expr {
	if init == nil || len(init.Lhs) != 1 || init.Lhs[0].Name() == "_" || largeVar(init.Lhs[0].Type()) {
		return nil
	}
	cl, ok := ast.Unparen(init.Rhs).(*ast.CompositeLit)
	if !ok {
		return nil
	}
	from, to := g.info.TypeOf(cl), init.Lhs[0].Type()
	if !plainType(from) || isStruct(from) && needsConversion(from, to) || isInterface(to) {
		return nil // (gp::convert of a struct is not constexpr, and neither is gp::to_iface of an array)
	}
	x := g.constLitOf(cl)
	if x == nil {
		return nil
	}
	// A literal of an unnamed array type for a variable of a named one (var g Grid = [3]int{...})
	// is converted to it (step 3.9).
	x = g.convertTo(from, to, x, cl.Pos())
	if l, ok := x.(*cppir.InitList); ok {
		l.Type = nil // the variable's braced initializer
	}
	return x
}

// constLitOf returns the C++ of the array or struct literal e if C++ can evaluate it at compile
// time (constLit), or nil.
func (g *gen) constLitOf(e *ast.CompositeLit) cppir.Expr {
	t := g.info.TypeOf(e)
	switch {
	case isArray(t):
		if at := under(t).(*types.Array); hasKeys(e) && at.Len() > 1<<20 {
			return nil // (clang++'s limit: constLit; an array of empty structs isn't large)
		}
		slot := under(t).(*types.Array).Elem()
		return g.arrayLitValue(e, func(x ast.Expr) cppir.Expr { return g.constElem(x, slot) })
	case isStruct(t):
		return g.constStruct(e, t)
	}
	return nil
}

// constStruct returns the C++ of the struct literal e of type t for constLit, written as
// structLit writes it: T{a, b}, or T{.X = a} for a keyed literal (or a large struct), in the
// fields' order. It returns nil if a value is not a constant or a literal of the same kind.
func (g *gen) constStruct(e *ast.CompositeLit, t types.Type) cppir.Expr {
	st := under(t).(*types.Struct)
	typ := g.cxxType(t, e.Lbrace)
	if !hasKeys(e) && !largeVar(t) {
		elems := make([]cppir.Expr, len(e.Elts))
		for i, elt := range e.Elts {
			if elems[i] = g.constElem(elt, st.Field(i).Type()); elems[i] == nil {
				return nil
			}
		}
		return &cppir.InitList{Type: typ, Elems: elems}
	}
	values := make([]ast.Expr, st.NumFields())
	for i, elt := range e.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			values[fieldIndex(st, kv.Key.(*ast.Ident).Name)] = kv.Value
		} else {
			values[i] = elt
		}
	}
	var elems []cppir.Expr
	for i, v := range values {
		if v == nil {
			continue
		}
		x := g.constElem(v, st.Field(i).Type())
		if x == nil {
			return nil
		}
		elems = append(elems, &cppir.Designated{Name: g.fieldName(t, i, e.Lbrace), X: x})
	}
	return &cppir.InitList{Type: typ, Elems: elems}
}

// constElem returns the C++ of x, an element of an array literal or a field's value in a struct
// literal, whose slot has the type slot, for constLit: a number (also a complex one) or bool
// constant (Celsius(1.5) is constexpr too), or a nested literal of the slot's own type.
// Anything else is nil, also a literal that Go converts implicitly to the slot's type
// ([2]int{...} for a named Pair slot).
func (g *gen) constElem(x ast.Expr, slot types.Type) cppir.Expr {
	if l, ok := ast.Unparen(x).(*ast.CompositeLit); ok {
		if !types.Identical(g.info.TypeOf(l), slot) {
			return nil
		}
		return g.constLitOf(l)
	}
	tv := g.info.Types[x]
	b := underBasic(tv.Type)
	if tv.Value == nil || b == nil || b.Info()&(types.IsNumeric|types.IsBoolean) == 0 {
		return nil
	}
	return g.constant(tv.Value, tv.Type, x.Pos(), false)
}

// plainType reports whether t is made of numbers (also complex ones: gp::complex is a literal
// type) and bools only, through arrays and structs: a literal type in C++ (constLit).
func plainType(t types.Type) bool {
	switch u := under(t).(type) {
	case *types.Basic:
		return u.Info()&(types.IsNumeric|types.IsBoolean) != 0
	case *types.Array:
		return plainType(u.Elem())
	case *types.Struct:
		for i := range u.NumFields() {
			if !plainType(u.Field(i).Type()) {
				return false
			}
		}
		return true
	}
	return false
}

// largeInit lowers the initializer init in gp_var_init if it gives one package variable larger
// than 128 KiB a composite literal, also inside conversions (block([1 << 21]int{5: f()})): the
// literal is built in the variable itself, element by
// element and field by field (storeLit, structs.go), as a large local variable is built in its
// box (largeBox, arrays.go), since `v = Big{...};` would first build the value in a temporary
// on the C++ stack, which could overflow it (D24). That is safe because the variable is still
// zero when its initializer runs, and nothing that the initializer evaluates can read it: Go
// reports such a reference, also through functions, as an initialization cycle. It reports
// whether it lowered init.
func (fg *funcGen) largeInit(init *types.Initializer) bool {
	if len(init.Lhs) != 1 || init.Lhs[0].Name() == "_" || !largeVar(init.Lhs[0].Type()) {
		return false
	}
	g := fg.g
	if !g.convertedLit(init.Rhs) {
		return false // a call's result, a copy of another variable: D24
	}
	p := g.pos(init.Rhs.Pos())
	lines := fg.order(false, init.Rhs)
	temps := len(lines) > 0
	x := fg.expr(init.Rhs)
	if l := unconvert(x); l != nil {
		x = l
	}
	storeLit(&lines, cxxName(g.names[init.Lhs[0]]), x)
	switch {
	case len(lines) == 0: // an empty literal: the zero value, which the variable has
	case temps:
		fg.add(&cppir.Block{Pos: p, Stmts: lines, Inline: true})
	default:
		fg.add(&cppir.Line{Pos: p, Stmts: lines})
	}
	return true
}

// convertedLit reports whether e is a composite literal, maybe inside conversions between types
// with the same underlying type (block([1 << 21]int{...}), rec2(rec{...})), and parentheses.
func (g *gen) convertedLit(e ast.Expr) bool {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.CompositeLit:
			return true
		case *ast.CallExpr:
			if !g.info.Types[x.Fun].IsType() || len(x.Args) != 1 ||
				!types.Identical(under(g.info.TypeOf(x)), under(g.info.TypeOf(x.Args[0]))) {
				return false
			}
			e = x.Args[0]
		default:
			return false
		}
	}
}

// unconvert returns the C++ of the literal inside the conversions x of a literal (largeInit):
// x itself if it is a literal, the array or struct literal inside T(lit) (a named type: a
// gp::array is the base class of a named array type), T(lit).gp_value() (to an unnamed type)
// and gp::convert<T>(lit) (between struct types, whose fields have the same names and types), or
// nil if x is something else. largeInit stores the literal's parts into the variable, whose
// type has the same underlying type.
func unconvert(x cppir.Expr) cppir.Expr {
	for {
		switch c := x.(type) {
		case *cppir.InitList:
			if c.Type == nil {
				return nil
			}
			return c // gp::array<T, N>{...}, T{...}
		case *cppir.Call:
			if n, ok := c.Fun.(*cppir.Name); ok && n.Name == "gp::array_keyed" {
				return c
			}
			switch {
			case len(c.Args) == 1: // T(lit), gp::convert<T>(lit)
				x = c.Args[0]
			case len(c.Args) == 0: // T(lit).gp_value()
				m, ok := c.Fun.(*cppir.Member)
				if !ok || m.Arrow || m.Name != "gp_value" {
					return nil
				}
				x = m.X
			default:
				return nil
			}
		default:
			return nil
		}
	}
}

// lazyVar adds the package variable v whose type holds a C or C++ class by value (cmpHasC):
// std.vector[int], std.mutex, a C struct, a struct of ours with such a field. C++ may not
// evaluate the constructor of such a class at compile time (std::ofstream, a class that counts
// its objects), so the variable cannot be constinit, and a variable with a dynamic initializer
// can be wiped by it after gp_var_init has stored its value (a library whose host calls an
// exported function from a static initializer of its own). It is gp::lazy storage (ADR-093,
// gocpp/lazy.hpp): constant-initialized with no object, the variable's own name is a constinit
// reference to the object, which gp_var_init builds first (T{}, the zero value) before it sets
// any variable; a dynamic initializer builds it when gp_var_init did not (a cpp-lib consumer
// that does not call init()). The object is never destroyed.
//
//	gp::lazy<std::vector<int64_t>> gp_lazy_global;
//	constinit std::vector<int64_t>& global = gp_lazy_global.gp_ref();
//	static const gp::lazy_boot gp_boot_global{gp_lazy_global};
func (g *gen) lazyVar(v *types.Var) {
	name := g.names[v]
	store := "gp_lazy_" + name
	typ := g.varType(v, v.Pos())
	pos := g.pos(v.Pos())
	g.out.vars = append(g.out.vars,
		varDecl(pos, &cppir.TypeName{Name: "gp::lazy", Args: []cppir.Node{typ}}, store, nil),
		&cppir.VarDecl{Pos: pos, Specs: []string{"constinit"}, Type: &cppir.Ref{Elem: typ}, Vars: []cppir.Declarator{{
			Name: name, Init: &cppir.Call{Fun: &cppir.Member{X: cxxName(store), Name: "gp_ref"}},
		}}},
		&cppir.VarDecl{Specs: []string{"static", "const"}, Type: typeName("gp::lazy_boot"), Vars: []cppir.Declarator{{
			Name: "gp_boot_" + name, Init: &cppir.InitList{Elems: []cppir.Expr{cxxName(store)}},
		}}})
	g.out.varExt = append(g.out.varExt, &cppir.VarDecl{Specs: []string{"extern"}, Type: &cppir.Ref{Elem: typ}, Vars: []cppir.Declarator{{Name: name}}})
	g.lazyVars = append(g.lazyVars, store)
	g.varInits = true
}
