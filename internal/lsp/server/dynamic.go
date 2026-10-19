package server

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// Dynamic targets of the method family (ADR-052, round 2).
//
// A use that needs an interface of a value of a type ties their methods (family.go
// ties). A type assertion, a type switch and errors.As take a value out as an
// interface with no value of a known type to tie it to: the dynamic type is
// any type of the module that implements the interface, and one that stops
// implementing it after a rename changes what the program does with no
// compile-time error (the assertion fails at run time; gopls renames the same
// way). So the interface's method, of the name the family has, and the method of
// every type that implements the interface, are one method, too:
//
//   - a type of the family that implements the interface brings the interface's
//     method into the family;
//   - the interface's method in the family brings in the method of every type of
//     the module that implements it.
//
// "Implements" cannot be asked of go/types here: the interface is in one
// package's check (main declares Temporary), the type in another's, which
// main's check does not have at all (apierr.retryable is unexported and
// reached only through error). So both are turned into a shape, the methods
// as name and signature text with the packages' import paths, which is
// the same text in every check, and compared as text. A type is looked at
// where it is declared: when an interface of the family is a dynamic target,
// every package of the module is searched (once).

// A dynTarget is an interface that a type assertion, a type switch case or an
// errors.As target takes a value out as.
type dynTarget struct {
	methods []dynMethod
}

// A dynMethod is one method of a dynTarget.
type dynMethod struct {
	name string
	pkg  *types.Package // the package that declares the method (for an unexported name)
	sig  string         // the signature text, without the receiver
	obj  types.Object   // the interface's method (its origin)
	key  objKey
}

// sigText is the signature of fn as text that is the same in every check: the
// types with the import paths of their packages.
func sigText(fn *types.Func) string {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return ""
	}
	plain := types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic())
	return types.TypeString(plain, func(p *types.Package) string { return p.Path() })
}

// methodID names a method independently of the check: an unexported name also
// by its package.
func methodID(name string, pkg *types.Package) string {
	if pkg != nil && name != "" && !token.IsExported(name) {
		return pkg.Path() + "." + name
	}
	return name
}

// assertions returns the ties of the view that are dynamic targets, once for
// each checked package.
func (v *view) assertions() []tie {
	if v.info == nil {
		return nil
	}
	pick := func() any {
		var out []tie
		for _, t := range v.ties() {
			if t.asserted {
				out = append(out, t)
			}
		}
		return out
	}
	if v.pkg == nil || v.test {
		return pick().([]tie)
	}
	return v.pkg.Memo(assertionsKey{}, pick).([]tie)
}

type assertionsKey struct{}

// relatedByShapes adds to keys the methods that the dynamic targets (the
// interfaces of type assertions, type switches and errors.As) of the views
// relate to the family, and reports whether it added one. It only does so
// for a method symbol.
func (r *searcher) relatedByShapes(views []*view, keys *keySet) bool {
	sym := r.sym
	if len(sym.objs) != 1 {
		return false
	}
	fn, ok := sym.objs[0].(*types.Func)
	if !ok || recvBase(fn) == nil {
		return false
	}
	name := sym.name
	// The dynamic targets that have a method of the family's name.
	var targets []*dynTarget
	seen := make(map[string]bool)
	for _, v := range views {
		for _, t := range v.assertions() {
			if !t.names[name] {
				continue
			}
			dt := newDynTarget(t)
			if dt == nil {
				continue
			}
			id := dt.id()
			if !seen[id] {
				seen[id] = true
				targets = append(targets, dt)
			}
		}
	}
	if len(targets) == 0 {
		return false
	}
	added := false
	add := func(obj types.Object) {
		if r.inWorkspace(obj) && keys.add(keyOf(obj)) {
			added = true
		}
	}
	for _, dt := range targets {
		var named *dynMethod // the target's method of the family's name
		for i := range dt.methods {
			if dt.methods[i].name == name {
				named = &dt.methods[i]
			}
		}
		if named == nil || !r.inWorkspace(named.obj) {
			continue // (an interface outside the workspace is found and refused by the ties)
		}
		inFamily := keys.has[named.key]
		// The workspace's types that implement the target, by the shape, with their
		// method of the family's name.
		var impl []types.Object
		for _, v := range views {
			for _, x := range r.namedTypes(v) {
				if types.IsInterface(x) || !dt.implementedBy(x) {
					continue
				}
				if m := lookupMethod(x, named.pkg, name); m != nil && r.inWorkspace(origin(m)) {
					impl = append(impl, origin(m))
				}
			}
		}
		if !inFamily {
			for _, m := range impl {
				if keys.has[keyOf(m)] {
					inFamily = true
					break
				}
			}
			if !inFamily {
				continue
			}
			add(named.obj)
		}
		// A family that is a dynamic target needs every type of the module: the
		// ones that are not searched yet are (once).
		r.wantAll = true
		for _, m := range impl {
			add(m)
		}
	}
	return added
}

// namedTypes returns the named types that the view's package declares (its
// test files' too, in a test view), that are not generic. They are the
// types that implement an interface where they are declared.
func (r *searcher) namedTypes(v *view) []*types.Named {
	if v.types == nil {
		return nil
	}
	scope := v.types.Scope()
	var out []*types.Named
	for _, n := range scope.Names() {
		tn, ok := scope.Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		if named, ok := tn.Type().(*types.Named); ok && named.TypeParams().Len() == 0 {
			out = append(out, named)
		}
	}
	return out
}

// newDynTarget returns the shape of the interface of the tie t (asserted).
func newDynTarget(t tie) *dynTarget {
	it, ok := t.dst.Underlying().(*types.Interface)
	if !ok || it.NumMethods() == 0 {
		return nil
	}
	dt := &dynTarget{}
	for m := range it.Methods() {
		dt.methods = append(dt.methods, dynMethod{
			name: m.Name(), pkg: m.Pkg(), sig: sigText(m), obj: origin(m), key: keyOf(m),
		})
	}
	return dt
}

// id is a text that is the same for the same interface seen from different
// checks.
func (dt *dynTarget) id() string {
	var parts []string
	for _, m := range dt.methods {
		parts = append(parts, fmt.Sprintf("%s|%s|%s|%s|%p", methodID(m.name, m.pkg), m.sig, m.key.pkg, m.key.path, m.key.local))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// implementedBy reports whether the type x (or a pointer to it) has all the
// methods of the target, by name and signature text.
func (dt *dynTarget) implementedBy(x *types.Named) bool {
	for _, m := range dt.methods {
		fn := lookupMethod(x, m.pkg, m.name)
		if fn == nil || sigText(fn) != m.sig {
			return false
		}
	}
	return true
}
