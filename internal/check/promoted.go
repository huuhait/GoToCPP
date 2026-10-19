package check

import (
	"go/ast"
	"go/types"
)

// Promoted members of embedded C/C++ types (Phase 05 step 5.8).
//
//	type MV struct {
//		std.vector[int]
//		n int
//	}
//	var mv MV
//	mv.push_back(1) // mv.vector.push_back(1)
//	mv.size()       // mv.vector.size()
//
// go/types has no type for std.vector[int] (every C/C++ type is invalid), so it knows none of its
// members and reports "mv.push_back undefined". A selector x.name that go/types can't find on a
// struct (or a pointer to one) whose embedded fields have exactly one C/C++ type, directly or
// through embedded structs, is that type's member: Go's rule for the depth would make it the
// member of the embedded field (x.vector.name, x.Mid.vector.name). The false error is dropped
// (falseErrors), the selector is opaque like x.vector.name would be (opaquePass.cOperand), and
// codegen writes the path of embedded fields (Package.PromotedPath) before the member: the C++
// compiler checks that the member exists, as for every member of a C/C++ value.
//
// With several embedded C/C++ types (also in different depths) check can't know which one has the
// member: go/types' error stays, and the gate says to name the field. A Go member that go/types
// finds is the right one only when every embedded C/C++ type is deeper than it (a member of a
// C/C++ type is one level deeper than its field); at the same or a shallower depth Go would pick
// the C++ member, or call the name ambiguous, and check can't tell whether the type has it: the
// gate says so (cMayShadow, featPromotedCppShadow).
// Interfaces: a method promoted from a C/C++ type is not in the method set that go/types knows,
// so the type does not implement an interface that needs it (the error says it lacks the method).

// PromotedPath returns the indexes of the embedded fields from the type of x to the one C/C++
// type that the selector sel = x.name takes its member from (see above), or nil if sel is not such
// a selector.
func (p *Package) PromotedPath(sel *ast.SelectorExpr) []int {
	if p == nil || p.Types == nil || p.Info == nil {
		return nil
	}
	if r, ok := p.cppSelOf(sel); ok {
		return r.path // decided by the second pass (cppmembers.go)
	}
	if p.Info.Selections[sel] != nil {
		return nil
	}
	tv, ok := p.Info.Types[sel.X]
	if !ok || !tv.IsValue() {
		return nil
	}
	return promotedPath(p.Types, tv.Type, sel.Sel.Name)
}

// promotedPath returns the path of the embedded fields from the struct type t (or a pointer to
// one) to the only C/C++ type that name can be a member of, or nil.
func promotedPath(pkg *types.Package, t types.Type, name string) []int {
	if t == nil || t == types.Typ[types.Invalid] || name == "_" {
		return nil
	}
	if obj, index, _ := types.LookupFieldOrMethod(t, true, pkg, name); obj != nil || index != nil {
		return nil // a Go member, or an ambiguous one: go/types' error is the right one
	}
	if goMember(t, name, make(map[types.Type]bool)) {
		return nil // a Go member that the package can't refer to (an unexported field of another package)
	}
	var found [][]int
	opaqueEmbedded(t, nil, make(map[*types.Struct]bool), &found)
	if len(found) != 1 {
		return nil
	}
	return found[0]
}

// cMayShadow reports whether the selector name on the type t, which go/types resolved to a Go
// member (a field or a method), might be another member: the struct embeds a C/C++ type whose
// members would be at the same depth (Go: ambiguous) or a shallower one (Go: the C++ member wins),
// and check can't know whether that type has a member of that name. A C/C++ type at a deeper
// depth than the Go member never matters. The depth of a member of an embedded field at the
// path p (of n indexes) is n; the depth of a Go member is the length of its index path minus one.
func cMayShadow(pkg *types.Package, t types.Type, name string) bool {
	if t == nil || t == types.Typ[types.Invalid] || name == "_" {
		return false
	}
	obj, index, _ := types.LookupFieldOrMethod(t, true, pkg, name)
	if obj == nil || len(index) == 0 {
		return false
	}
	var found [][]int
	opaqueEmbedded(t, nil, make(map[*types.Struct]bool), &found)
	for _, p := range found {
		if len(p) <= len(index)-1 {
			return true
		}
	}
	return false
}

// goMember reports whether the type t (a struct, a pointer to one, or a named type) has a field
// or method called name, at any depth through embedded fields and whatever its package: LookupFieldOrMethod
// doesn't find an unexported name of another package, and go/types' error for it ("cannot refer
// to unexported field") is the right one.
func goMember(t types.Type, name string, seen map[types.Type]bool) bool {
	t = types.Unalias(t)
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	if n, ok := t.(*types.Named); ok {
		for i := range n.NumMethods() {
			if n.Method(i).Name() == name {
				return true
			}
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Struct:
		for i := range u.NumFields() {
			f := u.Field(i)
			if f.Name() == name || f.Embedded() && goMember(f.Type(), name, seen) {
				return true
			}
		}
	case *types.Interface:
		for i := range u.NumMethods() {
			if u.Method(i).Name() == name {
				return true
			}
		}
	}
	return false
}

// opaqueEmbedded appends to found the paths of the embedded fields of the struct type t (or a
// pointer to one) that have a C/C++ type, going through the embedded structs; prefix is the path
// to t, and stack holds the structs on the way (a struct that embeds a pointer to itself).
func opaqueEmbedded(t types.Type, prefix []int, stack map[*types.Struct]bool, found *[][]int) {
	if p, ok := types.Unalias(t).Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	st, ok := types.Unalias(t).Underlying().(*types.Struct)
	if !ok || stack[st] {
		return
	}
	stack[st] = true
	defer delete(stack, st)
	for i := range st.NumFields() {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		path := append(append([]int(nil), prefix...), i)
		if cType(f.Type()) {
			*found = append(*found, path)
		} else {
			opaqueEmbedded(f.Type(), path, stack, found)
		}
	}
}

// cType reports whether the type of an embedded field is a C/C++ type: go/types left it invalid
// (std.vector[int], C.Point, an alias of one), or it is a pointer to one, or a named type
// declared over one (type Vec std.vector[int]).
func cType(t types.Type) bool {
	t = types.Unalias(t)
	if p, ok := t.Underlying().(*types.Pointer); ok {
		if _, named := t.(*types.Named); !named {
			t = types.Unalias(p.Elem())
		}
	}
	if b, ok := t.Underlying().(*types.Basic); ok {
		return b.Kind() == types.Invalid
	}
	return false
}
