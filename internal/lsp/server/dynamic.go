package server

import (
	"fmt"
	"go/token"
	"go/types"
	"slices"
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
	site    token.Position // the assertion (for messages)
	text    string         // the interface, as text (for messages)
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

// A dynAdd is a method that the dynamic rule added to the family of a rename: a
// method of a type that a type assertion, a type switch or errors.As can reach.
type dynAdd struct {
	obj   types.Object
	site  token.Position // the assertion that makes it part of the family
	iface string         // the interface it takes the value out as, as text
}

// relatedByShapes adds to keys, for a rename of a method, the methods that
// the dynamic targets (the interfaces of type assertions, type switches and
// errors.As) of the module relate to the family, and reports whether it added
// one.
//
// Only types that can reach the assertion count: a value of a type that is
// converted to no interface anywhere in the module (not even to any) is never
// the dynamic value of an interface, so no assertion takes it out, and
// renaming its method with the interface's would change code that does not
// depend on the assertion (and, for a name like Close that every type has, the
// whole module). A converted type is a conservative stand-in for "reaches":
// the value may flow anywhere from the conversion. References and highlights
// do not follow them at all (they answer where the method is used).
//
// The packages are searched for assertions by discoverDynamic, which needs no
// package of the family; the types that implement the interface are in every
// package, so when the family could be involved (a type of the family
// implements a dynamic target, or the target's method is in the family) the
// rest of the module is searched (once, wantAll) before anything is added.
func (r *searcher) relatedByShapes(views []*view, keys *keySet) bool {
	sym := r.sym
	if !sym.rename || r.s.wideFamily || len(sym.objs) != 1 {
		return false
	}
	fn, ok := sym.objs[0].(*types.Func)
	if !ok || recvBase(fn) == nil {
		return false
	}
	name := sym.name
	// The dynamic targets that have a method of the family's name.
	targets := slices.Clone(r.dyn)
	seen := make(map[string]bool, len(targets))
	for _, dt := range targets {
		seen[dt.id()] = true
	}
	for _, v := range views {
		for _, t := range v.assertions() {
			if !t.names[name] {
				continue
			}
			dt := newDynTarget(t, v)
			if dt == nil {
				continue
			}
			if id := dt.id(); !seen[id] {
				seen[id] = true
				targets = append(targets, dt)
			}
		}
	}
	if len(targets) == 0 {
		return false
	}
	added := false
	add := func(obj types.Object, dt *dynTarget, counted bool) {
		if !r.inWorkspace(obj) || !keys.add(keyOf(obj)) {
			return
		}
		added = true
		if counted {
			r.dynAdded = append(r.dynAdded, dynAdd{obj: obj, site: dt.site, iface: dt.text})
		}
	}
	// A value of a type can be the dynamic value of an interface only if the
	// type is converted to an interface (or flows to a place that is one:
	// scanTies) in its own package or in a package that imports it, directly or
	// not (a package that does not import it has no expression of its type).
	// So the types that can reach the assertions are the converted ones, the
	// types of the family are decided by the views of their importers, which the
	// search has, and the converted types of the rest of the module are known
	// when every package is searched (allLoaded).
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
		// The converted types that implement the target, by the shape, with
		// their method of the family's name (a struct that embeds a type has
		// its method).
		var reach []types.Object
		famReach := false
		seenObj := make(map[types.Object]bool)
		for _, v := range views {
			for _, x := range v.converted() {
				if !dt.implementedBy(x) {
					continue
				}
				m := lookupMethod(x, named.pkg, name)
				if m == nil || !r.inWorkspace(origin(m)) || seenObj[origin(m)] {
					continue
				}
				seenObj[origin(m)] = true
				reach = append(reach, origin(m))
				famReach = famReach || keys.has[keyOf(origin(m))]
			}
		}
		if !inFamily {
			// A type of the family that implements the interface and can reach it
			// brings the interface's method in.
			if !famReach {
				continue
			}
			add(named.obj, dt, false)
			inFamily = true
		}
		if inFamily && !r.allLoaded {
			// The interface's method is in the family: the types of the whole
			// module that implement it and can reach it are the family's, and
			// they can be anywhere.
			r.wantAll = true
			continue
		}
		for _, m := range reach {
			add(m, dt, true)
		}
	}
	return added
}

// newDynTarget returns the shape of the interface of the tie t (asserted).
func newDynTarget(t tie, v *view) *dynTarget {
	it, ok := t.dst.Underlying().(*types.Interface)
	if !ok || it.NumMethods() == 0 {
		return nil
	}
	dt := &dynTarget{text: types.TypeString(t.dst, func(p *types.Package) string { return p.Name() })}
	if v != nil && v.fset != nil && t.pos.IsValid() {
		dt.site = v.fset.Position(t.pos)
	}
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
func (dt *dynTarget) implementedBy(x types.Type) bool {
	for _, m := range dt.methods {
		fn := lookupMethod(x, m.pkg, m.name)
		if fn == nil || sigText(fn) != m.sig {
			return false
		}
	}
	return true
}
