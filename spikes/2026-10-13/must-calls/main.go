// Spike: can `F!(args)` be implemented with STOCK go/types by rewriting it into
// hidden generic helper calls (arity chosen from syntax), then validating after checking?
package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"strings"
)

// The hidden helper package the front-end would import as gp_must.
const mustSrc = `package must

func M1[T any](v T, err error) T             { return v }       // x := F!()
func M2[A, B any](a A, b B, err error) (A, B) { return a, b }   // a, b := F!()
func Any(results ...any)                      {}                // F!() as a statement (any arity)
`

// What the front-end would produce from:
//   data := os.ReadFile!(path)        n := strconv.Atoi!(string(data))
//   f := os.Create!(path + ".bak")    f.Write!(data)    f.Close!()
const userSrc = `package main

import (
	"os"
	"strconv"

	gp_must "gocpp/must"
)

func load(path string) (int, error) {
	data := gp_must.M1(os.ReadFile(path))
	n := gp_must.M1(strconv.Atoi(string(data)))
	f := gp_must.M1(os.Create(path + ".bak"))
	gp_must.Any(f.Write(data))
	gp_must.Any(f.Close())
	return n * 2, nil
}

func lookup(m map[string]int, k string) (int, bool) { v, ok := m[k]; return v, ok }

func bad1() int { return gp_must.M1(lookup(nil, "k")) }   // last result is bool, not error
func bad2()     { gp_must.Any(lookup(nil, "k")) }         // statement form: must be caught by OUR post-check
`

type imp struct {
	fset *token.FileSet
	must *types.Package
	src  types.Importer
}

func (i imp) Import(path string) (*types.Package, error) {
	if path == "gocpp/must" {
		return i.must, nil
	}
	return i.src.Import(path)
}

func main() {
	fset := token.NewFileSet()
	src := importer.ForCompiler(fset, "source", nil)

	mf, _ := parser.ParseFile(fset, "must.go", mustSrc, 0)
	mustPkg, err := (&types.Config{Importer: src}).Check("gocpp/must", fset, []*ast.File{mf}, nil)
	if err != nil {
		panic(err)
	}

	uf, err := parser.ParseFile(fset, "user.gp", userSrc, 0)
	if err != nil {
		panic(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: imp{fset, mustPkg, src}, Error: func(e error) { fmt.Println("  go/types error:", e) }}
	conf.Check("main", fset, []*ast.File{uf}, info)

	fmt.Println("inferred types of the must results:")
	for id, obj := range info.Defs {
		if v, ok := obj.(*types.Var); ok && (id.Name == "data" || id.Name == "n" || id.Name == "f") {
			fmt.Printf("  %-5s %s\n", id.Name, v.Type())
		}
	}

	fmt.Println("our post-check of statement-form F!() (inner call must end in error):")
	errType := types.Universe.Lookup("error").Type()
	ast.Inspect(uf, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Any" {
			return true
		}
		inner := call.Args[0]
		t := info.Types[inner].Type
		var last types.Type = t
		if tup, ok := t.(*types.Tuple); ok {
			last = tup.At(tup.Len() - 1).Type()
		}
		verdict := "OK"
		if !types.AssignableTo(last, errType) {
			verdict = "ERROR: last result is " + last.String() + ", not error"
		}
		fmt.Printf("  %-28s results=%-18s %s\n", types.ExprString(inner), t, verdict)
		return true
	})

	fmt.Println("stock go/printer with '!' and '#' inside identifier names:")
	call := &ast.ExprStmt{X: &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: ast.NewIdent("os"), Sel: ast.NewIdent("ReadFile!")},
		Args: []ast.Expr{ast.NewIdent("path")},
	}}
	mac := &ast.ExprStmt{X: &ast.CallExpr{
		Fun:  ast.NewIdent("#assert"),
		Args: []ast.Expr{&ast.BinaryExpr{X: ast.NewIdent("n"), Op: token.GTR, Y: &ast.BasicLit{Kind: token.INT, Value: "0"}}},
	}}
	var sb strings.Builder
	printer.Fprint(&sb, token.NewFileSet(), call)
	sb.WriteString("\n")
	printer.Fprint(&sb, token.NewFileSet(), mac)
	for _, l := range strings.Split(sb.String(), "\n") {
		fmt.Println("  " + l)
	}
	_ = os.Stdout
}
