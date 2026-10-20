package lower

import (
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
)

// The unnamed struct and interface types of the split layout (Phase 06 README §2.3, step 6.3).
//
// Go's type identity is structural: struct{ X int } in two packages is one type, and so is
// interface{ M() }. So the C++ must have one type for it in the whole program, whatever packages
// use it. A named type belongs to its package's namespace; an unnamed one belongs to the global
// namespace and is named by a hash of its identity text (gp_anon_<16 hex digits> for a struct,
// gp_anon_i<16 hex digits> for an interface), and every header whose package uses it writes the
// same definition under its own include guard:
//
//	struct gp_anon_3f2a9c01d4b5e678;                      // before the namespace's types
//	...
//	#ifndef GP_ANON_3F2A9C01D4B5E678
//	#define GP_ANON_3F2A9C01D4B5E678
//	struct gp_anon_3f2a9c01d4b5e678 {
//	    ::gpkg_geom::Point p;                             // the types of the packages: fully qualified
//	    ...
//	};
//	#endif
//
// The text is made once, by the first package that uses the type (the owner), in the global
// namespace's spelling (every name fully qualified), so all the headers have the same text and
// a translation unit that includes several of them sees one definition. The definition of an
// interface struct has its inline members (gp_desc, the wrappers) in a second guarded block,
// after all the structs, as the unit layout writes them. A header also writes the unnamed types
// that the definitions of the unnamed types it uses mention (sharedDef.refs), so that a type
// that C++ needs complete is complete.
//
// A shared type has no descriptor of its own: the unnamed interface types intern theirs
// (ifaces.go: gp_desc) and the composite descriptors of the types that hold them are interned
// at program start by their identity texts (typedesc.go), so the packages that use the same
// type get the same descriptor.

// A sharedDef is the definition of one shared type, made by finishShared.
type sharedDef struct {
	decls []cppir.Decl // the struct, its static_assert and a blank line
	impl  []cppir.Decl // an interface's inline members (gp_desc, the wrappers), after the structs
	post  []cppir.Decl // an instance of a generic type: its method definitions, after the prototypes
	refs  []sharedItem // the shared types that decls and impl mention
	made  bool
}

// A sharedItem is one shared type: a struct or an interface.
type sharedItem struct {
	st    *structInfo
	iface *ifaceInfo
}

func (it sharedItem) def() *sharedDef {
	if it.st != nil {
		return it.st.shared
	}
	return it.iface.shared
}

func (it sharedItem) name() string {
	if it.st != nil {
		return it.st.name
	}
	return it.iface.name
}

// ns returns the namespace that the item is written in: "" for the global one (an unnamed type),
// the owner's for an instance of a generic type.
func (it sharedItem) ns() string {
	if it.st != nil && it.st.inst != nil || it.iface != nil && it.iface.inst != nil {
		return it.owner().ns
	}
	return ""
}

// inst returns the instance of a generic type that the item is, or nil.
func (it sharedItem) inst() *instType {
	if it.st != nil {
		return it.st.inst
	}
	return it.iface.inst
}

func (it sharedItem) owner() *gen {
	if it.st != nil {
		return it.st.owner
	}
	return it.iface.owner
}

// guard returns the include guard macro of the definition: GP_ANON_3F2A..., GP_ANON_I3F2A....
func (it sharedItem) guard() string {
	return strings.ToUpper(it.name())
}

// noteShared records that the package of g uses the shared type x (a *structInfo or an
// *ifaceInfo), so that its header writes it, and that the shared type being made mentions it.
func (g *gen) noteShared(x any) {
	var it sharedItem
	switch x := x.(type) {
	case *structInfo:
		it = sharedItem{st: x}
	case *ifaceInfo:
		it = sharedItem{iface: x}
	default:
		return
	}
	if g.rec != nil {
		*g.rec = append(*g.rec, it)
	}
	if g.usedShared == nil {
		g.usedShared = make(map[sharedItem]bool)
	}
	if !g.usedShared[it] {
		g.usedShared[it] = true
		g.sharedList = append(g.sharedList, it)
	}
}

// finishShared makes the definitions of the shared types, each by its owner, until nothing new
// is found (the definition of one finds the types in its fields).
func (p *Program) finishShared() {
	for i := 0; i < len(p.sharedOrder); i++ {
		it := p.sharedOrder[i]
		if d := it.def(); !d.made {
			d.made = true
			if inst := it.inst(); inst != nil {
				it.owner().makeInstance(inst)
				continue
			}
			it.owner().makeShared(it)
		}
	}
	p.placeInstanceBodies()
}

// makeShared makes the definition of the shared type it, which this package owns.
func (g *gen) makeShared(it sharedItem) {
	d := it.def()
	var refs []sharedItem
	g.rec = &refs
	g.global++
	defer func() { g.rec = nil; g.global-- }()
	if it.st != nil {
		d.decls = append(g.structDef(it.st), &cppir.Blank{})
	} else {
		d.decls = g.ifaceDef(it.iface)
		p := implParts{split: true}
		g.ifaceImpl(it.iface, &p)
		d.impl = p.inl
	}
	for _, r := range refs {
		if r != it {
			d.refs = append(d.refs, r)
		}
	}
}

// sharedClosure returns the shared types that the package uses and the ones their definitions
// mention, in the order they were found.
func (g *gen) sharedClosure() []sharedItem {
	var list []sharedItem
	seen := make(map[sharedItem]bool)
	var add func(it sharedItem)
	add = func(it sharedItem) {
		if seen[it] {
			return
		}
		seen[it] = true
		list = append(list, it)
		for _, r := range it.def().refs {
			add(r)
		}
	}
	for _, it := range g.sharedList {
		add(it)
	}
	return list
}

// guarded returns decls inside #ifndef/#define/#endif of the include guard of it, suffix is "" for
// the definition and "_IMPL" for the inline members.
func guarded(it sharedItem, suffix string, decls []cppir.Decl) []cppir.Decl {
	if len(decls) == 0 {
		return nil
	}
	out := []cppir.Decl{&cppir.Directive{Text: "ifndef " + it.guard() + suffix}, &cppir.Directive{Text: "define " + it.guard() + suffix}}
	out = append(out, decls...)
	return append(out, &cppir.Directive{Text: "endif"})
}

// put adds decls to the section of the namespace that the item is written in.
func (it sharedItem) put(sec *sections, decls ...cppir.Decl) {
	ns := it.ns()
	sec.addNS(ns == "", ns, decls...)
}

// valueDepItems returns the shared structs (instances of generic types and unnamed structs) that the struct of it
// holds by value: as a field, or as the element of an array field (an unnamed struct too).
func (g *gen) valueDepItems(it sharedItem) []sharedItem {
	si := it.st
	if si == nil {
		return nil
	}
	var ts []types.Type
	if si.under != nil {
		ts = append(ts, si.under)
	} else if si.st != nil {
		for i := range si.st.NumFields() {
			ts = append(ts, si.st.Field(i).Type())
		}
	}
	var out []sharedItem
	for _, t := range ts {
		for {
			a, ok := types.Unalias(t).(*types.Array)
			if !ok {
				break
			}
			t = a.Elem()
		}
		switch t := types.Unalias(t).(type) {
		case *types.Named:
			if isInstance(t) {
				if d := g.prog.instTypes[g.canonText(t)]; d != nil && d.si != nil {
					out = append(out, sharedItem{st: d.si})
				}
			}
		case *types.Struct:
			// An unnamed struct is a shared struct of its own (and holds instances by value too).
			if si := g.prog.anonStructs[g.anonKey(t)]; si != nil {
				out = append(out, sharedItem{st: si})
			}
		}
	}
	return out
}
