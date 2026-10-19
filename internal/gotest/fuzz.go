package gotest

import (
	"go/ast"
	"go/token"
	"strconv"
)

// rewriteFuzz rewrites the calls f.Fuzz(ff) in the body of the fuzz target fn (func FuzzXxx(f
// *testing.F)) to the call that gocpp's testing package can run (stdlib/testing/fuzz.gp, GocppFuzz):
//
//	f.Fuzz(func(t *testing.T, a int, b []byte) { ... })
//
// becomes
//
//	f.GocppFuzz(func(gocppT *testing.T, gocppVals []interface{}) {
//		(func(t *testing.T, a int, b []byte) { ... })(gocppT, gocppVals[0].(int), gocppVals[1].([]byte))
//	}, []interface{}{*new(int), *new([]byte)})
//
// because Go's F.Fuzz calls ff with reflect, which gocpp has not got. ff is a function literal, or
// the name of a function of the package (decls). A call it cannot rewrite (another kind of
// argument, a parameter type that is not one of the types that a fuzz target may have) is left
// alone: F.Fuzz then skips the target and says why.
func rewriteFuzz(fn *ast.FuncDecl, decls map[string]*ast.FuncDecl) {
	if fn.Body == nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) != 1 {
		return
	}
	recv := fn.Type.Params.List[0].Names[0].Name
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Fuzz" {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != recv {
			return true
		}
		var ft *ast.FuncType
		var callee ast.Expr
		switch a := call.Args[0].(type) {
		case *ast.FuncLit:
			ft, callee = a.Type, a
		case *ast.Ident:
			if d := decls[a.Name]; d != nil && d.Recv == nil && d.Type.TypeParams == nil {
				ft, callee = d.Type, a
			}
		}
		if ft == nil || ft.Params == nil {
			return true
		}
		// The parameter types, one for each name.
		var types []ast.Expr
		for _, field := range ft.Params.List {
			count := max(len(field.Names), 1)
			for i := 0; i < count; i++ {
				types = append(types, field.Type)
			}
		}
		if len(types) < 2 {
			return true
		}
		// The nodes made here lie inside the argument (the lowering finds a function literal's
		// scope by position), the zero values at the closing parenthesis of the call.
		pos, end := call.Args[0].Pos(), call.Args[0].End()-1
		zpos := call.Rparen
		tType := cloneType(types[0], pos)
		if tType == nil {
			return true
		}
		var asserts, zeros []ast.Expr
		for i, typ := range types[1:] {
			if !fuzzableType(typ) {
				return true
			}
			asserts = append(asserts, &ast.TypeAssertExpr{
				X:    &ast.IndexExpr{X: ident("gocppVals", pos), Lbrack: pos, Index: &ast.BasicLit{ValuePos: pos, Kind: token.INT, Value: strconv.Itoa(i)}, Rbrack: pos},
				Type: cloneType(typ, pos), Lparen: pos, Rparen: end,
			})
			zeros = append(zeros, &ast.StarExpr{Star: zpos, X: &ast.CallExpr{Fun: ident("new", zpos), Lparen: zpos, Args: []ast.Expr{cloneType(typ, zpos)}, Rparen: zpos}})
		}
		anyType := func(pos token.Pos) ast.Expr {
			return &ast.InterfaceType{Interface: pos, Methods: &ast.FieldList{Opening: pos, Closing: pos}}
		}
		wrapper := &ast.FuncLit{
			Type: &ast.FuncType{
				Func: pos,
				Params: &ast.FieldList{Opening: pos, Closing: pos, List: []*ast.Field{
					{Names: []*ast.Ident{ident("gocppT", pos)}, Type: tType},
					{Names: []*ast.Ident{ident("gocppVals", pos)}, Type: &ast.ArrayType{Lbrack: pos, Elt: anyType(pos)}},
				}},
			},
			Body: &ast.BlockStmt{Lbrace: pos, Rbrace: end, List: []ast.Stmt{
				&ast.ExprStmt{X: &ast.CallExpr{Fun: callee, Lparen: pos, Args: append([]ast.Expr{ident("gocppT", pos)}, asserts...), Rparen: end}},
			}},
		}
		sel.Sel.Name = "GocppFuzz"
		call.Args = []ast.Expr{wrapper, &ast.CompositeLit{
			Type: &ast.ArrayType{Lbrack: zpos, Elt: anyType(zpos)}, Lbrace: zpos, Elts: zeros, Rbrace: zpos,
		}}
		return false
	})
}

func ident(name string, pos token.Pos) *ast.Ident { return &ast.Ident{NamePos: pos, Name: name} }

// fuzzableType reports whether typ, a parameter type of a fuzz function, is by its syntax one of
// the types a fuzz target may have: []byte, string, bool, byte, rune, float32, float64, int, int8,
// int16, int32, int64, uint, uint8, uint16, uint32 or uint64.
func fuzzableType(typ ast.Expr) bool {
	switch x := typ.(type) {
	case *ast.Ident:
		switch x.Name {
		case "string", "bool", "byte", "rune", "float32", "float64", "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			return true
		}
	case *ast.ArrayType:
		if elt, ok := x.Elt.(*ast.Ident); ok && x.Len == nil && elt.Name == "byte" {
			return true
		}
	}
	return false
}

// cloneType copies a type expression made of names, selectors, pointers and slices, or returns
// nil, with the position pos for every node.
func cloneType(typ ast.Expr, pos token.Pos) ast.Expr {
	switch x := typ.(type) {
	case *ast.Ident:
		return ident(x.Name, pos)
	case *ast.SelectorExpr:
		if base, ok := x.X.(*ast.Ident); ok {
			return &ast.SelectorExpr{X: ident(base.Name, pos), Sel: ident(x.Sel.Name, pos)}
		}
	case *ast.StarExpr:
		if elem := cloneType(x.X, pos); elem != nil {
			return &ast.StarExpr{Star: pos, X: elem}
		}
	case *ast.ArrayType:
		if elem := cloneType(x.Elt, pos); elem != nil && x.Len == nil {
			return &ast.ArrayType{Lbrack: pos, Elt: elem}
		}
	}
	return nil
}
