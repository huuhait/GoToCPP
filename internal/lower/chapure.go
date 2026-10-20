package lower

import (
	"go/types"
)

// Class hierarchy analysis for the purity of interface calls (step 15.2, ADR-114).
//
// borrow.go calls a function pure when it cannot drop an owner, and a call through an interface
// is unknown code there. The escape analysis asks a narrower question about a region of code: can
// it overwrite a slot of a slice that holds an interface value, a pointer or a string? That is
// answered for an interface call by looking at every method that the call can reach: the
// methods of all named types of the program (and of every package it imports, lowered or not)
// that have the interface method's name and signature. The call is pure when each of them is
// pure by the summaries of borrow.go; a method of a package that is not lowered (a native), of a
// generic type, or an interface method that nothing in the program implements is not.
//
// This is a whole-program fact, valid for the packages of the program as they are lowered. A
// value of a type that the program does not know (reflection, a plugin) cannot be in an
// interface of a Go program.

// A chaIndex lists the methods of the named types of a program by the key of the method.
type chaIndex struct {
	methods map[string][]*types.Func // by methodKey: the methods that implement an interface method
	generic map[string]bool          // a generic type has a method of that name: unknown signatures
	pure    map[chaQuery]bool        // the answers
}

type chaQuery struct {
	key    string
	strict bool
}

// methodKey names the method fn without its receiver: the id of the name (a package qualifies
// an unexported one) and the signature.
func methodKey(fn *types.Func) string {
	sig := fn.Signature()
	plain := types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic())
	return types.Id(fn.Pkg(), fn.Name()) + " " + types.TypeString(plain, nil)
}

// nameKey is the key of just the name, for generic types.
func nameKey(fn *types.Func) string {
	return types.Id(fn.Pkg(), fn.Name())
}

// chaOf returns the index of the program of g (built once).
func (g *gen) chaOf() *chaIndex {
	if g.prog != nil {
		if g.prog.cha == nil {
			g.prog.cha = buildCHA(g.programTypes())
		}
		return g.prog.cha
	}
	if g.cha == nil {
		g.cha = buildCHA([]*types.Package{g.pkg.Types})
	}
	return g.cha
}

// programTypes returns the packages of the program and everything they import, transitively.
func (g *gen) programTypes() []*types.Package {
	var out []*types.Package
	seen := make(map[*types.Package]bool)
	var visit func(p *types.Package)
	visit = func(p *types.Package) {
		if p == nil || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
		for _, q := range p.Imports() {
			visit(q)
		}
	}
	for _, o := range g.prog.gens {
		visit(o.pkg.Types)
	}
	return out
}

func buildCHA(pkgs []*types.Package) *chaIndex {
	idx := &chaIndex{
		methods: make(map[string][]*types.Func),
		generic: make(map[string]bool),
		pure:    make(map[chaQuery]bool),
	}
	seen := make(map[*types.Func]bool)
	for _, p := range pkgs {
		scope := p.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			named, ok := types.Unalias(tn.Type()).(*types.Named)
			if !ok || types.IsInterface(named) {
				continue
			}
			if named.TypeParams().Len() > 0 {
				for i := range named.NumMethods() {
					idx.generic[nameKey(named.Method(i))] = true
				}
				continue
			}
			ms := types.NewMethodSet(types.NewPointer(named))
			for i := range ms.Len() {
				fn, ok := ms.At(i).Obj().(*types.Func)
				if !ok || isIfaceMethod(fn) {
					continue
				}
				fn = fn.Origin()
				if seen[fn] {
					continue
				}
				seen[fn] = true
				idx.methods[methodKey(fn)] = append(idx.methods[methodKey(fn)], fn)
			}
		}
	}
	return idx
}

// ifaceMethodPure reports whether a call of the interface method m can only reach pure methods
// (strict: pure for strings too, borrowSum.pureV).
func (g *gen) ifaceMethodPure(m *types.Func, strict bool) bool {
	idx := g.chaOf()
	key := methodKey(m)
	q := chaQuery{key: key, strict: strict}
	if v, ok := idx.pure[q]; ok {
		return v
	}
	ok := !idx.generic[nameKey(m)] && len(idx.methods[key]) > 0
	for _, fn := range idx.methods[key] {
		if !ok {
			break
		}
		s := g.borrowOf(fn)
		switch {
		case s == nil:
			ok = false
		case strict:
			ok = s.pureV
		default:
			ok = s.pure
		}
	}
	idx.pure[q] = ok
	return ok
}
