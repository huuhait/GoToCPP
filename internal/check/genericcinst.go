package check

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// A genericReason says why an instance of a generic function or type would be refused by the lowering:
// the generic type of the refused instance (Box) and what the function or type does to make it ("makes
// Box[T]", "calls Wrap, which makes Box[T]", "has Box[C.std.string]").
type genericReason struct {
	typ, desc string
}

// genericInstMsg is the error of a use of the generic function fn whose instance would make an instance
// of a generic type with a C/C++ type argument (the words of lower's, insttypes.go: instOf, and why).
func genericInstMsg(fn string, r genericReason) string {
	return "an instance of the generic type " + r.typ + " with a C/C++ type argument of a generic function is not supported yet (" +
		fn + " " + r.desc + "): the type arguments of a generic type are Go types"
}

// genericSums knows, for the generic functions and types of the packages checked so far, what their
// instances make that the lowering refuses (ADR-030 point 5, ADR-141): an instance of a generic type
// with a C/C++ type argument. For each type parameter, whether a C/C++ type argument makes one (the
// lowering gives the instance a stand-in for it, and every generic type that the signature or the body
// instantiates with it is such an instance: Box[T], iter.Seq[T], a type declared in the function, which
// takes the function's type parameters, also through the generic functions it calls); and whether
// every instance makes one, because the declaration writes one (Box[std.string] in a generic body or
// in a field of a generic type), which the lowering only meets when it makes an instance.
//
// Only the parameters, the results and the body of a function are walked (not its type parameter list,
// whose constraints the lowering makes no instance of). The summaries of declarations that use each
// other are computed to a fixpoint (pending, cycle).
type genericSums struct {
	funcs map[*types.Func]funcSrc
	types map[*types.TypeName]typeSrc
	memo  map[types.Object]*genericSum

	depth   int
	cycle   bool
	pending []*genericSum

	cxxUse map[*types.Func]bool // usesCxx, by function
}

// A genericSum is the summary of one generic function or type (genericSums).
type genericSum struct {
	obj    types.Object
	params []genericReason // per type parameter; the zero reason when it makes nothing
	always genericReason   // when every instance makes one
	done   bool
}

// A funcSrc is the declaration of a generic function and the Info of its package.
type funcSrc struct {
	decl *ast.FuncDecl
	info *types.Info
	pkg  *Package
}

// A typeSrc is the declaration of a generic type, its methods and the Info of its package.
type typeSrc struct {
	spec    *ast.TypeSpec
	methods []*ast.FuncDecl
	info    *types.Info
}

func newGenericSums() *genericSums {
	return &genericSums{funcs: make(map[*types.Func]funcSrc), types: make(map[*types.TypeName]typeSrc),
		memo: make(map[types.Object]*genericSum)}
}

// add notes the generic functions and the generic types (with their methods) of the checked package p.
func (s *genericSums) add(p *Package) {
	if s == nil || p.Info == nil {
		return
	}
	for _, f := range p.astFiles() {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				fn, ok := p.Info.Defs[d.Name].(*types.Func)
				if !ok || d.Body == nil {
					continue
				}
				if fn.Signature().Recv() != nil {
					if tn := recvTypeName(fn.Signature().Recv().Type()); tn != nil && fn.Signature().RecvTypeParams().Len() > 0 {
						src := s.types[tn]
						src.methods, src.info = append(src.methods, d), p.Info
						s.types[tn] = src
					}
					continue
				}
				if d.Type.TypeParams != nil {
					s.funcs[fn] = funcSrc{decl: d, info: p.Info, pkg: p}
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.TypeParams == nil || ts.Assign.IsValid() {
						continue
					}
					if tn, ok := p.Info.Defs[ts.Name].(*types.TypeName); ok {
						src := s.types[tn]
						src.spec, src.info = ts, p.Info
						s.types[tn] = src
					}
				}
			}
		}
	}
}

// recvTypeName returns the type name of a receiver type T or *T, or nil.
func recvTypeName(t types.Type) *types.TypeName {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Origin().Obj()
	}
	return nil
}

// summary returns the summary of the generic function or type obj, made the first time. A function
// whose declaration is not known (a package of export data: the editor's dependencies) is summarized
// from its signature; a type without one makes nothing.
func (s *genericSums) summary(obj types.Object) *genericSum {
	if fn, ok := obj.(*types.Func); ok {
		obj = fn.Origin()
	}
	if e := s.memo[obj]; e != nil {
		if !e.done {
			s.cycle = true // in progress, or waiting for the fixpoint of its cycle
		}
		return e
	}
	e := &genericSum{obj: obj}
	switch o := obj.(type) {
	case *types.Func:
		e.params = make([]genericReason, o.Signature().TypeParams().Len())
	case *types.TypeName:
		if n, ok := o.Type().(*types.Named); ok {
			e.params = make([]genericReason, n.TypeParams().Len())
		}
	}
	s.memo[obj] = e
	s.pending = append(s.pending, e)
	s.depth++
	s.fill(e)
	s.depth--
	if s.depth == 0 {
		for changed := s.cycle; changed; {
			changed = false
			for _, p := range s.pending {
				if s.fill(p) {
					changed = true
				}
			}
		}
		for _, p := range s.pending {
			p.done = true
		}
		s.pending, s.cycle = nil, false
	}
	return e
}

// fill walks the declaration of e's function or type and adds what it finds to e; it reports whether
// e changed (a reason is set once, so the fixpoint ends).
func (s *genericSums) fill(e *genericSum) bool {
	changed := false
	setParam := func(i int, r genericReason) {
		if i < len(e.params) && e.params[i] == (genericReason{}) {
			e.params[i], changed = r, true
		}
	}
	setAlways := func(r genericReason) {
		if e.always == (genericReason{}) {
			e.always, changed = r, true
		}
	}
	switch o := e.obj.(type) {
	case *types.Func:
		tparams := o.Signature().TypeParams()
		src, ok := s.funcs[o]
		if !ok {
			// No declaration: the instances that the signature makes.
			sig := o.Signature()
			for _, tuple := range []*types.Tuple{sig.Params(), sig.Results()} {
				eachInstance(tuple, nil, func(n *types.Named) {
					s.markInstance(n, tparams, setParam, setAlways)
				})
			}
			return changed
		}
		w := &summaryWalk{s: s, info: src.info, tparams: tparams, setParam: setParam, setAlways: setAlways}
		w.walk(src.decl.Type.Params)
		w.walk(src.decl.Type.Results)
		w.walk(src.decl.Body)
	case *types.TypeName:
		src, ok := s.types[o]
		if !ok {
			return false
		}
		has := func(r genericReason) { // what a generic type makes, it has
			r.desc = "has " + strings.TrimPrefix(r.desc, "makes ")
			setAlways(r)
		}
		if named, ok := o.Type().(*types.Named); ok && src.spec != nil {
			w := &summaryWalk{s: s, info: src.info, tparams: named.TypeParams(), setParam: setParam, setAlways: has}
			w.walk(src.spec.Type)
		}
		for _, m := range src.methods {
			fn, ok := src.info.Defs[m.Name].(*types.Func)
			if !ok {
				continue
			}
			// The method's own copies of the type parameters, by index.
			w := &summaryWalk{s: s, info: src.info, tparams: fn.Signature().RecvTypeParams(), setParam: setParam, setAlways: has}
			w.walk(m.Type.Params)
			w.walk(m.Type.Results)
			w.walk(m.Body)
		}
	}
	return changed
}

// markInstance adds what the instance n of a generic type, met in a declaration whose type parameters
// are tparams, says: a type argument with a C/C++ part makes every instance refused, a type argument
// that mentions a type parameter makes the instances with a C/C++ type argument for it refused, and an
// instance of a generic type that is always refused is refused.
func (s *genericSums) markInstance(n *types.Named, tparams *types.TypeParamList, setParam func(int, genericReason), setAlways func(genericReason)) {
	if constraintOnly(n) {
		return // a constraint (interface{ ~[]T }): the lowering makes no instance of it
	}
	name := n.Origin().Obj().Name()
	shown := types.TypeString(n, func(p *types.Package) string {
		if p == n.Origin().Obj().Pkg() && tparams != nil && tparams.Len() > 0 && p == tparams.At(0).Obj().Pkg() {
			return ""
		}
		return p.Name()
	})
	for a := range n.TypeArgs().Types() {
		if typeHasC(a, nil) {
			setAlways(genericReason{name, "makes " + shown})
			return
		}
	}
	for i := range tparams.Len() {
		for a := range n.TypeArgs().Types() {
			if mentionsParam(a, tparams.At(i), nil) {
				setParam(i, genericReason{name, "makes " + shown})
			}
		}
	}
	if sub := s.summary(n.Origin().Obj()); sub.always != (genericReason{}) {
		setAlways(genericReason{sub.always.typ, "makes " + shown + ", which " + sub.always.desc})
	}
}

// constraintOnly reports whether the named type n is an interface that only a constraint can be (a
// type set with terms), not a type of values.
func constraintOnly(n *types.Named) bool {
	it, ok := n.Underlying().(*types.Interface)
	return ok && !it.IsMethodSet()
}

// A summaryWalk walks a part of a generic declaration for genericSums.fill.
type summaryWalk struct {
	s         *genericSums
	info      *types.Info
	tparams   *types.TypeParamList
	setParam  func(int, genericReason)
	setAlways func(genericReason)
}

func (w *summaryWalk) walk(root ast.Node) {
	switch r := root.(type) {
	case nil:
		return
	case *ast.FieldList:
		if r == nil {
			return
		}
	case *ast.BlockStmt:
		if r == nil {
			return
		}
	}
	info := w.info
	instances := func(t types.Type) {
		eachInstance(t, nil, func(n *types.Named) {
			for a := range n.TypeArgs().Types() {
				if typeHasC(a, nil) {
					return // a written instance with a C/C++ type argument: met at its expression
				}
			}
			w.s.markInstance(n, w.tparams, w.setParam, w.setAlways)
		})
	}
	calls := make(map[ast.Expr]*ast.CallExpr)
	ast.Inspect(root, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			calls[ast.Unparen(c.Fun)] = c
		}
		if e, ok := n.(ast.Expr); ok {
			if origin := genericTypeOrigin(e, info); origin != nil && !constraintOnly(origin) {
				for _, a := range typeArgExprs(e) {
					if at := info.TypeOf(a); at == nil || typeHasC(at, nil) {
						w.setAlways(genericReason{origin.Obj().Name(), "makes " + types.ExprString(e)})
						return false
					}
				}
			}
			if u, fn, ok := w.s.use(e, info, calls); ok {
				w.markUse(u, fn)
				if tv, ok := info.Types[e]; ok && tv.Type != nil {
					instances(tv.Type)
				}
				return false // (its name inside, F of F[A], is this use)
			}
			if tv, ok := info.Types[e]; ok && tv.Type != nil {
				instances(tv.Type)
			}
		}
		if id, ok := n.(*ast.Ident); ok {
			if obj := info.Defs[id]; obj != nil {
				if tn, ok := obj.(*types.TypeName); ok {
					_, isParam := tn.Type().(*types.TypeParam)
					if !tn.IsAlias() && !isParam && obj.Pkg() != nil && obj.Parent() != obj.Pkg().Scope() {
						// A type declared in a generic function takes the function's type parameters (ADR-141).
						for i := range w.tparams.Len() {
							w.setParam(i, genericReason{tn.Name(), "declares " + tn.Name()})
						}
					}
				} else if obj.Type() != nil {
					instances(obj.Type())
				}
			}
		}
		return true
	})
}

// markUse adds what the use u of the generic function fn in the walked declaration makes.
func (w *summaryWalk) markUse(u genericUse, fn *types.Func) {
	via := func(r genericReason) genericReason {
		return genericReason{r.typ, "calls " + fn.Name() + ", which " + r.desc}
	}
	if u.always != (genericReason{}) {
		w.setAlways(via(u.always))
	}
	for j, r := range u.params {
		if r == (genericReason{}) || j >= len(u.args) {
			continue
		}
		a := u.args[j]
		mentions := false
		for i := range w.tparams.Len() {
			if a != nil && mentionsParam(a, w.tparams.At(i), nil) {
				w.setParam(i, via(r))
				mentions = true
			}
		}
		if !mentions && u.bad[j] {
			w.setAlways(via(r)) // a C/C++ type argument written in the declaration
		}
	}
}

// A genericUse is what a use of a generic function asks of its instance: the function's summary, the
// type arguments (nil where unknown) and which of them are C/C++ types for the lowering (bad).
type genericUse struct {
	*genericSum
	args []types.Type
	bad  []bool
}

// use returns the generic function that the expression e uses (F, pkg.F, F[A], pkg.F[A, B]), with its
// summary and its type arguments, or false. A type argument is bad as the lowering makes it a stand-in
// or bakes a C/C++ part in (lower's opaqueInstRef): a written type with an invalid or a C/C++ part,
// or one not written that go/types did not infer as a Go type without C/C++ parts (unless lower's
// shapeArgs takes it from the call's map argument). A type parameter of the declaration around the use
// is not bad: the summary of that declaration follows it.
func (s *genericSums) use(e ast.Expr, info *types.Info, calls map[ast.Expr]*ast.CallExpr) (genericUse, *types.Func, bool) {
	var idx []ast.Expr
	switch x := e.(type) {
	case *ast.IndexExpr, *ast.IndexListExpr:
		idx = typeArgExprs(x)
	case *ast.Ident, *ast.SelectorExpr:
	default:
		return genericUse{}, nil, false
	}
	id := instanceIdent(e)
	if id == nil {
		return genericUse{}, nil, false
	}
	fn, ok := info.Uses[id].(*types.Func)
	if !ok || fn.Signature().TypeParams().Len() == 0 || fn.Signature().Recv() != nil {
		return genericUse{}, nil, false
	}
	inst, hasInst := info.Instances[id]
	tparams := fn.Signature().TypeParams()
	u := genericUse{args: make([]types.Type, tparams.Len()), bad: make([]bool, tparams.Len())}
	for i := range tparams.Len() {
		var t types.Type
		switch {
		case i < len(idx):
			t = info.TypeOf(idx[i])
		case hasInst && i < inst.TypeArgs.Len():
			t = inst.TypeArgs.At(i)
		default:
			t = shapedArg(fn, i, calls[e], info) // lower's shapeArgs: K of M ~map[K]V from the argument
		}
		u.args[i] = t
		u.bad[i] = t == nil || !cxxComplete(t) || typeHasC(t, nil)
	}
	u.genericSum = s.summary(fn)
	return u, fn, true
}

// checkGenericInstances reports each use of a generic function (a call or a func value) outside generic
// declarations whose instance would make an instance of a generic type with a C/C++ type argument
// (genericSums): at the use, which the lowering of the instance would reach from there. A use inside a
// generic declaration is in that declaration's summary, and reported where it is instantiated.
func checkGenericInstances(files []*ast.File, info *types.Info, sums *genericSums, report func(pos token.Pos, msg string)) {
	if sums == nil {
		return
	}
	calls := make(map[ast.Expr]*ast.CallExpr) // the calls by their functions
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if genericDecl(n) {
				return false
			}
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			if c, ok := e.(*ast.CallExpr); ok {
				calls[ast.Unparen(c.Fun)] = c
				return true
			}
			u, fn, ok := sums.use(e, info, calls)
			if !ok {
				return true
			}
			if u.always != (genericReason{}) {
				report(e.Pos(), genericInstMsg(fn.Name(), u.always))
				return false
			}
			for i, r := range u.params {
				if r != (genericReason{}) && u.bad[i] {
					report(e.Pos(), genericInstMsg(fn.Name(), r))
					return false
				}
			}
			return false // (its name inside, F of F[A], is this use)
		})
	}
}

// genericDecl reports whether n is a generic declaration, whose body the lowering only lowers for its
// instances: a generic function, a method of a generic type, the declaration of a generic type.
func genericDecl(n ast.Node) bool {
	switch d := n.(type) {
	case *ast.FuncDecl:
		if d.Type.TypeParams != nil {
			return true
		}
		if d.Recv != nil && len(d.Recv.List) == 1 {
			t := d.Recv.List[0].Type
			if st, ok := t.(*ast.StarExpr); ok {
				t = st.X
			}
			switch ast.Unparen(t).(type) {
			case *ast.IndexExpr, *ast.IndexListExpr:
				return true
			}
		}
	case *ast.TypeSpec:
		return d.TypeParams != nil
	}
	return false
}

// shapedArg returns the type that the lowering gives the type parameter number i of the generic
// function fn, which has no type argument written or inferred, from the argument of call for a
// parameter whose type is a type parameter M ~map[K]V with K or V that parameter (lower's shapeArgs and
// argTypeOf: Put(m, k, cv) with m map[int]std.string gives K int), or nil.
func shapedArg(fn *types.Func, i int, call *ast.CallExpr, info *types.Info) types.Type {
	sig := fn.Signature()
	if call == nil || call.Ellipsis.IsValid() || len(call.Args) != sig.Params().Len() {
		return nil
	}
	tp := sig.TypeParams().At(i)
	for k := range sig.Params().Len() {
		if sig.Variadic() && k == sig.Params().Len()-1 {
			break
		}
		m, ok := types.Unalias(sig.Params().At(k).Type()).(*types.TypeParam)
		if !ok {
			continue
		}
		core, ok := CoreType(m.Constraint()).(*types.Map)
		if !ok {
			continue
		}
		at, ok := types.Unalias(info.TypeOf(call.Args[k])).(*types.Map)
		if !ok {
			continue
		}
		switch tp {
		case types.Unalias(core.Key()):
			return at.Key()
		case types.Unalias(core.Elem()):
			return at.Elem()
		}
	}
	return nil
}

// eachInstance calls f for each instance of a generic type in t (a named type with type arguments),
// also in the type arguments of those and in the parts of composite types, not in the underlying
// types of named types.
func eachInstance(t types.Type, seen map[types.Type]bool, f func(*types.Named)) {
	if t == nil {
		return
	}
	if seen == nil {
		seen = make(map[types.Type]bool)
	}
	if seen[t] {
		return
	}
	seen[t] = true
	switch t := t.(type) {
	case *types.Alias:
		eachInstance(types.Unalias(t), seen, f)
	case *types.Named:
		if t.TypeArgs().Len() > 0 {
			f(t)
			for a := range t.TypeArgs().Types() {
				eachInstance(a, seen, f)
			}
		}
	case *types.Pointer:
		eachInstance(t.Elem(), seen, f)
	case *types.Slice:
		eachInstance(t.Elem(), seen, f)
	case *types.Array:
		eachInstance(t.Elem(), seen, f)
	case *types.Chan:
		eachInstance(t.Elem(), seen, f)
	case *types.Map:
		eachInstance(t.Key(), seen, f)
		eachInstance(t.Elem(), seen, f)
	case *types.Signature:
		eachInstance(t.Params(), seen, f)
		eachInstance(t.Results(), seen, f)
	case *types.Tuple:
		for v := range t.Variables() {
			eachInstance(v.Type(), seen, f)
		}
	case *types.Struct:
		for i := range t.NumFields() {
			eachInstance(t.Field(i).Type(), seen, f)
		}
	}
}

// mentionsParam reports whether the type t mentions the type parameter tp.
func mentionsParam(t types.Type, tp *types.TypeParam, seen map[types.Type]bool) bool {
	if t == nil {
		return false
	}
	if seen == nil {
		seen = make(map[types.Type]bool)
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t := t.(type) {
	case *types.TypeParam:
		return t == tp
	case *types.Alias:
		return mentionsParam(types.Unalias(t), tp, seen)
	case *types.Named:
		for a := range t.TypeArgs().Types() {
			if mentionsParam(a, tp, seen) {
				return true
			}
		}
	case *types.Pointer:
		return mentionsParam(t.Elem(), tp, seen)
	case *types.Slice:
		return mentionsParam(t.Elem(), tp, seen)
	case *types.Array:
		return mentionsParam(t.Elem(), tp, seen)
	case *types.Chan:
		return mentionsParam(t.Elem(), tp, seen)
	case *types.Map:
		return mentionsParam(t.Key(), tp, seen) || mentionsParam(t.Elem(), tp, seen)
	case *types.Signature:
		return mentionsParam(t.Params(), tp, seen) || mentionsParam(t.Results(), tp, seen)
	case *types.Tuple:
		for v := range t.Variables() {
			if mentionsParam(v.Type(), tp, seen) {
				return true
			}
		}
	case *types.Struct:
		for i := range t.NumFields() {
			if mentionsParam(t.Field(i).Type(), tp, seen) {
				return true
			}
		}
	case *types.Interface:
		for i := range t.NumMethods() {
			if mentionsParam(t.Method(i).Type(), tp, seen) {
				return true
			}
		}
	}
	return false
}

// usesCxx reports whether the body of the generic function fn uses C/C++ names or values, as the
// lowering decides it (lower/generics.go: genericFunc.usesCxx): a name of C (the loader rewrites the
// header packages' names to C), or an opaque expression. Such a function is instantiated only by its
// own package. False for a function whose declaration is not known (export data).
func (s *genericSums) usesCxx(fn *types.Func) bool {
	if s == nil {
		return false
	}
	fn = fn.Origin()
	if v, ok := s.cxxUse[fn]; ok {
		return v
	}
	src, ok := s.funcs[fn]
	found := false
	if ok && src.pkg != nil {
		ast.Inspect(src.decl, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if pn, ok := src.info.Uses[n].(*types.PkgName); ok && pn.Imported().Path() == "C" {
					found = true
				}
			case ast.Expr:
				if src.pkg.Opaque.IsOpaque(n) {
					found = true
				}
			}
			return !found
		})
	}
	if s.cxxUse == nil {
		s.cxxUse = make(map[*types.Func]bool)
	}
	s.cxxUse[fn] = found
	return found
}

// checkForeignCxxGenerics reports each use (a call or a func value) outside generic declarations of a
// generic function of another package whose body uses C/C++ names: the lowering instantiates a
// generic function in the package that uses it, with that package's C/C++ state, so it refuses such
// a one (lower/generics.go: instRef) at the same place, with the same words. (Inside a generic
// declaration, the use is the build's, where the declaration is instantiated, as for genericSums.)
func checkForeignCxxGenerics(files []*ast.File, pkg *types.Package, info *types.Info, sums *genericSums, report func(pos token.Pos, msg string)) {
	if sums == nil || len(sums.funcs) == 0 {
		return
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if genericDecl(n) {
				return false
			}
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			switch e.(type) {
			case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
			default:
				return true
			}
			id := instanceIdent(e)
			if id == nil {
				return true
			}
			fn, ok := info.Uses[id].(*types.Func)
			if !ok || fn.Pkg() == nil || fn.Pkg() == pkg || fn.Signature().TypeParams().Len() == 0 || fn.Signature().Recv() != nil {
				return true
			}
			if sums.usesCxx(fn) {
				report(e.Pos(), fmt.Sprintf("%s.%s uses C/C++ names, so it can only be instantiated in its own package (Phase 12)", fn.Pkg().Name(), fn.Name()))
			}
			return false // (the name inside, F of F[A], is this use)
		})
	}
}
