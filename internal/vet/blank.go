package vet

import (
	"go/ast"
	"go/types"
)

// typeParenBlanks gives the parenthesized blank identifiers of the files a
// type in info. go/types records no type for the blank identifier on the
// left of an assignment, a range clause or a declaration, and none for the
// parentheses around it: (_) = x, for (_) = range s. Analyzers that look up
// the type of every expression they walk (x/tools refactor/satisfy, which
// printf uses to find the String and Error methods) panic on the
// ParenExpr: "no type for *ast.ParenExpr". For the bare blank identifier
// satisfy answers the invalid type; the parentheses get the same, which is
// the type that go/types gives to every other expression that has none.
//
// It must run after illTyped, which takes an invalid type in info.Types for
// a part that comes from C/C++.
func typeParenBlanks(info *types.Info, files []*ast.File) {
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			p, ok := n.(*ast.ParenExpr)
			if !ok {
				return true
			}
			if _, has := info.Types[p]; has {
				return true
			}
			x := p.X
			for {
				q, ok := x.(*ast.ParenExpr)
				if !ok {
					break
				}
				x = q.X
			}
			if id, ok := x.(*ast.Ident); ok && id.Name == "_" {
				info.Types[p] = types.TypeAndValue{Type: types.Typ[types.Invalid]}
			}
			return true
		})
	}
}
