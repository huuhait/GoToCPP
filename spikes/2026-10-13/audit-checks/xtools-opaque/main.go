package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"golang.org/x/tools/refactor/satisfy"
)

const src = `package p

import "C"
import "fmt"

type Stringer interface{ String() string }

func f(s string) int {
	n := C.strlen(C.cstr(s)) + len(s)
	w := C.InitWindow(800, 600)
	if w == nil {
		return 0
	}
	var x Stringer = C.make_thing()
	fmt.Printf("%d %v\n", C.count, x)
	return n
}
`

func main() {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		panic(err)
	}
	var errs []error
	conf := types.Config{FakeImportC: true, Importer: importer.Default(), Error: func(e error) { errs = append(errs, e) }}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Implicits: map[ast.Node]types.Object{}, Scopes: map[ast.Node]*types.Scope{}, Instances: map[*ast.Ident]types.Instance{}, FileVersions: map[*ast.File]string{}}
	pkg, _ := conf.Check("p", fset, []*ast.File{f}, info)
	fmt.Println("type errors:", len(errs), errs)

	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("satisfy PANIC:", r)
			}
		}()
		var fd satisfy.Finder
		fd.Find(info, []*ast.File{f})
		fmt.Println("satisfy ok")
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("ssa PANIC:", r)
			}
		}()
		prog := ssa.NewProgram(fset, ssa.SanityCheckFunctions)
		for _, imp := range pkg.Imports() {
			prog.CreatePackage(imp, nil, nil, true)
		}
		sp := prog.CreatePackage(pkg, []*ast.File{f}, info, false)
		sp.Build()
		_ = ssautil.AllFunctions(prog)
		fmt.Println("ssa ok")
	}()
}
