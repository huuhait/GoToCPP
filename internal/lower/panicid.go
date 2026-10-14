package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

// The identity of a panic value (runtime/include/gocpp/panic.hpp, panic_identity; step 3.13).
//
// Go prints two neighbouring panics of a chain once, as "[recovered, repanicked]", when their
// values have the same interface words, and gc gives a constant other data words than a value
// that is computed: so the emitter says which a panic's value is (gp::panic_constant or
// gp::panic). gc counts more as a constant than go/types does: its escape analysis
// (cmd/compile/internal/escape rewriteWithLiterals, with ir.ReassignOracle) replaces a value
// converted to an interface by its constant when the value is a local variable whose only
// definition gives it a constant, through other such variables and conversions that change
// no representation: in
//
//	n := -1
//	defer panic(n)
//	panic(n)
//
// both panics have the constant -1, and Go prints "panic: -1 [recovered, repanicked]". Such a
// variable is declared by := or var with a value for each name (not a, b := f()), is not a
// parameter or a result, its address is never taken (&x, a pointer method, x[:]), and nothing
// assigns it again (=, op=, ++, --, a := that redeclares it, a range clause with =), also in a
// function literal, and conversions that change no representation (convNop) are looked
// through. (gc follows inlined calls too, n := five() with a five that returns a
// constant; gocpp doesn't: such a chain prints both panics.)

// staticConst returns the constant that gc's escape analysis puts in place of e where e is
// converted to an interface (the argument of panic), or nil.
func (fg *funcGen) staticConst(e ast.Expr) constant.Value {
	for range 100 { // a chain of variables; a cycle is impossible (each is defined before use)
		e = ast.Unparen(e)
		if v := fg.constValue(e); v != nil {
			return v
		}
		switch x := e.(type) {
		case *ast.Ident:
			v, ok := fg.g.info.Uses[x].(*types.Var)
			if !ok {
				return nil
			}
			init := fg.singleDef(v)
			if init == nil {
				return nil
			}
			e = init
		case *ast.CallExpr:
			tv := fg.g.info.Types[x.Fun]
			if !tv.IsType() || len(x.Args) != 1 || !convNop(fg.g.info.TypeOf(x.Args[0]), tv.Type) {
				return nil
			}
			e = x.Args[0]
		default:
			return nil
		}
	}
	return nil
}

// singleDef returns the initializer of the local variable v if its declaration is its only
// definition (see the top of the file), or nil.
func (fg *funcGen) singleDef(v *types.Var) ast.Expr {
	if fg.fn == nil {
		return nil
	}
	if fg.singleDefs == nil {
		fg.singleDefs = singleDefs(fg.g.info, fg.fn.top, fg.boxed)
	}
	return fg.singleDefs[v]
}

// singleDefs finds the local variables in top (a function declaration, or a package variable's
// initializer, with the func literals in it) that have one definition, with a value, and no
// other assignment, and whose address is never taken (boxed: box.go's reasons),
// with their initializers.
func singleDefs(info *types.Info, top ast.Node, boxed map[*types.Var]boxReason) map[*types.Var]ast.Expr {
	defs := make(map[*types.Var]ast.Expr)
	changed := make(map[*types.Var]bool)
	assigned := func(e ast.Expr) {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok {
			if v, ok := info.Uses[id].(*types.Var); ok {
				changed[v] = true
			}
		}
	}
	define := func(lhs []ast.Expr, rhs []ast.Expr) {
		for i, e := range lhs {
			id, ok := e.(*ast.Ident)
			if !ok {
				continue
			}
			if v, ok := info.Defs[id].(*types.Var); ok {
				if len(rhs) == len(lhs) {
					defs[v] = rhs[i]
				}
			} else {
				assigned(e) // a := that assigns an existing variable
			}
		}
	}
	ast.Inspect(top, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				define(n.Lhs, n.Rhs)
			} else {
				for _, e := range n.Lhs {
					assigned(e)
				}
			}
		case *ast.ValueSpec:
			ids := make([]ast.Expr, len(n.Names))
			for i, id := range n.Names {
				ids[i] = id
			}
			define(ids, n.Values)
		case *ast.IncDecStmt:
			assigned(n.X)
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				assigned(n.Key)
				if n.Value != nil {
					assigned(n.Value)
				}
			} else if n.Tok == token.DEFINE {
				// A range variable is defined by the range clause, never by a value.
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if id, ok := e.(*ast.Ident); ok {
						if v, ok := info.Defs[id].(*types.Var); ok {
							changed[v] = true
						}
					}
				}
			}
		}
		return true
	})
	for v := range defs {
		if changed[v] || boxed[v] != 0 && boxed[v] != boxLarge {
			delete(defs, v)
		}
	}
	return defs
}

// convNop reports whether gc makes the conversion of a value of type from to type to an
// OCONVNOP, which ir.ReassignOracle.StaticValue looks through (typecheck's convertOp and
// tcConv): identical underlying types, except floats and complex numbers (gc keeps a float
// conversion, which rounds, as OCONV), and integer types of the same SimType: int and int64,
// uint, uint64 and uintptr (gocpp's targets are 64-bit).
func convNop(from, to types.Type) bool {
	if from == nil || to == nil {
		return false
	}
	fb, _ := from.Underlying().(*types.Basic)
	tb, _ := to.Underlying().(*types.Basic)
	if fb != nil && tb != nil && fb.Info()&types.IsInteger != 0 && tb.Info()&types.IsInteger != 0 {
		return simKind(fb.Kind()) == simKind(tb.Kind())
	}
	if !types.Identical(from.Underlying(), to.Underlying()) {
		return false
	}
	return fb == nil || fb.Info()&(types.IsFloat|types.IsComplex) == 0
}

// simKind is gc's SimType of an integer kind on a 64-bit target: int is int64, and uint and
// uintptr are uint64.
func simKind(k types.BasicKind) types.BasicKind {
	switch k {
	case types.Int:
		return types.Int64
	case types.Uint, types.Uintptr:
		return types.Uint64
	}
	return k
}
