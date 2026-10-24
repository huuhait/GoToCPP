package lower

import (
	"go/ast"
	"go/token"
	"go/types"
)

// The results of generic calls whose type arguments C++ deduces (ADR-141, dev.genericstd).
//
// go/types gives no type to a call of a generic function that it could not instantiate because an
// argument is a C/C++ value (slices.Insert(c, 1, std.string("q")) with c a []std.string), so the
// result, and a variable declared from it (r := slices.Insert(...)), is a C/C++ value to the
// lowering: len(r), r[1:] and append(r, x) were refused, although the result is a gocpp slice (or
// map) whose C++ type only the C++ compiler knows the element of. typeDeducedResults gives such a
// call, and the uses of a variable declared from it with :=, the gocpp type that the instance's
// result has with every type argument that C++ deduces replaced by the invalid type, the type that
// go/types gives a []std.string itself: []invalid. Then the operations of a gocpp container of
// C/C++ values apply, as for c. A result that is not a container (a T, a func, a struct) keeps no
// type: it stays a C/C++ value, which it is.

// typeDeducedResults records the types described above in the package's maps, before the checks of
// the C/C++ values (checkCUses). Also in generic functions, for calls that do not depend on their type
// parameters: the snapshot of the generic function (generics.go) keeps them for its instances.
func (g *gen) typeDeducedResults() {
	fg := &funcGen{g: g}
	count := g.cxxCount
	defer func() { g.cxxCount = count }()
	typed := make(map[*types.Var]types.Type)
	var uses []*ast.Ident
	// define notes the variables declared from such calls (r := slices.Insert(...)).
	define := func(lhs, rhs []ast.Expr) {
		for i, l := range lhs {
			id, ok := l.(*ast.Ident)
			if !ok {
				continue
			}
			v, ok := g.info.Defs[id].(*types.Var)
			if !ok || complete(v.Type()) {
				continue
			}
			if call, ok := ast.Unparen(rhs[i]).(*ast.CallExpr); ok {
				if t := g.deducedResult(fg, call); t != nil {
					typed[v] = t
				}
			}
		}
	}
	for _, f := range g.pkg.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				uses = append(uses, n)
			case *ast.CallExpr:
				t := g.deducedResult(fg, n)
				if t == nil {
					return true
				}
				tv, ok := g.info.Types[n]
				if !ok {
					tv = types.TypeAndValue{Type: t}
					g.setValueMode(&tv)
				}
				tv.Type = t
				g.setTypeTemp(n, tv)
				g.noteDeduced(n)
			case *ast.AssignStmt:
				if n.Tok == token.DEFINE && len(n.Lhs) == len(n.Rhs) {
					define(n.Lhs, n.Rhs)
				}
			case *ast.ValueSpec:
				// var r = slices.Insert(...) in a function (a package variable has the instance's
				// result type, or check's error: cvalues.go, pkgcvars.go)
				if n.Type == nil && len(n.Names) == len(n.Values) && len(n.Names) > 0 && !isPkgLevel(g.info.Defs[n.Names[0]]) {
					lhs := make([]ast.Expr, len(n.Names))
					for i, id := range n.Names {
						lhs[i] = id
					}
					define(lhs, n.Values)
				}
			}
			return true
		})
	}
	for _, id := range uses {
		v, ok := g.info.Uses[id].(*types.Var)
		if !ok {
			continue
		}
		if t := typed[v]; t != nil {
			tv := types.TypeAndValue{Type: t}
			g.setValueMode(&tv)
			if old, ok := g.info.Types[id]; ok {
				tv = old
				tv.Type = t
			}
			g.setTypeTemp(id, tv)
			g.noteDeduced(id)
		}
	}
	if len(g.deduced) > 0 {
		g.typeDeducedOperands()
	}
}

// typeDeducedOperands types the expressions that go/types left untyped because an operand was such a
// result, and that are Go values now: len(r) and cap(r) are ints, and so is len(r) + cap(r[:1]), a
// comparison len(r) > 0 is a bool, (x) is x, r[1:] is r's slice type.
func (g *gen) typeDeducedOperands() {
	intType := types.Typ[types.Int]
	set := func(e ast.Expr, t types.Type) {
		tv := types.TypeAndValue{Type: t}
		g.setValueMode(&tv)
		g.setTypeTemp(e, tv)
		g.noteDeduced(e)
		if !gocppContainer(t) {
			// A Go number or bool: no C/C++ value for the checks and the lowering (restoreTypes
			// puts the mark back).
			was := g.pkg.Opaque.SetOpaque(e, false)
			g.opaqueUndo = append(g.opaqueUndo, opaqueUndo{e, was})
		}
	}
	known := func(e ast.Expr) types.Type {
		tv, ok := g.info.Types[e]
		if !ok || tv.Type == nil || !complete(tv.Type) {
			return nil
		}
		return tv.Type
	}
	// ast.Inspect is pre-order: walk again until nothing changes (such expressions are shallow).
	for changed := true; changed; {
		changed = false
		for _, f := range g.pkg.Load.Files {
			ast.Inspect(f.AST, func(n ast.Node) bool {
				e, ok := n.(ast.Expr)
				if !ok || known(e) != nil {
					return true
				}
				switch x := e.(type) {
				case *ast.ParenExpr:
					if t := known(x.X); t != nil && g.deduced[ast.Unparen(x.X)] {
						set(x, t)
						changed = true
					}
				case *ast.CallExpr:
					id, ok := ast.Unparen(x.Fun).(*ast.Ident)
					if !ok || len(x.Args) != 1 {
						return true
					}
					if b, ok := g.info.Uses[id].(*types.Builtin); ok && (b.Name() == "len" || b.Name() == "cap") && g.deduced[ast.Unparen(x.Args[0])] {
						set(x, intType)
						changed = true
					}
				case *ast.SliceExpr:
					if t := known(x.X); t != nil && g.deduced[ast.Unparen(x.X)] && isSlice(t) {
						set(x, t) // r[1:] of a deduced slice
						changed = true
					}
				case *ast.BinaryExpr:
					if !g.deduced[ast.Unparen(x.X)] && !g.deduced[ast.Unparen(x.Y)] {
						return true
					}
					xt, yt := known(x.X), known(x.Y)
					if xt == nil || yt == nil {
						return true
					}
					if isUntypedType(xt) {
						xt = yt
					}
					if isUntypedType(yt) {
						yt = xt
					}
					if !types.Identical(xt, yt) || underBasic(xt) == nil {
						return true
					}
					switch x.Op {
					case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
						set(x, types.Typ[types.Bool])
					default:
						set(x, xt)
					}
					changed = true
				}
				return true
			})
		}
	}
}

// isUntypedType reports whether t is an untyped basic type (the 1 of len(r) + 1).
func isUntypedType(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Info()&types.IsUntyped != 0
}

// A typeUndo is what setTypeTemp replaced in a map of type information.
type typeUndo struct {
	m   map[ast.Expr]types.TypeAndValue
	tv  types.TypeAndValue
	had bool
}

// setTypeTemp records tv as the type information of e for the lowering of the package, in the
// package's map itself, and remembers what was there for restoreTypes, which Program.Lower (and file,
// for one translation unit) calls at the end, after the shared instances too.
// It does not make the maps private (privateInfoOnce): the typed interop of a package with private
// maps is off (typed.go: inShared0), and so the checks of its C/C++ values would change.
func (g *gen) setTypeTemp(e ast.Expr, tv types.TypeAndValue) {
	if g.typesUndo == nil {
		g.typesUndo = make(map[ast.Expr]typeUndo)
	}
	if _, done := g.typesUndo[e]; !done {
		old, had := g.info.Types[e]
		g.typesUndo[e] = typeUndo{m: g.info.Types, tv: old, had: had}
	}
	g.info.Types[e] = tv
}

// An opaqueUndo is an opaque mark that typeDeducedOperands took away.
type opaqueUndo struct {
	e   ast.Expr
	was bool
}

// restoreTypes puts back what setTypeTemp replaced, and the opaque marks: the maps belong to the
// checker's result.
func (g *gen) restoreTypes() {
	for _, u := range g.opaqueUndo {
		g.pkg.Opaque.SetOpaque(u.e, u.was)
	}
	g.opaqueUndo = nil
	for e, u := range g.typesUndo {
		if u.had {
			u.m[e] = u.tv
		} else {
			delete(u.m, e)
		}
	}
	g.typesUndo = nil
}

func (g *gen) noteDeduced(e ast.Expr) {
	if g.deduced == nil {
		g.deduced = make(map[ast.Expr]bool)
	}
	g.deduced[e] = true
}

// setValueMode makes tv the TypeAndValue of a value (go/types' mode "value": IsValue, not
// addressable). The mode is not exported: it is taken from an expression of the package that is one.
func (g *gen) setValueMode(tv *types.TypeAndValue) {
	if !g.valueModeDone {
		g.valueModeDone = true
		for _, x := range g.info.Types {
			if x.IsValue() && !x.Addressable() && !x.HasOk() && !x.IsNil() && !x.IsBuiltin() && x.Value == nil && !x.IsVoid() {
				g.valueMode = x
				break
			}
		}
	}
	t := tv.Type
	*tv = g.valueMode
	tv.Type, tv.Value = t, nil
}

// deducedResult returns the gocpp container type of the result of the call c of a generic function
// whose type arguments C++ deduces (see typeDeducedResults), or nil.
func (g *gen) deducedResult(fg *funcGen, c *ast.CallExpr) types.Type {
	if tv, ok := g.info.Types[c]; ok && tv.Type != nil && complete(tv.Type) {
		return nil
	}
	if !g.genericRef(genericFuncOf(c.Fun)) {
		return nil
	}
	r := fg.instRef(c.Fun)
	if r == nil || r.gf == nil || r.sig == nil || r.sig.Results().Len() == 0 || len(r.open) == 0 {
		return nil // (a call with written C++ type arguments only stays as it is: its C++ is exact)
	}
	for _, a := range r.targs {
		if hasTypeParam(a) {
			return nil // in a generic function, a call whose type arguments are its type parameters: each instance types it
		}
	}
	tparams := r.gf.fn.Signature().TypeParams()
	replace := func(n *types.Named) (types.Type, bool) {
		info, ok := g.cxxNamed[n]
		if !ok {
			return nil, false
		}
		if !info.open {
			return nil, true // a written C++ type argument (F[std.string]): not retyped
		}
		for i, a := range r.targs {
			if a != n || i >= tparams.Len() {
				continue
			}
			if info.core != nil {
				if at, ok := g.argTypeOf(r.gf, c, tparams.At(i)).(*types.Slice); ok && !complete(at) {
					return at, true // S of slices.Insert(c, ...): c's type, []invalid
				}
				return types.NewSlice(types.Typ[types.Invalid]), true
			}
		}
		return types.Typ[types.Invalid], true
	}
	var walk func(t types.Type) types.Type
	walk = func(t types.Type) types.Type {
		switch t := types.Unalias(t).(type) {
		case *types.Named:
			if r, ok := replace(t); ok {
				return r
			}
			if g.hasCxxNamed(t) {
				return nil
			}
			return t
		case *types.Slice:
			if e := walk(t.Elem()); e != nil {
				return types.NewSlice(e)
			}
		case *types.Map:
			k, v := walk(t.Key()), walk(t.Elem())
			if k != nil && v != nil && !cmpHasC(k) {
				return types.NewMap(k, v)
			}
		case *types.Basic:
			return t
		default:
			if !g.hasCxxNamed(t) {
				return t
			}
		}
		return nil
	}
	if res := r.sig.Results(); res.Len() > 1 {
		// v, ok = Get(m, k): a tuple whose Go parts are typed (the bool), the others C/C++ values.
		vars := make([]*types.Var, res.Len())
		known := false
		for i := range res.Len() {
			t := walk(res.At(i).Type())
			if t == nil {
				return nil
			}
			known = known || complete(t)
			vars[i] = types.NewVar(token.NoPos, nil, "", t)
		}
		if !known {
			return nil
		}
		return types.NewTuple(vars...)
	}
	res := walk(r.sig.Results().At(0).Type())
	if res == nil || complete(res) || !gocppContainer(res) {
		return nil
	}
	if _, isMap := res.Underlying().(*types.Map); !isMap {
		if _, isSlice := res.Underlying().(*types.Slice); !isSlice {
			return nil
		}
	}
	return res
}

// genericFuncOf returns the part of the function expression of a call that names the function: F
// of F[int].
func genericFuncOf(fun ast.Expr) ast.Expr {
	for {
		switch x := ast.Unparen(fun).(type) {
		case *ast.IndexExpr:
			fun = x.X
		case *ast.IndexListExpr:
			fun = x.X
		default:
			return x
		}
	}
}

// deducedContainer reports whether e is a C/C++ value to check that typeDeducedResults gave a gocpp
// container type: the result of a generic call whose type arguments C++ deduces, or a variable
// declared from one.
func (g *gen) deducedContainer(e ast.Expr) bool {
	return g.deduced[ast.Unparen(e)]
}

// deducedKind returns "slice" or "map", what such an expression is.
func (g *gen) deducedKind(e ast.Expr) string {
	if isMap(g.info.TypeOf(e)) {
		return "map"
	}
	return "slice"
}

// deducedTupleOK checks v, ok = Get(m, k), the results of a generic call whose type arguments C++
// deduces (a tuple that typeDeducedResults typed) assigned to existing variables: a result of a Go
// type goes to a variable of a type it is assignable to, a C/C++ result to a C/C++ variable. done is
// false for any other e.
func (g *gen) deducedTupleOK(e ast.Expr, parent ast.Node) (ok, done bool) {
	tup, isTuple := g.info.TypeOf(e).(*types.Tuple)
	if !isTuple || !g.deducedContainer(e) {
		return false, false
	}
	var lhs []ast.Expr
	switch p := parent.(type) {
	case *ast.AssignStmt:
		lhs = p.Lhs
	case *ast.ValueSpec:
		for _, n := range p.Names {
			lhs = append(lhs, n)
		}
	default:
		return true, true // f(Get(m, k)): the parameters say (calledSig)
	}
	if len(lhs) != tup.Len() {
		return false, false
	}
	for i, l := range lhs {
		if id, isID := l.(*ast.Ident); isID && id.Name == "_" {
			continue
		}
		lt := g.info.TypeOf(l)
		if lt == nil {
			if id, isID := l.(*ast.Ident); isID {
				if v, isVar := g.info.ObjectOf(id).(*types.Var); isVar {
					lt = v.Type()
				}
			}
		}
		rt := tup.At(i).Type()
		switch {
		case lt == nil || !complete(lt):
			// a C/C++ variable takes what C++ converts to it
		case complete(rt) && types.AssignableTo(rt, lt):
		default:
			ts := types.TypeString(lt, types.RelativeTo(g.pkg.Types))
			g.userError(l.Pos(), "cannot use result %d of %s (C/C++ value) as %s value in assignment: declare new variables for its values (a, b := x), and convert those", i+1, g.srcText(e), ts)
			return false, true
		}
	}
	return true, true
}

// fromGenericCall reports whether e is a call of a generic function of the program, or a variable
// declared from one.
func (g *gen) fromGenericCall(e ast.Expr) bool {
	e = ast.Unparen(e)
	if id, ok := e.(*ast.Ident); ok {
		if v, ok := g.info.Uses[id].(*types.Var); ok {
			e = ast.Unparen(g.initialValue(v))
		}
	}
	c, ok := e.(*ast.CallExpr)
	return ok && g.genericRef(genericFuncOf(c.Fun))
}
