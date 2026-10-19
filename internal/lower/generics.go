package lower

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/load"
)

// Generic functions (Phase 07 steps 7.1 and 7.2, docs/adr/030-generic-instances.md).
//
// A generic function is lowered once for each list of type arguments that a call or a func value
// names: an instance, a C++ function of its own that the code of the instantiating package
// calls by a name. The body is lowered exactly as the body of a function written with the type
// arguments would be: every expression has the type that go/types recorded for it, with the type
// parameters replaced by the type arguments (subst.go), so each operation (a + b on strings and
// on small integers, T(x), len of a slice or a string, any(x) of an int and of an int64, the
// method call x.String() for a value or a pointer receiver) is the one that the lowering already
// knows for that type. A C++ template cannot be this precise: int and int64 are the same C++
// type, an operator has to be chosen by the type, and a method call has several forms.
//
// An instance is written where it is used: as an ordinary function (not static; its name has a
// hash of the translation unit) of the namespace of the package that asked for it, in the
// translation unit (the .gp file) of the function that asked, before the functions of the unit.
// Two units that need the same instance have one copy each. The instances of one unit are found
// while its functions are lowered, and lowered one after the other (drainInstances), as the
// instances of the instances are found.
//
// The instance of a function of another package is lowered by the asking package, from the
// other package's syntax tree and type information (genericFunc.owner): the names in it are
// the other package's and are written fully qualified, as for any use of that package.

// A genericFunc is a generic function of a package that is lowered, with what its instances need.
type genericFunc struct {
	fn    *types.Func
	decl  *ast.FuncDecl
	owner *gen // the package that declares it

	snap *genSnap // the type information of the declaration, as go/types recorded it

	// cxx is 0 before usesCxx looked at the body, 1 if the body uses C/C++ names or values, else 2.
	cxx int

	// callable marks the parameters whose type is a type parameter of a macro that stands for a
	// closure (callableParam): the instance takes them as a C++ template parameter, so that a lazy
	// argument or the code of a wrapped function is a plain lambda, with no gp::func and no
	// allocation. It is nil for a function with none.
	callable []bool
}

// constraintFunc returns the func type T of a type parameter whose constraint is ~T, or nil.
func constraintFunc(tp *types.TypeParam) types.Type {
	iface, ok := tp.Constraint().Underlying().(*types.Interface)
	if !ok || iface.NumEmbeddeds() != 1 {
		return nil
	}
	u, ok := iface.EmbeddedType(0).(*types.Union)
	if !ok || u.Len() != 1 {
		return nil
	}
	if _, ok := u.Term(0).Type().Underlying().(*types.Signature); !ok {
		return nil
	}
	return u.Term(0).Type()
}

// callableParam reports whether the type t of a parameter is a type parameter that the macros of
// Phase 10 add for a lazy parameter (gp_L_name, constrained to ~func() T) or for the code of a wrap
// macro (gp_B, ~func()): the loader's rewrite (internal/load/macros.go) names them so.
func callableParam(t types.Type) (string, bool) {
	tp, ok := types.Unalias(t).(*types.TypeParam)
	if !ok {
		return "", false
	}
	name := tp.Obj().Name()
	return name, strings.HasPrefix(name, "gp_L_") || strings.HasPrefix(name, "gp_LN_") || name == "gp_B" || name == "gp_BN"
}

// macroSig returns the signature of the macro that the function fn is, or nil if it is not a macro instance.
// The loader's rewrite (internal/load/macros.go) turns a macro into a generic function of the same name in the
// package that declares it, and Package.Macros lists them, so a function is a macro by the object (its package
// and its name), never by the names of its parameters: gocpp allows gp_ names outside macros.
func (g *gen) macroSig(fn *types.Func) *load.MacroSig {
	if fn == nil || fn.Pkg() == nil {
		return nil
	}
	if sig, _ := fn.Type().(*types.Signature); sig == nil || sig.Recv() != nil {
		return nil
	}
	if o := g.other(fn.Pkg()); o != nil && o.pkg.Load != nil {
		return o.pkg.Load.Macros[fn.Name()]
	}
	for _, dep := range g.pkg.Load.Imports {
		if dep.ImportPath == fn.Pkg().Path() {
			return dep.Macros[fn.Name()]
		}
	}
	return nil
}

// viewIndex reports whether the parameter i of the macro m is one of its hidden string arguments: the file (0), the
// function (2) and the text of the arguments that the body gives to #text (3 and on), after the line (1) (the order
// of macroDecl in internal/load/macros.go). They are the place of the use and the text of its arguments, always
// string constants of the use. They are not gp::string but gp::static_view in C++ (a pointer and a length: trivial to
// copy and to destroy), and the body makes a gp::string where it reads one (ident). A gp::string argument is not
// trivially destructible, and a call that may throw (a lazy message) has to keep four of them alive and destroy
// them on every iteration of a hot loop (docs/benchmarks/must-macros.md).
func viewIndex(m *load.MacroSig, i int) bool {
	return m != nil && (i == 0 || i == 2 || i >= 3 && i < 3+len(m.Text))
}

// viewVar reports whether v is a hidden string parameter of the macro that the function being lowered is
// (recorded by funcInfo.signature).
func (g *gen) viewVar(v *types.Var) bool { return g.viewVars[v] }

// refParam reports whether the type t of a parameter is a closure type parameter of a macro that the
// macro only calls (gp_LN_x, gp_BN): the lambda for it can capture by reference.
func refParam(t types.Type) bool {
	tp, ok := types.Unalias(t).(*types.TypeParam)
	if !ok {
		return false
	}
	name := tp.Obj().Name()
	return strings.HasPrefix(name, "gp_LN_") || name == "gp_BN"
}

// usesCxx reports whether the body of the generic function uses C/C++ names or values: a name of
// C (C.int, or the header and namespace uses that the loader rewrote to C), or an opaque expression.
func (gf *genericFunc) usesCxx() bool {
	if gf.cxx == 0 {
		gf.cxx = 2
		info := gf.owner.info
		ast.Inspect(gf.decl, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if pn, ok := info.Uses[n].(*types.PkgName); ok && pn.Imported().Path() == "C" {
					gf.cxx = 1
				}
			case ast.Expr:
				if gf.owner.pkg.Opaque.IsOpaque(n) {
					gf.cxx = 1
				}
			}
			return gf.cxx != 1
		})
	}
	return gf.cxx == 1
}

// A genSnap is the type information go/types recorded for the nodes of a generic function. The
// information is replaced, in the maps of the package, by the information of the instance being
// lowered (apply), so the original is kept.
type genSnap struct {
	types     map[ast.Expr]types.TypeAndValue
	defs      map[*ast.Ident]types.Object
	uses      map[*ast.Ident]types.Object
	implicits map[ast.Node]types.Object
	instances map[*ast.Ident]typeInst
	sels      map[*ast.SelectorExpr]*selection
}

// A typeInst is an entry of Info.Instances with its own representation: go/types' has a list of
// type arguments that only the checker can make.
type typeInst struct {
	targs []types.Type
	typ   types.Type
}

// An instance is a generic function with type arguments, lowered for one translation unit.
type instance struct {
	gf    *genericFunc
	targs []types.Type
	name  string // the C++ name, in the namespace of the package that asked
	unit  string // the unit that asked

	// open are the type arguments that are C++ template parameters of the instance (opaque
	// instances): it is a function template over them.
	open []*types.Named
}

// An instKey identifies an instance in a unit.
type instKey struct{ unit, key string }

// collectGeneric records the generic function d: it has no C++ of its own until it is
// instantiated.
func (g *gen) collectGeneric(d *ast.FuncDecl) {
	fn, _ := g.info.Defs[d.Name].(*types.Func)
	if fn == nil {
		g.errorf(d.Pos(), "internal error: no object for func %s", d.Name.Name)
		return
	}
	if d.Body == nil {
		g.unsupported(d.Pos(), "a generic function without a body")
		return
	}
	if d.Name.Name == "_" || d.Name.Name == "init" || d.Name.Name == "main" {
		return // go/types rejects them
	}
	if g.generics == nil {
		g.generics = make(map[*types.Func]*genericFunc)
	}
	g.privateInfoOnce()
	gf := &genericFunc{fn: fn, decl: d, owner: g}
	params := fn.Signature().Params()
	for i := range params.Len() {
		if _, ok := callableParam(params.At(i).Type()); ok {
			if gf.callable == nil {
				gf.callable = make([]bool, params.Len())
			}
			gf.callable[i] = true
		}
	}
	g.generics[fn] = gf
}

// privateInfo gives the package maps of its own: an instance replaces the entries of the nodes of
// its generic function (apply), and the maps belong to the checker's result otherwise.
func (g *gen) privateInfoOnce() {
	if g.infoPrivate {
		return
	}
	g.infoPrivate = true
	old := g.info
	cp := *old.Info
	cp.Types = maps.Clone(old.Types)
	cp.Defs = maps.Clone(old.Defs)
	cp.Uses = maps.Clone(old.Uses)
	cp.Implicits = maps.Clone(old.Implicits)
	g.info = &tinfo{
		Info:       &cp,
		Selections: maps.Clone(old.Selections),
		Instances:  maps.Clone(old.Instances),
	}
}

// snapshot returns the type information of the generic function, as go/types recorded it.
func (gf *genericFunc) snapshot() *genSnap {
	if gf.snap != nil {
		return gf.snap
	}
	info := gf.owner.info
	s := &genSnap{
		types:     make(map[ast.Expr]types.TypeAndValue),
		defs:      make(map[*ast.Ident]types.Object),
		uses:      make(map[*ast.Ident]types.Object),
		implicits: make(map[ast.Node]types.Object),
		instances: make(map[*ast.Ident]typeInst),
		sels:      make(map[*ast.SelectorExpr]*selection),
	}
	ast.Inspect(gf.decl, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if e, ok := n.(ast.Expr); ok {
			if tv, ok := info.Types[e]; ok {
				s.types[e] = tv
			}
		}
		if obj, ok := info.Implicits[n]; ok {
			s.implicits[n] = obj
		}
		switch n := n.(type) {
		case *ast.Ident:
			if obj, ok := info.Defs[n]; ok {
				s.defs[n] = obj
			}
			if obj, ok := info.Uses[n]; ok {
				s.uses[n] = obj
			}
			if in, ok := info.Instances[n]; ok {
				s.instances[n] = in
			}
		case *ast.SelectorExpr:
			if sel, ok := info.Selections[n]; ok {
				s.sels[n] = sel
			}
		}
		return true
	})
	gf.snap = s
	return s
}

// apply puts the type information of the instance with the type arguments targs into the
// package's maps, for the nodes of the generic function, and returns the function that
// stands for the instance, with its signature. The variables of the instance are new
// objects, so that the tables of the lowering, which are keyed by variable, cannot mix two
// instances up.
func (gf *genericFunc) apply(targs []types.Type, ctxt *types.Context, cxxNamed map[*types.Named]bool) (*types.Func, error) {
	info := gf.owner.info
	snap := gf.snapshot()
	osig := gf.fn.Signature()
	tparams := osig.TypeParams()
	if tparams.Len() == 0 {
		tparams = osig.RecvTypeParams() // a method of a generic type: the receiver's type parameters
	}
	s := newSubster(tparams, targs, ctxt)
	if fnScope := gf.fn.Scope(); fnScope != nil {
		s.local = func(l *types.Named) types.Type {
			if !localTypeIn(l, fnScope) {
				return nil
			}
			return gf.owner.localInstance(l, gf.fn, tparams, targs, ctxt)
		}
	}
	params, results := s.tuple(osig.Params()), s.tuple(osig.Results())
	var recv *types.Var
	if osig.Recv() != nil {
		recv = s.varFor(osig.Recv())
	}
	sig := types.NewSignatureType(recv, nil, nil, params, results, osig.Variadic())
	obj := types.NewFunc(gf.fn.Pos(), gf.fn.Pkg(), gf.fn.Name(), sig)

	for e, tv := range snap.types {
		tv.Type = s.typ(tv.Type)
		info.Types[e] = tv
	}
	// unsafe.Sizeof(x) and Alignof(x) are constants in the instance, though they were not for
	// the type parameter.
	for e, tv := range snap.types {
		call, ok := e.(*ast.CallExpr)
		if !ok || tv.Value != nil || len(call.Args) != 1 {
			continue
		}
		sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if !ok {
			continue
		}
		b, ok := snap.uses[sel.Sel].(*types.Builtin)
		if !ok || b.Name() != "Sizeof" && b.Name() != "Alignof" {
			continue
		}
		t := info.Types[call.Args[0]].Type
		if t == nil {
			continue
		}
		n := check.Sizes().Sizeof(t)
		if b.Name() == "Alignof" {
			n = check.Sizes().Alignof(t)
		}
		tv = info.Types[e]
		tv.Value = constant.MakeInt64(n)
		info.Types[e] = tv
	}
	for id, o := range snap.defs {
		info.Defs[id] = s.object(o)
	}
	for id, o := range snap.uses {
		info.Uses[id] = s.object(o)
	}
	for n, o := range snap.implicits {
		info.Implicits[n] = s.object(o)
	}
	for id, in := range snap.instances {
		nargs := make([]types.Type, len(in.targs))
		for i, a := range in.targs {
			nargs[i] = s.typ(a)
		}
		ni := typeInst{targs: nargs, typ: in.typ}
		if orig := snap.uses[id]; orig != nil {
			if r, err := types.Instantiate(ctxt, orig.Type(), nargs, false); err == nil {
				ni.typ = r
			} else {
				return nil, err
			}
		}
		info.Instances[id] = ni
	}
	for x, sel := range snap.sels {
		recv := s.typ(sel.recv)
		if recv == sel.recv {
			info.Selections[x] = sel
			continue
		}
		o, index, indirect := types.LookupFieldOrMethod(recv, true, sel.obj.Pkg(), sel.obj.Name())
		if o == nil {
			if n, ok := recv.(*types.Named); ok && cxxNamed[n] {
				// A method of a C/C++ type argument: the C++ compiler finds it (methodCall).
				info.Selections[x] = &selection{kind: sel.kind, recv: recv, obj: sel.obj, index: []int{0}}
				continue
			}
			return nil, fmt.Errorf("no %s in %s", sel.obj.Name(), recv)
		}
		info.Selections[x] = &selection{kind: sel.kind, recv: recv, obj: o, index: index, indirect: indirect}
	}
	return obj, s.err
}

// An instRef is a use of a generic function with its type arguments: a call, or a func value.
type instRef struct {
	gf    *genericFunc
	fn    *types.Func
	targs []types.Type
	keys  []string         // the identity of each type argument, for a name
	sig   *types.Signature // the signature of the instance
	open  []*types.Named   // the type arguments that C++ deduces (opaque instances)
	pos   token.Pos
}

// instRef returns the instantiated generic function that the expression e names (f, pkg.F, or
// either with explicit type arguments, F[int]), or nil if e names something else.
//
// go/types records no instance when a type argument is a C/C++ type, or when an argument of the
// call is a C/C++ value that determines one (Phase 07 README §2.4: C/C++ type arguments): the
// instance is made from the syntax then (opaqueInstRef).
func (fg *funcGen) instRef(e ast.Expr) *instRef {
	g := fg.g
	var id *ast.Ident
	var idx []ast.Expr
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.IndexExpr:
			idx, e = []ast.Expr{x.Index}, x.X
			continue
		case *ast.IndexListExpr:
			idx, e = x.Indices, x.X
			continue
		case *ast.Ident:
			id = x
		case *ast.SelectorExpr:
			if g.info.Selections[x] != nil {
				return nil
			}
			id = x.Sel
		default:
			return nil
		}
		break
	}
	fn, _ := g.info.Uses[id].(*types.Func)
	if fn == nil || fn.Signature().TypeParams().Len() == 0 {
		return nil
	}
	owner := g.other(fn.Pkg())
	if owner == nil {
		return nil // an intrinsic of a package that is not lowered (weak.Make)
	}
	gf := owner.generics[fn.Origin()]
	if gf == nil {
		g.errorf(e.Pos(), "internal error: no generic function %s", fn.Name())
		return &instRef{fn: fn}
	}
	if gf.owner != g && gf.usesCxx() {
		// The body is lowered with the asking package's C/C++ state (its headers, names and
		// probes), not the owner's: not built yet.
		g.notYet(e.Pos(), fmt.Sprintf("%s.%s uses C/C++ names, so it can only be instantiated in its own package (Phase 12)", fn.Pkg().Name(), fn.Name()))
		return &instRef{fn: fn}
	}
	in, ok := g.info.Instances[id]
	if !ok {
		return fg.opaqueInstRef(gf, idx, nil, e.Pos())
	}
	for _, a := range in.targs {
		if !complete(a) || hasC(a) {
			return fg.opaqueInstRef(gf, idx, in.targs, e.Pos()) // go/types inferred an invalid type argument
		}
	}
	sig, _ := in.typ.(*types.Signature)
	if sig == nil {
		return nil
	}
	return &instRef{gf: gf, fn: fn, targs: in.targs, sig: sig, pos: e.Pos()}
}

// opaqueInstRef returns the instance of the generic function gf whose type arguments are the explicit
// ones in idx (the syntax of F[A, B]) and, for the rest, whatever C++ deduces from the arguments of
// the call: the instance is a C++ function template over those. An explicit type argument that is
// a C/C++ type (std.vector[int]) is written into the instance, as it is spelled at the call
// (typeExpr).
func (fg *funcGen) opaqueInstRef(gf *genericFunc, idx []ast.Expr, inferred []types.Type, pos token.Pos) *instRef {
	g := fg.g
	tparams := gf.fn.Signature().TypeParams()
	r := &instRef{gf: gf, fn: gf.fn, targs: make([]types.Type, tparams.Len()), keys: make([]string, tparams.Len()), pos: pos}
	for i := range r.targs {
		name := tparams.At(i).Obj().Name()
		if _, ok := callableParam(tparams.At(i)); ok {
			// A closure of a macro (gp_L_x, gp_B): the func type of its constraint, whatever the other type
			// arguments are. (It is a C++ template parameter of the instance, but a func in the body.)
			if ft := constraintFunc(tparams.At(i)); ft != nil {
				r.targs[i], r.keys[i] = ft, g.canonText(ft)
				continue
			}
		}
		if i < len(inferred) && complete(inferred[i]) && !hasC(inferred[i]) && !(i < len(idx) && !g.info.Types[idx[i]].IsType()) {
			r.targs[i], r.keys[i] = inferred[i], g.canonText(inferred[i])
			continue
		}
		if i < len(idx) {
			if tv := g.info.Types[idx[i]]; tv.IsType() && tv.Type != nil && complete(tv.Type) {
				r.targs[i], r.keys[i] = tv.Type, g.canonText(tv.Type)
				continue
			}
			named := g.newCxxNamed(gf, name, g.typeExpr(idx[i]), false)
			r.targs[i], r.keys[i] = named, "cxx:"+g.srcText(idx[i])
			continue
		}
		named := g.newCxxNamed(gf, name, typeName(name), true)
		r.targs[i], r.keys[i] = named, "open"
		r.open = append(r.open, named)
	}
	g.openCores(tparams, r)
	s := newSubster(tparams, r.targs, g.context())
	r.sig, _ = s.typ(gf.fn.Signature()).(*types.Signature)
	return r
}

// openCores gives the open type parameters of an opaque instance their core slice type: S ~[]E is
// the C++ type that C++ deduces, which Go code can index, slice, range over and take the len of, and
// E is that slice's element type, which C++ cannot deduce from the arguments, so it gets
// typename gp::detail::slice_elem_of<S>::type as a default.
func (g *gen) openCores(tparams *types.TypeParamList, r *instRef) {
	for i := range tparams.Len() {
		named, ok := r.targs[i].(*types.Named)
		if !ok || !g.cxxNamed[named].open {
			continue
		}
		sl := sliceCore(tparams.At(i).Constraint())
		if sl == nil {
			continue
		}
		info := g.cxxNamed[named]
		elem := sl.Elem()
		info.elemOf = -1
		if tp, ok := types.Unalias(elem).(*types.TypeParam); ok {
			j := tp.Index()
			if j < tparams.Len() && tparams.At(j) == tp {
				elem = r.targs[j]
				if en, ok := elem.(*types.Named); ok && g.cxxNamed[en].open && j > i {
					ei := g.cxxNamed[en]
					ei.from = i + 1 // 1-based: the template parameter of S
					g.cxxNamed[en] = ei
				}
			}
		}
		info.core = types.NewSlice(elem)
		g.cxxNamed[named] = info
	}
}

// sliceCore returns the slice type that every type of the type set of the constraint c has as its
// underlying type (S ~[]E: []E), or nil.
func sliceCore(c types.Type) *types.Slice {
	it, ok := c.Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	var found *types.Slice
	for i := range it.NumEmbeddeds() {
		var sl *types.Slice
		switch e := it.EmbeddedType(i).(type) {
		case *types.Union:
			if e.Len() != 1 {
				return nil
			}
			sl, _ = e.Term(0).Type().Underlying().(*types.Slice)
		default:
			sl = sliceCore(e)
			if sl == nil {
				if _, isIface := e.Underlying().(*types.Interface); isIface {
					continue
				}
				sl, _ = e.Underlying().(*types.Slice)
			}
		}
		if sl == nil {
			continue
		}
		if found != nil {
			return nil
		}
		found = sl
	}
	return found
}

// cxxCore returns the slice type that the C++-deduced type parameter t stands for (S ~[]E), or nil
// when t is not one.
func (g *gen) cxxCore(t types.Type) *types.Slice {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil
	}
	return g.cxxNamed[n].core
}

// A cxxInfo is the C++ type that stands for a type argument that is a C/C++ type: written out, or a
// template parameter of the instance that C++ deduces (open).
type cxxInfo struct {
	typ  cppir.Type
	open bool
	// core is the slice type []E of a type parameter whose constraint has that core type (S ~[]E),
	// when it is open: index, slice expressions, range, len and cap treat the stand-in as that slice
	// (the C++ type is whatever slice C++ deduced). elemOf is the index of E among the type
	// parameters when E is one (E is then given S's element type as its default).
	core   *types.Slice
	elemOf int
	from   int
}

// newCxxNamed returns a type that stands for a C/C++ type argument in the instance of gf: a named
// type with an empty struct as its underlying type, which cxxType writes as typ. Values of it can
// be copied, passed, compared and returned, which is what a constraint of any or comparable allows.
func (g *gen) newCxxNamed(gf *genericFunc, name string, typ cppir.Type, open bool) *types.Named {
	g.cxxCount++
	tn := types.NewTypeName(token.NoPos, gf.fn.Pkg(), fmt.Sprintf("gp_cxx_%d_%s", g.cxxCount, name), nil)
	named := types.NewNamed(tn, types.NewStruct(nil, nil), nil)
	if g.cxxNamed == nil {
		g.cxxNamed = make(map[*types.Named]cxxInfo)
	}
	g.cxxNamed[named] = cxxInfo{typ: typ, open: open}
	return named
}

// context returns the context that go/types instantiates types with.
func (g *gen) context() *types.Context {
	if g.ctxt == nil {
		g.ctxt = types.NewContext()
	}
	return g.ctxt
}

// instanceName returns the C++ name of the instance, qualified with the namespace of the asking package,
// and asks for the instance to be written. A function template (an instance with open type
// arguments) is also given the type arguments that are in scope here, the template parameters of the
// instance being lowered: Inner<T>(x).
func (g *gen) instanceName(r *instRef) cppir.Expr {
	if r.gf == nil {
		return placeholder // (reported)
	}
	var key strings.Builder
	key.WriteString(importPath(r.gf.fn.Pkg()) + "." + r.gf.fn.Name())
	for i, a := range r.targs {
		if r.keys != nil {
			key.WriteString("\x00" + r.keys[i])
		} else {
			key.WriteString("\x00" + g.canonText(a))
		}
	}
	// The template parameters: the open type arguments, also those inside the type arguments
	// (a []T of an instance that is a template over T).
	var open []*types.Named
	seen := map[*types.Named]bool{}
	var walk func(t types.Type)
	walk = func(t types.Type) {
		if n, ok := types.Unalias(t).(*types.Named); ok {
			if c, ok := g.cxxNamed[n]; ok && c.open && !seen[n] {
				seen[n] = true
				open = append(open, n)
			}
		}
		g.eachPart(t, walk)
	}
	for _, a := range r.targs {
		walk(a)
	}
	k := instKey{g.unit, key.String()}
	in, ok := g.insts[k]
	if !ok {
		text := g.pkg.Load.ImportPath + "\x00" + filepath.Base(k.unit) + "\x00" + k.key
		sum := sha256.Sum256([]byte(text))
		// Eight hex digits of the hash; a clash with another instance of the package (another text
		// with the same name) takes more of it.
		n := 4
		name := "gp_inst_" + hex.EncodeToString(sum[:n]) + "_" + g.escape(r.gf.fn.Name())
		for g.instNames[name] != "" && g.instNames[name] != text && n < len(sum) {
			n++
			name = "gp_inst_" + hex.EncodeToString(sum[:n]) + "_" + g.escape(r.gf.fn.Name())
		}
		if g.instNames == nil {
			g.instNames = make(map[string]string)
		}
		g.instNames[name] = text
		in = &instance{gf: r.gf, targs: r.targs, unit: g.unit, open: open, name: name}
		g.noteInstanceUse(r, name) // why.go
		if g.insts == nil {
			g.insts = make(map[instKey]*instance)
		}
		g.insts[k] = in
		g.instQueue = append(g.instQueue, in)
	}
	fun := cxxName(g.ns + "::" + in.name)
	if len(open) > 0 {
		// Explicit template arguments where they are in scope: the open placeholders of the
		// instance being lowered.
		var targs []cppir.Node
		inScope := true
		for _, n := range open {
			if !g.curOpen[n] {
				inScope = false
				break
			}
			targs = append(targs, g.cxxNamed[n].typ)
		}
		if inScope {
			fun.Args = targs
		}
	}
	return fun
}

// eachPart calls f for the types that t is made of, one level down.
func (g *gen) eachPart(t types.Type, f func(types.Type)) {
	switch t := types.Unalias(t).(type) {
	case *types.Pointer:
		f(t.Elem())
	case *types.Slice:
		f(t.Elem())
	case *types.Array:
		f(t.Elem())
	case *types.Map:
		f(t.Key())
		f(t.Elem())
	case *types.Chan:
		f(t.Elem())
	case *types.Signature:
		for v := range t.Params().Variables() {
			f(v.Type())
		}
		for v := range t.Results().Variables() {
			f(v.Type())
		}
	case *types.Struct:
		for i := range t.NumFields() {
			f(t.Field(i).Type())
		}
	case *types.Named:
		for a := range t.TypeArgs().Types() {
			f(a)
		}
	}
}

// drainInstances lowers the instances that the code lowered so far asked for, and the instances
// that those ask for.
func (g *gen) drainInstances() {
	for len(g.instQueue) > 0 && g.err == nil {
		in := g.instQueue[0]
		g.instQueue = g.instQueue[1:]
		g.lowerInstance(in)
	}
}

// lowerInstance lowers the instance in into the output of its unit.
func (g *gen) lowerInstance(in *instance) {
	gf := in.gf
	owner := gf.owner
	cxx := make(map[*types.Named]bool, len(g.cxxNamed))
	for n := range g.cxxNamed {
		cxx[n] = true
	}
	obj, err := gf.apply(in.targs, g.context(), cxx)
	if err != nil {
		g.errorf(gf.decl.Pos(), "internal error: instantiating %s: %v", gf.fn.Name(), err)
		return
	}
	// The instance is lowered with the package's tables (the conversions Go makes implicitly,
	// the numbers of the func literals) made for it alone.
	saved := struct {
		info       *tinfo
		implicit   map[ast.Expr]types.Type
		uintCounts map[ast.Expr]bool
		unit       string
		open       map[*types.Named]bool
		inst       string
	}{g.info, g.implicit, g.uintCounts, g.unit, g.curOpen, g.curInst}
	g.info, g.implicit, g.uintCounts, g.unit = owner.info, nil, make(map[ast.Expr]bool), in.unit
	g.curOpen = make(map[*types.Named]bool)
	g.curInst = in.name
	for _, n := range in.open {
		g.curOpen[n] = true
	}
	defer func() {
		g.info, g.implicit, g.uintCounts, g.unit, g.curOpen, g.curInst = saved.info, saved.implicit, saved.uintCounts, saved.unit, saved.open, saved.inst
	}()

	g.noteInstanceBody(in) // why.go
	f := &funcInfo{decl: gf.decl, obj: obj, name: in.name}
	f.prepare(g)
	// The parameters that stand for a closure of a macro (callableParam) are C++ template
	// parameters: the call passes the lambda itself.
	var callable []string
	for i := range f.params {
		if gf.callable != nil && gf.callable[i] {
			name, _ := callableParam(gf.fn.Signature().Params().At(i).Type())
			f.params[i].Type = typeName(name)
			callable = append(callable, name)
		}
	}
	proto := &cppir.Func{Result: f.result, Name: f.name, Params: f.params}
	fn := g.funcBuild(f)
	if g.err != nil {
		return
	}
	if strings.HasPrefix(in.unit, "\x00inst:") {
		// In the header of an instance of a generic type: inline.
		proto.Specs, fn.Specs = []string{"inline"}, []string{"inline"}
	}
	if len(in.open) > 0 || len(callable) > 0 {
		// A function template over the type arguments that C++ deduces (opaque instances) and over
		// the closures of a macro.
		t, pt := &cppir.Template{}, &cppir.Template{} // pt, of the prototype, has the defaults
		for _, n := range in.open {
			name := cppir.TypeString(g.cxxNamed[n].typ)
			t.Params = append(t.Params, cppir.Param{Type: typeName("typename"), Name: name})
			if f := g.cxxNamed[n].from; f > 0 && f <= len(in.open) {
				// E of S ~[]E: C++ cannot deduce it from the arguments (a return type, a local).
				name += " = typename gp::detail::slice_elem_of<" + cppir.TypeString(g.cxxNamed[in.open[f-1]].typ) + ">::type"
			}
			pt.Params = append(pt.Params, cppir.Param{Type: typeName("typename"), Name: name})
		}
		for _, name := range callable {
			p := cppir.Param{Type: typeName("typename"), Name: name}
			t.Params, pt.Params = append(t.Params, p), append(pt.Params, p)
		}
		proto.Template, fn.Template = pt, t
	}
	u := g.out.instUnit(in.unit)
	u.protos = append(u.protos, proto)
	u.defs = append(u.defs, &cppir.Comment{Text: g.instanceText(in)}, fn, &cppir.Blank{})
}

// instanceText returns Map[int,string], the instance in the Go notation, for the comment above
// its C++.
func (g *gen) instanceText(in *instance) string {
	args := make([]string, len(in.targs))
	for i, a := range in.targs {
		if n, ok := a.(*types.Named); ok && g.cxxNamed[n].typ != nil {
			args[i] = cppir.TypeString(g.cxxNamed[n].typ)
			if g.cxxNamed[n].open {
				args[i] = "deduced " + args[i]
			}
			continue
		}
		args[i] = types.TypeString(a, types.RelativeTo(g.pkg.Types))
	}
	return in.gf.fn.Name() + "[" + strings.Join(args, ",") + "]"
}

// An instUnit is the instances of one translation unit.
type instUnit struct {
	protos []cppir.Decl
	defs   []cppir.Decl
}

// instUnit returns the instances of the unit (the .gp file, or initUnit for the package's
// initialization).
func (o *outputs) instUnit(unit string) *instUnit {
	if o.insts == nil {
		o.insts = make(map[string]*instUnit)
	}
	u := o.insts[unit]
	if u == nil {
		u = &instUnit{}
		o.insts[unit] = u
	}
	return u
}

// initUnit is the unit of the package's initialization (pkg_init.cpp).
const initUnit = "\x00init"

// decls returns the declarations of the instances of the unit, to put before the functions: the
// prototypes, then the definitions.
func (o *outputs) instDecls(unit string) []cppir.Decl {
	u := o.insts[unit]
	if u == nil {
		return nil
	}
	return append(append([]cppir.Decl{}, u.protos...), append([]cppir.Decl{&cppir.Blank{}}, u.defs...)...)
}

// allInstDecls returns the instances of every unit, for the single translation unit.
func (o *outputs) allInstDecls(order []string) []cppir.Decl {
	var out []cppir.Decl
	for _, u := range order {
		out = append(out, o.instDecls(u)...)
	}
	return out
}

// fileOf returns the name of the .gp file that declares the node at pos, as the units are named.
func (g *gen) fileOf(pos token.Pos) string {
	file := g.fset.Position(pos).Filename
	if tf := g.fset.File(pos); tf != nil {
		file = tf.Name()
	}
	return file
}

// instUnitOrder returns the units that have instances, the files in the order of their first
// function and the initialization last.
func (g *gen) instUnitOrder() []string {
	var order []string
	for _, f := range g.out.files {
		if g.out.insts[f] != nil {
			order = append(order, f)
		}
	}
	if g.out.insts[initUnit] != nil {
		order = append(order, initUnit)
	}
	return order
}

// instFuncValue returns the instance r as a func value, as funcRef does for a function of the
// package.
func (fg *funcGen) instFuncValue(r *instRef) cppir.Expr {
	if r.gf == nil {
		return placeholder
	}
	name := fg.g.instanceName(r)
	if n, ok := name.(*cppir.Name); ok && len(r.open) > 0 && len(n.Args) == 0 {
		fg.g.userError(r.pos, "a generic function with a type argument that C++ deduces cannot be a func value: write the type arguments, F[std.string]")
		return placeholder
	}
	addr := &cppir.Unary{Op: "&", X: name}
	if fg.g.awareFunc(r.fn) {
		return templateCall("gp::func_of_aware", []cppir.Node{addr}) // recover.go
	}
	return templateCall("gp::func_of", []cppir.Node{addr})
}

// hasTypeParam reports whether the type t mentions a type parameter: the parameter of a generic
// function that is called with a C/C++ value, whose type argument the C++ compiler deduces.
func hasTypeParam(t types.Type) bool {
	found := false
	var walk func(t types.Type)
	seen := map[types.Type]bool{}
	walk = func(t types.Type) {
		if found || seen[t] {
			return
		}
		seen[t] = true
		if _, ok := types.Unalias(t).(*types.TypeParam); ok {
			found = true
			return
		}
		(*gen)(nil).eachPart(t, walk)
	}
	walk(t)
	return found
}

// foreignVal reports whether a value of type t is a C/C++ value as far as descriptors go: its type
// is not complete, or it is made of a C/C++ type argument of a generic function (cxxNamed).
func (g *gen) foreignVal(t types.Type) bool {
	return !complete(t) || g.hasCxxNamed(t)
}

// hasCxxNamed reports whether t is made of a C/C++ type argument of a generic function.
func (g *gen) hasCxxNamed(t types.Type) bool {
	if len(g.cxxNamed) == 0 {
		return false
	}
	found := false
	seen := map[types.Type]bool{}
	var walk func(t types.Type)
	walk = func(t types.Type) {
		if found || seen[t] {
			return
		}
		seen[t] = true
		if n, ok := types.Unalias(t).(*types.Named); ok {
			if _, ok := g.cxxNamed[n]; ok {
				found = true
				return
			}
		}
		g.eachPart(t, walk)
	}
	walk(t)
	return found
}

// isCxxNamed reports whether t is a C/C++ type argument of a generic function.
func (g *gen) isCxxNamed(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	_, ok = g.cxxNamed[n]
	return ok
}

// genericRef reports whether e names a generic function of a package of the program: F or pkg.F,
// the part of F[int] before the type arguments.
func (g *gen) genericRef(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		fn, _ := g.info.Uses[x].(*types.Func)
		return fn != nil && g.isGeneric(fn)
	case *ast.SelectorExpr:
		fn := g.qualifiedFunc(x)
		return fn != nil && g.isGeneric(fn)
	}
	return false
}

// cxxConst returns the C++ literal of the constant v of a C/C++ type argument (x + 1 with x of the
// type parameter T that is C.double): a literal of the kind of v, which C++ converts as it does for C.
func (g *gen) cxxConst(v constant.Value, pos token.Pos) cppir.Expr {
	switch v.Kind() {
	case constant.Bool:
		return g.constant(v, types.Typ[types.Bool], pos, false)
	case constant.String:
		// A C string literal, which std::string and the other C++ string types take (a
		// gp::string would not: a + "!" with a std::string a).
		return lit(cppir.CString(constant.StringVal(v)))
	case constant.Float:
		return g.constant(v, types.Typ[types.Float64], pos, false)
	}
	return g.constant(v, types.Typ[types.UntypedInt], pos, false)
}
