package server

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
)

// The fields of an anonymous struct type (struct{ x int } written where a type
// is wanted, not as the definition of a named type) are the fields of every
// identical struct type: Hidden() struct{ x int } and ShowHidden(h struct{ x
// int }) take the same values, and renaming x in one and not in the other
// stops them being identical. So a rename of such a field renames the field in
// every identical struct type of the module (a struct type is identical to
// another when its fields have the same names, types, tags and embedding, and,
// for an unexported name, the same package).

// An anonField is a field of an anonymous struct type: the shape of the
// struct, and the index of the field in it.
type anonField struct {
	canon string
	index int
}

// An anonInfo is the anonymous struct types of a view's package.
type anonInfo struct {
	byCanon map[string][]*types.Struct
	fields  map[*types.Var]anonField
}

// anonStructs returns the anonymous struct types of the view, once for each
// checked package.
func (v *view) anonStructs() *anonInfo {
	build := func() any {
		ai := &anonInfo{byCanon: make(map[string][]*types.Struct), fields: make(map[*types.Var]anonField)}
		if v.info == nil {
			return ai
		}
		for _, f := range v.files {
			defs := make(map[*ast.StructType]bool)
			ast.Inspect(f, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.TypeSpec:
					if st, ok := n.Type.(*ast.StructType); ok && !n.Assign.IsValid() {
						defs[st] = true // the definition of a named type
					}
				case *ast.StructType:
					if defs[n] {
						return true
					}
					if st, ok := v.info.TypeOf(n).(*types.Struct); ok && st.NumFields() > 0 {
						c := structCanon(st)
						ai.byCanon[c] = append(ai.byCanon[c], st)
						for i := range st.NumFields() {
							ai.fields[st.Field(i)] = anonField{c, i}
						}
					}
				}
				return true
			})
		}
		return ai
	}
	if v.pkg == nil || v.test {
		return build().(*anonInfo)
	}
	return v.pkg.Memo(anonKey{}, build).(*anonInfo)
}

type anonKey struct{}

// structCanon is a text that is the same for identical struct types, in
// whichever check they are.
func structCanon(st *types.Struct) string {
	var b strings.Builder
	for i := range st.NumFields() {
		f := st.Field(i)
		pkg := ""
		if !f.Exported() && f.Pkg() != nil {
			pkg = f.Pkg().Path()
		}
		fmt.Fprintf(&b, "%s|%s|%s|%v|%q;", pkg, f.Name(), types.TypeString(f.Type(), func(p *types.Package) string { return p.Path() }), f.Embedded(), st.Tag(i))
	}
	return b.String()
}

// relatedAnonFields adds to keys, for a field of an anonymous struct type, the
// same field of every identical struct type of the views, and reports whether
// it added one.
func (r *searcher) relatedAnonFields(views []*view, keys *keySet) bool {
	if r.s.wideFamily || len(r.sym.objs) != 1 {
		return false
	}
	if f, ok := r.sym.objs[0].(*types.Var); !ok || !f.IsField() {
		return false
	}
	added := false
	for _, k := range append([]objKey(nil), keys.list...) {
		for _, v := range views {
			// Only the package that declares the field has the struct type.
			if k.pkg != "" && (v.types == nil || v.types.Path() != k.pkg) {
				continue
			}
			f, ok := v.resolve(k).(*types.Var)
			if !ok {
				continue
			}
			af, ok := v.anonStructs().fields[f]
			if !ok {
				continue
			}
			for _, w := range views {
				for _, st := range w.anonStructs().byCanon[af.canon] {
					if keys.add(keyOf(st.Field(af.index))) {
						added = true
					}
				}
			}
		}
	}
	return added
}
