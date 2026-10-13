package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
)

// Build what OUR patched parser would produce, then print it with the STOCK printer.
const src = `package main

import (
	"fmt"
	"::std"
	"<vector>"
	"./physics.h"
)

func main() {
	m := std.PLACEHOLDER[std.string, int]{}
	var a std.array[int, N]
	t := node.KW()
	fmt.Println(m, a, t)
}
`

func main() {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.gp", src, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			switch x.Name {
			case "PLACEHOLDER":
				x.Name = "map" // Go keyword as a selector name (parser patch #1)
			case "KW":
				x.Name = "type"
			}
		case *ast.IndexListExpr:
			if id, ok := x.Indices[1].(*ast.Ident); ok && id.Name == "N" {
				x.Indices[1] = &ast.BasicLit{ValuePos: id.Pos(), Kind: token.INT, Value: "5"} // patch #2
			}
		}
		return true
	})
	ast.SortImports(fset, f)
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8} // gofmt settings
	if err := cfg.Fprint(os.Stdout, fset, f); err != nil {
		panic(err)
	}
}
