package lower

import (
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strconv"
)

// Types declared in generic functions (Phase 07 step 7.3).
//
// Go gives every type declared in a generic function the function's type parameters as implicit
// parameters: in F[T any, U any], the type `type local struct{ x T }` is a different type in
// F[int, string] and in F[bool, bool], and %T prints main.local[int,string] and main.local[bool,bool],
// also for a local type that does not mention T or U. go/types has one Named for it.
//
// The lowering makes it a generic type of its own, with a synthetic origin whose type parameters
// are copies of the function's (localOrigin), and replaces every mention of the local type in an
// instance's body by the instance of that origin with the instance's type arguments
// (subster.local): from there it is an instance of a generic type (insttypes.go), shared by the
// program, named like Go's %T, with no methods (a local type has none).

// localInfo is the synthetic origin of a type declared in a generic function.
type localInfo struct {
	origin *types.Named
	tag    string // the unique tag that canonText appends (two functions may declare local)
	// implicit is the number of leading type parameters of origin that are the function's: a
	// generic local type has its own after them, and Go writes them after a semicolon
	// (main.Box[int,string;bool]).
	implicit int
}

// localTypeIn reports whether t is a type declared inside the function whose scope is fn.
func localTypeIn(t *types.Named, fn *types.Scope) bool {
	if fn == nil || t.TypeArgs().Len() > 0 {
		return false
	}
	obj := t.Obj()
	if obj.Pkg() == nil {
		return false
	}
	for sc := obj.Parent(); sc != nil && sc != obj.Pkg().Scope(); sc = sc.Parent() {
		if sc == fn {
			return true
		}
	}
	return false
}

// localInstance returns the instance, for the type arguments targs of the function fn whose type
// parameters are tparams, of the local type t (its origin, for a generic one), where own are the
// type arguments of the type itself (none unless t is generic).
func (g *gen) localInstance(t *types.Named, fn *types.Func, tparams *types.TypeParamList, targs, own []types.Type, ctxt *types.Context) types.Type {
	li := g.localOrigin(t, fn, tparams, ctxt)
	all := append(slices.Clone(targs), own...)
	r, err := types.Instantiate(ctxt, li.origin, all, false)
	if err != nil {
		g.errorf(t.Obj().Pos(), "internal error: instantiating the local type %s: %v", t.Obj().Name(), err)
		return t
	}
	return r
}

// localOrigin returns the synthetic generic type of the local type t, made the first time.
func (g *gen) localOrigin(t *types.Named, fn *types.Func, tparams *types.TypeParamList, ctxt *types.Context) *localInfo {
	if li := g.prog.localOrigins[t]; li != nil {
		return li
	}
	if g.prog.localOrigins == nil {
		g.prog.localOrigins = make(map[*types.Named]*localInfo)
		g.prog.localTagOf = make(map[*types.TypeName]string)
		g.prog.localImplicit = make(map[*types.TypeName]int)
	}
	tn := types.NewTypeName(t.Obj().Pos(), t.Obj().Pkg(), t.Obj().Name(), nil)
	o := types.NewNamed(tn, nil, nil)
	// The tag names the declaration, not the order in which the lowering met it (the substitution
	// walks maps): the function, the type and the place in the function.
	li := &localInfo{origin: o, implicit: tparams.Len(), tag: "Lg" + funcTagName(fn) + "_" + t.Obj().Name() + "_" + g.placeInFunc(t.Obj().Pos(), fn.Pos())}
	g.prog.localOrigins[t] = li
	g.prog.localTagOf[tn] = li.tag
	g.prog.localImplicit[tn] = li.implicit
	if owner := g.other(t.Obj().Pkg()); owner != nil {
		if owner.genericTypes == nil {
			owner.genericTypes = make(map[*types.TypeName]*genericType)
		}
		owner.genericTypes[tn] = &genericType{obj: tn}
	}
	// Fresh type parameters (go/types binds a type parameter to one type only), with the
	// constraints replaced the same way: the function's, then the type's own.
	var old []*types.TypeParam
	for i := range tparams.Len() {
		old = append(old, tparams.At(i))
	}
	for i := range t.TypeParams().Len() {
		old = append(old, t.TypeParams().At(i))
	}
	fresh := make([]*types.TypeParam, len(old))
	freshT := make([]types.Type, len(old))
	for i, p := range old {
		fresh[i] = types.NewTypeParam(types.NewTypeName(p.Obj().Pos(), p.Obj().Pkg(), p.Obj().Name(), nil), types.Typ[types.Invalid])
		freshT[i] = fresh[i]
	}
	s := newSubster(tparams, freshT[:tparams.Len()], ctxt)
	for i := tparams.Len(); i < len(old); i++ {
		s.env[old[i]] = freshT[i]
	}
	s.local = func(l *types.Named, own []types.Type) types.Type {
		if !localTypeIn(l, fn.Scope()) || len(own) != l.TypeParams().Len() {
			return nil
		}
		return g.localInstance(l, fn, tparams, freshT[:tparams.Len()], own, ctxt)
	}
	for i := range fresh {
		fresh[i].SetConstraint(s.typ(old[i].Constraint()))
	}
	o.SetTypeParams(fresh)
	o.SetUnderlying(s.typ(t.Underlying()))
	// The fields of the origin are new variables: their type expressions are those of the declared
	// type's fields (varOrigin), written with the function's type parameters and the type's own
	// (localParams), which typedecl.go: instTypeExpr replaces by an instance's type arguments.
	g.prog.noteVarOrigins(s)
	if g.prog.localParams == nil {
		g.prog.localParams = make(map[*types.TypeName][]*types.TypeParam)
	}
	g.prog.localParams[tn] = old
	return li
}

// funcTagName returns the name of fn for a tag: a method's includes its receiver type (the methods
// of two generic types may have the same name and declare the same local type at the same place).
func funcTagName(fn *types.Func) string {
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		t := sig.Recv().Type()
		if p, ok := types.Unalias(t).(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := types.Unalias(t).(*types.Named); ok {
			return n.Obj().Name() + "_" + fn.Name()
		}
	}
	return fn.Name()
}

// placeInFunc returns the place pos of a declaration in the function that starts at fn, for a tag:
// the offset from the function's start when both are in one file (as the token.Pos difference was),
// else the file's name and the offset in it (a method that #Derive writes in another file than the
// type: the token.Pos difference of two files changes with the size of every file between them).
func (g *gen) placeInFunc(pos, fn token.Pos) string {
	p, q := g.fset.Position(pos), g.fset.Position(fn)
	if p.Filename == q.Filename {
		return strconv.Itoa(p.Offset - q.Offset)
	}
	name := []byte(filepath.Base(p.Filename))
	for i, c := range name {
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9') {
			name[i] = '_'
		}
	}
	return string(name) + "_" + strconv.Itoa(p.Offset)
}
