package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

// A state says why an expression has no type, for the analysis of the
// arguments of conversions. The order matters: an expression with several
// sources has the largest state of them (see analysis.state), and only an
// argument that is opaque is rewritten.
type state int

const (
	// typed: the expression has a type (it is no source of invalid).
	typed state = iota

	// waiting: invalid only because of conversions that this round
	// rewrites. The conversion waits for the next round, where its
	// argument may be typed.
	waiting

	// opaque: invalid because it comes from C/C++, whatever the other
	// conversions become.
	opaque

	// unknown: invalid for a reason that the analysis doesn't follow,
	// which can be an error. It wins over opaque (C.f() + v, where v is
	// invalid because of an error, gets no rewrite), and it is never
	// rewritten.
	unknown

	// errored: an error is the reason, or may be. Never rewritten.
	errored
)

// A declNode is what gives a declared name its type: the value of
// x := v, or the type of a parameter p T.
type declNode struct {
	node   ast.Node
	isType bool // node is a type expression, else a value
}

// declNodes maps the place of each name that files declare (the place of
// its types.Object) to the expressions that give it its type: the value
// or the type of a var or const (also the values that a const without
// values repeats), the value of x := v and of a range clause (x in range
// v), the guard and the case types of a type switch, the type of a
// parameter, result, receiver or struct field (also an embedded one), the
// signature of a
// function, and the type of a type declaration. For a value that is a
// function literal, it is the type of the literal: errors in its body
// don't change its type.
func declNodes(files []*ast.File) map[token.Pos][]declNode {
	m := make(map[token.Pos][]declNode)
	add := func(id *ast.Ident, nodes ...declNode) {
		if id != nil && id.Name != "_" {
			m[id.Pos()] = append(m[id.Pos()], nodes...)
		}
	}
	value := func(e ast.Expr) declNode {
		if lit, ok := e.(*ast.FuncLit); ok {
			return declNode{node: lit.Type, isType: true}
		}
		return declNode{node: e}
	}
	values := func(id *ast.Ident, i, n int, vals []ast.Expr) {
		if len(vals) == n {
			add(id, value(vals[i]))
			return
		}
		for _, v := range vals {
			add(id, value(v))
		}
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.GenDecl:
				if n.Tok != token.CONST && n.Tok != token.VAR {
					break
				}
				var last *ast.ValueSpec // of a const declaration: the last spec with values
				for _, s := range n.Specs {
					vs, ok := s.(*ast.ValueSpec)
					if !ok {
						continue
					}
					src := vs
					if n.Tok == token.CONST {
						if vs.Type == nil && len(vs.Values) == 0 && last != nil {
							src = last
						} else {
							last = vs
						}
					}
					for i, name := range vs.Names {
						if src.Type != nil {
							add(name, declNode{node: src.Type, isType: true})
						} else {
							values(name, i, len(vs.Names), src.Values)
						}
					}
				}
			case *ast.AssignStmt:
				if n.Tok != token.DEFINE {
					break
				}
				if len(n.Rhs) == 1 {
					if ta, ok := n.Rhs[0].(*ast.TypeAssertExpr); ok && ta.Type == nil {
						break // the guard of a type switch, below
					}
				}
				for i, lhs := range n.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						values(id, i, len(n.Lhs), n.Rhs)
					}
				}
			case *ast.RangeStmt:
				if n.Tok != token.DEFINE {
					break
				}
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if id, ok := e.(*ast.Ident); ok {
						add(id, declNode{node: n.X})
					}
				}
			case *ast.TypeSwitchStmt:
				as, ok := n.Assign.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
					break
				}
				id, ok1 := as.Lhs[0].(*ast.Ident)
				ta, ok2 := as.Rhs[0].(*ast.TypeAssertExpr)
				if !ok1 || !ok2 {
					break
				}
				add(id, declNode{node: ta.X})
				for _, s := range n.Body.List {
					if cc, ok := s.(*ast.CaseClause); ok {
						for _, t := range cc.List {
							add(id, declNode{node: t, isType: true})
						}
					}
				}
			case *ast.Field:
				for _, name := range n.Names {
					add(name, declNode{node: n.Type, isType: true})
				}
				if len(n.Names) == 0 {
					// An embedded field: go/types puts it at the name of
					// its type (FILE in *C.FILE), see embeddedIdent.
					if id := embeddedIdent(n.Type); id != nil {
						add(id, declNode{node: n.Type, isType: true})
					}
				}
			case *ast.FuncDecl:
				add(n.Name, declNode{node: n.Type, isType: true})
			case *ast.TypeSpec:
				add(n.Name, declNode{node: n.Type, isType: true})
			}
			return true
		})
	}
	return m
}

// An analysis finds out, in one round, which arguments of conversions are
// opaque (see converter).
type analysis struct {
	info *types.Info
	pkg  *types.Package

	// errs are the places of the errors of the check that can make a
	// value invalid (see collector.blocking), sorted.
	errs []token.Pos

	decls map[token.Pos][]declNode

	// cands holds the conversions found in this round, and cmps the
	// comparisons.
	cands map[*ast.CallExpr]bool
	cmps  map[*ast.BinaryExpr]bool

	// uns holds the calls of unsafe.Slice and unsafe.String found in this
	// round (unsafecalls.go): like a conversion, one gets its type from the
	// rewrite, so what depends on it waits for the next round.
	uns map[*ast.CallExpr]bool

	// sls holds the slice expressions of opaque values found in this round
	// (slicecalls.go), which the rewrite types too.
	sls map[*ast.SliceExpr]bool

	// memo holds the state of each object looked at (unknown while it is
	// being looked at: a cycle of values is an initialization cycle), and
	// types the state of each type name (typed while it is being looked
	// at: a type that refers to itself, type L struct{ next *L; v C.int },
	// gets its state from its other parts).
	memo  map[types.Object]state
	types map[*types.TypeName]state
}

// errorIn reports whether an error is inside the text of n.
func (a *analysis) errorIn(n ast.Node) bool {
	return a.errorBetween(n.Pos(), n.End())
}

// errorAt reports whether an error is at pos.
func (a *analysis) errorAt(pos token.Pos) bool {
	return a.errorBetween(pos, pos+1)
}

// errorAtName reports whether an error is at the name of obj, where its
// declaration is ("invalid recursive type T", "initialization cycle", and
// the errors of range variables are there).
func (a *analysis) errorAtName(obj types.Object) bool {
	return a.errorBetween(obj.Pos(), obj.Pos()+token.Pos(len(obj.Name())))
}

// errorBetween reports whether an error is at a place from pos to end
// (without end).
func (a *analysis) errorBetween(pos, end token.Pos) bool {
	i, _ := slices.BinarySearch(a.errs, pos)
	return i < len(a.errs) && a.errs[i] < end
}

// state returns why the expression e has no type: typed if it has one.
// The state of an invalid expression is the largest state of its sources:
// the operands whose invalid value makes e invalid (go/types makes the
// result of an operation on an invalid operand invalid, without an
// error). An invalid expression whose sources the analysis doesn't know is
// unknown.
func (a *analysis) state(e ast.Expr) state {
	if e == nil {
		return typed
	}
	if id, ok := e.(*ast.Ident); ok && a.genericFunc(id) {
		return typed // the name of a generic function is no source of invalid: its arguments are
	}
	tv, ok := a.info.Types[e]
	if ok && isValidType(tv.Type) && !hasInvalid(tv.Type) {
		return typed
	}
	if ok && tv.IsType() {
		return a.typeExprState(e) // []C.int in make([]C.int, 3)
	}
	if b, ok := e.(*ast.BinaryExpr); ok && a.cmps[b] {
		return waiting // a comparison that this round rewrites to a bool
	}
	if s, ok := e.(*ast.SliceExpr); ok && a.sls[s] {
		return waiting // a slice of a C array that this round rewrites
	}
	if call, ok := e.(*ast.CallExpr); ok {
		if a.cands[call] || a.uns[call] {
			return waiting
		}
		if _, typ := cgohCall(call, a.info); typ != nil {
			return a.typeExprState(typ) // As[[]C.int](x) of an earlier round
		}
	}
	if s := a.source(e); s != typed {
		return s
	}
	return unknown
}

// source returns the largest state of the sources of the invalid
// expression e, or typed if it finds none.
func (a *analysis) source(e ast.Expr) state {
	switch e := e.(type) {
	case *ast.Ident:
		return a.objState(a.info.Uses[e])
	case *ast.ParenExpr:
		return a.state(e.X)
	case *ast.SelectorExpr:
		if rootedInC(e, a.info) {
			return opaque
		}
		if id, ok := e.X.(*ast.Ident); ok {
			if _, ok := a.info.Uses[id].(*types.PkgName); ok {
				return a.objState(a.info.Uses[e.Sel]) // a qualified name
			}
		}
		if sel, ok := a.info.Selections[e]; ok {
			// A field or a method of a typed value: its own type decides,
			// not the rest of x's type (in struct{ a Undef; b C.int },
			// s.b is opaque).
			return a.objState(sel.Obj())
		}
		return max(a.state(e.X), a.objState(a.info.Uses[e.Sel]))
	case *ast.CallExpr:
		fun := a.info.Types[e.Fun]
		switch {
		case fun.IsType():
			// A conversion that is not rewritten (where Go needs a
			// constant): it is invalid if its argument is.
			// A type with a C/C++ part is a source too: (*[16]C.char)(p) is a pointer to
			// an array of C elements, which go/types gives that type without a rewrite.
			if len(e.Args) == 1 {
				return max(a.typeState(e.Fun), a.state(e.Args[0]))
			}
			return typed
		case fun.IsBuiltin():
			// make([]C.int, 3), len(v), append(xs, 1): the arguments (a
			// type argument too).
			s := typed
			for _, arg := range e.Args {
				s = max(s, a.state(arg))
			}
			return s
		}
		// A call of an invalid function is invalid whatever its
		// arguments are, and a call of a function with an invalid result
		// type is invalid because of that type.
		s := a.state(e.Fun)
		if a.uninstantiated(e.Fun) {
			// A generic function that go/types couldn't instantiate, id(cv): its result
			// has the type of an opaque argument.
			for _, arg := range e.Args {
				s = max(s, a.state(arg))
			}
			// And the type arguments written out, Mk[std.vector[int]]().
			for f := ast.Unparen(e.Fun); ; {
				switch x := f.(type) {
				case *ast.IndexExpr:
					s = max(s, a.state(x.Index))
					f = ast.Unparen(x.X)
					continue
				case *ast.IndexListExpr:
					for _, idx := range x.Indices {
						s = max(s, a.state(idx))
					}
					f = ast.Unparen(x.X)
					continue
				}
				break
			}
		}
		return s
	case *ast.IndexExpr:
		if rootedInC(e, a.info) {
			return opaque
		}
		return max(a.state(e.X), a.state(e.Index))
	case *ast.IndexListExpr:
		if rootedInC(e, a.info) {
			return opaque
		}
		s := a.state(e.X)
		for _, idx := range e.Indices {
			s = max(s, a.state(idx))
		}
		return s
	case *ast.SliceExpr:
		return max(a.state(e.X), a.state(e.Low), a.state(e.High), a.state(e.Max))
	case *ast.StarExpr:
		return a.state(e.X)
	case *ast.UnaryExpr:
		return a.state(e.X)
	case *ast.BinaryExpr:
		return max(a.state(e.X), a.state(e.Y))
	case *ast.TypeAssertExpr:
		return max(a.state(e.X), a.state(e.Type))
	case *ast.CompositeLit:
		if e.Type != nil {
			return a.typeExprState(e.Type) // []C.int{1}
		}
	case *ast.FuncLit:
		return a.typeExprState(e.Type) // func() C.int { ... }
	}
	return typed
}

// objState returns why the object obj, which an expression uses, has an
// invalid type (typed if it hasn't): from its declaration in this package,
// or opaque for an object of another package (that package has no errors,
// else nothing is rewritten, so its invalid types come from C).
func (a *analysis) objState(obj types.Object) state {
	switch obj := obj.(type) {
	case *types.Var, *types.Const:
		if !hasInvalid(obj.Type()) {
			return typed
		}
	case *types.Func:
		if sig, ok := obj.Type().(*types.Signature); !ok || !hasInvalid(sig.Results()) {
			return typed
		}
	default:
		return typed
	}
	switch obj.Pkg() {
	case nil:
		return typed
	case a.pkg:
	default:
		return opaque
	}
	if s, ok := a.memo[obj]; ok {
		return s
	}
	a.memo[obj] = unknown
	s := errored
	if !a.errorAtName(obj) {
		s = a.declState(a.decls[obj.Pos()])
	}
	a.memo[obj] = s
	return s
}

// typeNameState returns why the type of the type name tn has an invalid
// part (typed if it hasn't).
func (a *analysis) typeNameState(tn *types.TypeName) state {
	if !hasInvalid(tn.Type()) {
		return typed
	}
	switch tn.Pkg() {
	case nil:
		return typed
	case a.pkg:
	default:
		return opaque
	}
	if s, ok := a.types[tn]; ok {
		return s
	}
	a.types[tn] = typed
	s := errored
	if !a.errorAtName(tn) {
		s = a.declState(a.decls[tn.Pos()])
	}
	a.types[tn] = s
	return s
}

// declState returns the state of the declaration nodes of a name: unknown
// if they don't explain why its type has an invalid part.
func (a *analysis) declState(nodes []declNode) state {
	s := typed
	for _, d := range nodes {
		if d.isType {
			if a.errorIn(d.node) {
				return errored
			}
			s = max(s, a.typeState(d.node))
		} else {
			s = max(s, a.state(d.node.(ast.Expr)))
		}
	}
	if s == typed {
		return unknown
	}
	return s
}

// typeExprState returns why the type expression n has an invalid part: as
// typeState, but errored for an error inside n, and unknown if typeState
// finds no reason.
func (a *analysis) typeExprState(n ast.Expr) state {
	if a.errorIn(n) {
		return errored
	}
	if s := a.typeState(n); s != typed {
		return s
	}
	return unknown
}

// typeState returns why the type expression n, which has no error inside,
// has an invalid part: opaque for a part rooted in C (C.int,
// C.std.vector[int]), else the state of the type names it uses (typed if
// none has an invalid part).
func (a *analysis) typeState(n ast.Node) state {
	s := typed
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if rootedInC(n, a.info) {
				s = max(s, opaque)
				return false
			}
		case *ast.Ident:
			if tn, ok := a.info.Uses[n].(*types.TypeName); ok {
				s = max(s, a.typeNameState(tn))
			}
		}
		return true
	})
	return s
}

// isValidType reports whether t is a type and not go/types' invalid type.
func isValidType(t types.Type) bool {
	return t != nil && types.Unalias(t) != types.Typ[types.Invalid]
}

// hasInvalid reports whether the type t has an invalid part: the element
// of a pointer, slice, array, map or channel, a field, or a result type,
// also inside named types.
func hasInvalid(t types.Type) bool {
	return invalidIn(t, make(map[*types.Named]bool))
}

func invalidIn(t types.Type, seen map[*types.Named]bool) bool {
	if t == nil {
		return false
	}
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		return t.Kind() == types.Invalid
	case *types.Pointer:
		return invalidIn(t.Elem(), seen)
	case *types.Slice:
		return invalidIn(t.Elem(), seen)
	case *types.Array:
		return invalidIn(t.Elem(), seen)
	case *types.Chan:
		return invalidIn(t.Elem(), seen)
	case *types.Map:
		return invalidIn(t.Key(), seen) || invalidIn(t.Elem(), seen)
	case *types.Struct:
		for i := range t.NumFields() {
			if invalidIn(t.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Tuple:
		for i := range t.Len() {
			if invalidIn(t.At(i).Type(), seen) {
				return true
			}
		}
	case *types.Signature:
		return invalidIn(t.Results(), seen)
	case *types.Named:
		if seen[t] {
			return false
		}
		seen[t] = true
		return invalidIn(t.Underlying(), seen)
	}
	return false
}

// uninstantiated reports whether fun, the function of a call, is a generic function without an
// instance in Info.Instances: go/types couldn't infer or accept its type arguments, because an
// argument or a type argument comes from C/C++.
func (a *analysis) uninstantiated(fun ast.Expr) bool {
	id := instanceIdent(fun)
	if id == nil {
		return false
	}
	fn, ok := a.info.Uses[id].(*types.Func)
	if !ok || fn.Signature().TypeParams().Len() == 0 {
		return false
	}
	inst, ok := a.info.Instances[id]
	if !ok {
		return true
	}
	for i := range inst.TypeArgs.Len() {
		if hasInvalid(inst.TypeArgs.At(i)) {
			return true // go/types inferred an invalid type argument from an opaque argument
		}
	}
	return false
}

// genericFunc reports whether id names a generic function.
func (a *analysis) genericFunc(id *ast.Ident) bool {
	fn, ok := a.info.Uses[id].(*types.Func)
	return ok && fn.Signature().TypeParams().Len() > 0
}
