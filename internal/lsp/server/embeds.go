package server

import (
	"go/ast"
	"go/types"
)

// The embeddings of a package (struct types and interface types that embed
// another type), for the rename of a method: the method set of a type that
// embeds the type of the method is made by promotion (the shallowest method of
// a name, an ambiguity at one depth hides all), so renaming a method can
// change the method set of a type that does not name it, in a package that
// does not mention the new name: the embedder gets an ambiguous or a hidden
// method, and stops implementing an interface, or calls another method.

// An embedRec is a struct or interface type with the types it embeds.
type embedRec struct {
	outer    types.Type
	embedded []types.Type
}

// embeds returns the embeddings of the view's package, once for each checked
// package: the named types, the anonymous ones and the ones declared in a
// function.
func (v *view) embeds() []embedRec {
	build := func() any {
		var out []embedRec
		if v.info == nil {
			return out
		}
		for _, f := range v.files {
			named := make(map[ast.Node]types.Type)
			ast.Inspect(f, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.TypeSpec:
					if !n.Assign.IsValid() {
						if tn, ok := v.info.Defs[n.Name].(*types.TypeName); ok {
							named[n.Type] = tn.Type()
						}
					}
				case *ast.StructType:
					var emb []types.Type
					for _, fl := range n.Fields.List {
						if len(fl.Names) == 0 {
							if t := v.info.TypeOf(fl.Type); t != nil {
								emb = append(emb, t)
							}
						}
					}
					if len(emb) > 0 {
						outer := named[n]
						if outer == nil {
							outer = v.info.TypeOf(n)
						}
						if outer != nil {
							out = append(out, embedRec{outer, emb})
						}
					}
				case *ast.InterfaceType:
					var emb []types.Type
					if n.Methods != nil {
						for _, fl := range n.Methods.List {
							if len(fl.Names) == 0 {
								if t := v.info.TypeOf(fl.Type); t != nil {
									emb = append(emb, t)
								}
							}
						}
					}
					if len(emb) > 0 {
						outer := named[n]
						if outer == nil {
							outer = v.info.TypeOf(n)
						}
						if outer != nil {
							out = append(out, embedRec{outer, emb})
						}
					}
				}
				return true
			})
		}
		return out
	}
	if v.pkg == nil || v.test {
		return build().([]embedRec)
	}
	return v.pkg.Memo(embedsKey{}, build).([]embedRec)
}

type embedsKey struct{}
