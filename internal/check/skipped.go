package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

// A skipped is an expression that go/types doesn't check (or doesn't
// record) because of an expression near it, the root of a subtree that
// go/types skips:
//   - skipAssert: the T of x.(T), when x is opaque (go/types stops at x);
//   - skipMake: a size argument of make(T, n, m), when T is opaque (it
//     stops at T);
//   - skipConst: a value of a constant declaration whose type comes from
//     C/C++, const k C.int = 1 (it stops at the type);
//   - skipKey: the key of an element of a composite literal of an opaque
//     type, std.map[int, int]{n + 4: 2} (it can't know whether the key is a
//     field name, so it doesn't look at it);
//   - skipGeneric: the function of a call of a generic function that
//     go/types couldn't instantiate, id(cv) with cv opaque;
//   - skipCall: an argument of a call of a C value that go/types can't
//     call, lib.CB(n) with var CB = x.get_cb() in lib, or h(n) with
//     var h Handle and type Handle C.int (it stops at the function, see
//     falseErrors);
//   - skipSlice: an index of a slice expression of such a value,
//     lib.CB[1:n] (it stops at the value);
//   - skipMapValue: the value of an element of a map literal whose key
//     type is a named type over a C/C++ type, map[Handle]int{1: n} (it
//     stops at the key, which it can't assign);
//   - skipRange: a target of a range that assigns, for k, v = range c, when
//     c is opaque, or a gocpp container of C/C++ values for the value
//     (var arr [3]C.int): go/types has no type for the key or the value
//     then, so it doesn't look at the target. A blank target is a blank
//     name on the left of an assignment (blankFrom).
//
// A root counts only when go/types really stopped at its near expression
// (see stopped), but for skipGeneric, skipMapValue and skipRange; else
// go/types checked the root as usual. A root of skipCall, skipSlice,
// skipMapValue or skipRange also counts only when go/types recorded
// nothing in it (a C function, C.f(n), has its arguments checked). After go/types, checkSkipped gives
// these roots real types with types.CheckExpr, where it can (the same way
// as the type arguments of opaque instantiations). Each expression in a root that still has no
// type afterwards is opaque when the root's near expression says so (see
// skippedFromC).
type skipped struct {
	root ast.Expr
	near ast.Expr // x, T, the declared type, the literal, the call
	kind skipKind
}

type skipKind int

const (
	skipAssert skipKind = iota
	skipMake
	skipConst
	skipKey
	skipGeneric
	skipCall
	skipSlice
	skipMapValue
	skipRange
)

// addSkipped records the skipped root root, whose near expression is near,
// if go/types really stopped at near (see stopped): each expression in root
// maps to it, until a root inside it (found later, because collect walks
// the files top-down) takes over its own part.
func (op *opaquePass) addSkipped(root, near ast.Expr, kind skipKind) {
	if kind != skipGeneric && kind != skipMapValue && kind != skipRange && !op.stopped(near) {
		return
	}
	sk := &skipped{root: root, near: near, kind: kind}
	op.roots = append(op.roots, sk)
	ast.Inspect(root, func(n ast.Node) bool {
		if e, ok := n.(ast.Expr); ok {
			op.skipped[e] = sk
		}
		return true
	})
}

// collectCall records the skipped roots of a call: the size arguments of
// make, the function of a generic call that go/types couldn't
// instantiate, and the arguments of a call that it couldn't check.
func (op *opaquePass) collectCall(call *ast.CallExpr) {
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok && len(call.Args) > 1 {
		if b, ok := op.info.Uses[id].(*types.Builtin); ok && b.Name() == "make" {
			for _, arg := range call.Args[1:] {
				op.addSkipped(arg, call.Args[0], skipMake)
			}
			return
		}
	}
	if tv := op.info.Types[call.Fun]; !tv.IsType() && !tv.IsBuiltin() {
		for _, arg := range call.Args {
			op.addUnrecorded(arg, call.Fun, skipCall)
		}
	}
	for e := call.Fun; e != nil; {
		if op.uninstantiated(e) || op.opaqueInstance(e) {
			op.addSkipped(call.Fun, call, skipGeneric)
			return
		}
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		default:
			e = nil
		}
	}
}

// addUnrecorded records the skipped root root (of skipCall, skipSlice,
// skipMapValue or skipRange) if go/types recorded nothing in it (and, but
// for skipMapValue and skipRange, stopped at near).
func (op *opaquePass) addUnrecorded(root, near ast.Expr, kind skipKind) {
	if root != nil && !recordedIn(root, op.info) {
		op.addSkipped(root, near, kind)
	}
}

// stopped reports whether go/types stopped at near, the x of x.(T), the T
// of make(T, n), the declared type of a constant or the composite literal
// of a key, before it got to the root next to it: near has no type, or its
// type is invalid (it is still invalid after checkSkipped, which doesn't
// check near). When near has a type that is valid but not complete
// (make([]C.int, n)), go/types checks the root itself.
func (op *opaquePass) stopped(near ast.Expr) bool {
	tv, ok := op.info.Types[near]
	return !ok || tv.Type == nil || tv.Type.Underlying() == types.Typ[types.Invalid]
}

// uninstantiated reports whether e is a generic function without an
// instance (see typed).
func (op *opaquePass) uninstantiated(e ast.Expr) bool {
	tv, ok := op.info.Types[e]
	if !ok {
		return false
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok || sig.TypeParams().Len() == 0 {
		return false
	}
	id := instanceIdent(e)
	if id == nil {
		return false
	}
	_, ok = op.info.Instances[id]
	return !ok
}

// opaqueInstance reports whether e is F[T] or F[T, U] with a generic function F and a type
// argument that comes from C/C++ (Mk[std.vector[int]]): go/types stops at the argument and records
// nothing for e or F, not even F's generic signature.
func (op *opaquePass) opaqueInstance(e ast.Expr) bool {
	var base ast.Expr
	var args []ast.Expr
	switch x := e.(type) {
	case *ast.IndexExpr:
		base, args = x.X, []ast.Expr{x.Index}
	case *ast.IndexListExpr:
		base, args = x.X, x.Indices
	default:
		return false
	}
	id := instanceIdent(base)
	if id == nil {
		return false
	}
	if fn, ok := op.info.Uses[id].(*types.Func); !ok || fn.Signature().TypeParams().Len() == 0 {
		return false
	}
	if _, ok := op.info.Instances[id]; ok {
		return false
	}
	return slices.ContainsFunc(args, op.comesFromC)
}

// skippedFromC reports whether the expressions of the skipped root sk come
// from C/C++:
//   - x of x.(T) or T of make(T, n) is opaque, or has a named type over a
//     C/C++ type (type V C.vec; go/types reports an error then, "hv
//     (variable of invalid type type H) is not an interface", "cannot make
//     V", which falseErrors drops);
//   - the type of the literal of a key, its own or the one it was left out
//     from, comes from C/C++ (P{X: 1} and []P{{X: 1}} with type P C.Point:
//     the literal is typed as P, but go/types doesn't look at its keys);
//   - the declared type of a constant comes from C/C++ (also through a
//     named type: const k H = 1 with type H C.int);
//   - an argument of the generic call is opaque.
//
// A root counts only where go/types stopped at near (see addSkipped), so
// near has no valid underlying type there.
func (op *opaquePass) skippedFromC(sk *skipped) bool {
	switch sk.kind {
	case skipConst:
		return op.comesFromC(sk.near)
	case skipKey:
		lit := sk.near.(*ast.CompositeLit)
		return op.opaqueExpr(lit) || op.litTypeFromC(lit)
	case skipMapValue:
		if t := op.info.Types[sk.near].Type; t != nil {
			m, _ := t.Underlying().(*types.Map)
			return m != nil && op.namedOverC(m.Key())
		}
		return false
	case skipGeneric:
		call := sk.near.(*ast.CallExpr)
		for _, arg := range call.Args {
			if op.opaqueExpr(arg) {
				return true
			}
		}
		for e := call.Fun; e != nil; {
			if op.opaqueInstance(e) {
				return true
			}
			switch x := e.(type) {
			case *ast.ParenExpr:
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			case *ast.IndexListExpr:
				e = x.X
			default:
				e = nil
			}
		}
		return false
	}
	return op.cValue(sk.near)
}

// litTypeFromC reports whether the type expression of the composite
// literal lit comes from C/C++: its own, or, if it was left out, the one
// of the literal that lit is an element of.
func (op *opaquePass) litTypeFromC(lit *ast.CompositeLit) bool {
	for lit != nil {
		if lit.Type != nil {
			return op.comesFromC(lit.Type)
		}
		lit = op.litParent[lit]
	}
	return false
}

// checkSkipped type-checks the skipped roots (see skipped) whose near
// expression comes from C/C++, after go/types and checkOpaqueArgs, with
// types.CheckExpr, and records the types in info, like checkOpaqueArgs does
// for type arguments. So cv.(strings.Builder), make(C.vec, n+1),
// const k C.int = 1 + 2 and the key n + 4 of std.map[int, int]{n + 4: 2}
// get their types, and the imports and the variables that they use count
// as used (added to used): go/types would report "imported and not used"
// and "declared and not used" for them. report gets the errors. The roots
// can be in any file of the package, also in one without import "C": the
// opaque expression next to them can come from another file or package
// (make(lib.Vec, n), with type Vec = C.vec in package lib).
//
// It never checks a root in which go/types (or the check of a root around
// it) recorded something already: CheckExpr would make new objects and
// scopes for it, and info would disagree with the scope tree. (stopped
// already keeps out the roots that go/types checks, and the check of an
// outer root stops at the same opaque expression as go/types, so this is
// only a guard.) Of the other
// roots, it doesn't check the function of a generic call (go/types checked
// it) and a key that is a literal whose type was left out. For a constant
// value that uses iota (CheckExpr can't check iota) and for a key that is
// a name, it only marks what they use (markUnchecked, markKeyName). Their
// expressions without types stay opaque.
//
// CheckExpr writes into info and into the scope tree of pkg, so
// checkSkipped runs once, after the last check of the package by go/types.
func checkSkipped(fset *token.FileSet, pkg *types.Package, files []*ast.File, info *types.Info,
	report func(pos token.Pos, err error), used *argUses) {
	op := newOpaquePass(pkg, info, files)
	// The dot imports of each file, for the names in its roots.
	dots := make(map[*token.File]map[*types.Package]*types.PkgName, len(files))
	for _, f := range files {
		dots[fset.File(f.Pos())] = dotImports(f, info)
	}
	for _, sk := range op.roots {
		if sk.kind == skipGeneric || recordedIn(sk.root, info) || !op.skippedFromC(sk) {
			continue
		}
		pos := sk.root.Pos()
		u := &useMarker{info: info, dots: dots[fset.File(pos)], used: used,
			scope: innermost(pkg, pos), pos: pos}
		switch r := sk.root.(type) {
		case *ast.CompositeLit:
			if sk.kind == skipKey && r.Type == nil {
				continue // stays opaque by the elided-literal rule
			}
		case *ast.Ident:
			if sk.kind == skipKey {
				markKeyName(u, pkg, r)
				continue
			}
		}
		if sk.kind == skipConst && usesIota(sk.root, u.scope) {
			markUnchecked(u, pkg, sk.root)
			continue
		}
		if err := types.CheckExpr(fset, pkg, pos, sk.root, info); err != nil {
			report(pos, err)
		} else if err := sizeCallError(fset, sk.root, info, "next to a C/C++ expression"); err != nil {
			report(pos, err) // CheckExpr computed it with Go's sizes (see gocppSizes)
		}
		if _, ok := ast.Unparen(sk.root).(*ast.Ident); ok && sk.kind == skipRange {
			continue // assigning to a variable doesn't use it, as in Go (i in for i = range c)
		}
		u.mark(sk.root)
	}
}

// recordedIn reports whether info has something about a node of the
// subtree root: a type, an object that a name declares or uses, a scope or
// an implicit object.
func recordedIn(root ast.Expr, info *types.Info) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		if e, ok := n.(ast.Expr); ok {
			if _, ok := info.Types[e]; ok {
				found = true
			}
		}
		if id, ok := n.(*ast.Ident); ok {
			_, def := info.Defs[id]
			_, use := info.Uses[id]
			found = found || def || use
		}
		_, scope := info.Scopes[n]
		_, implicit := info.Implicits[n]
		found = found || scope || implicit
		return !found
	})
	return found
}

// markKeyName marks the local variable that the key id of an opaque
// literal names, if it names one (std.map[int, int]{k: 1}), as used. The
// key can also be a field name of a C/C++ struct (C.Rect{w: 640}), so
// info gets no use of it (Phase 04's kind probe decides what it is). The
// cost: a local variable that is used nowhere else, but has the name of a
// field of the C/C++ struct of such a literal, is not reported as not
// used.
func markKeyName(u *useMarker, pkg *types.Package, id *ast.Ident) {
	if u.scope == nil {
		return
	}
	_, obj := u.scope.LookupParent(id.Name, id.Pos())
	if v, ok := obj.(*types.Var); ok && !v.IsField() && v.Pkg() == pkg && v.Parent() != pkg.Scope() {
		u.used.vars[v.Pos()] = true
	}
}

// markUnchecked marks what the root e, which CheckExpr doesn't check (a
// constant value with iota), uses: the packages of its qualified names
// (strconv in iota + strconv.IntSize) and the names that a dot import
// brought into the file, found in the scope of e.
func markUnchecked(u *useMarker, pkg *types.Package, e ast.Expr) {
	u.mark(e) // package names, through the scope
	if u.scope == nil {
		return
	}
	var walk func(n ast.Node) bool
	walk = func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			ast.Inspect(n.X, walk)
			return false // n.Sel is a name in a package or a field
		case *ast.Ident:
			_, obj := u.scope.LookupParent(n.Name, n.Pos())
			if obj != nil && obj.Pkg() != nil && obj.Pkg() != pkg && obj.Parent() == obj.Pkg().Scope() {
				if pn := u.dots[obj.Pkg()]; pn != nil {
					u.used.imports[pn] = true
				}
			}
		}
		return true
	}
	ast.Inspect(e, walk)
}

// usesIota reports whether e uses the predeclared iota, seen from scope
// (any name iota, if scope is nil).
func usesIota(e ast.Expr, scope *types.Scope) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "iota" {
			if scope == nil {
				found = true
			} else if _, obj := scope.LookupParent("iota", id.Pos()); obj == types.Universe.Lookup("iota") {
				found = true
			}
		}
		return !found
	})
	return found
}
