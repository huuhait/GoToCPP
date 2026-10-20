package main

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/check"
)

// fmtIOFuncs are the functions of package fmt that take an io.Writer or an io.Reader as their
// first argument.
var fmtIOFuncs = map[string]bool{
	"Fprint": true, "Fprintf": true, "Fprintln": true,
	"Fscan": true, "Fscanf": true, "Fscanln": true,
}

// usesFmtIO reports whether the package calls (or takes as a value) a function of fmt that needs
// the package io: Fprint and Fscan with a writer or a reader of its own, or used as a value (a
// call with a fmt.State, the argument of a Format method, needs no io). Go's fmt imports io, so
// such a program has the package io also when it does not import it itself (a type with a Write
// method is converted to io.Writer at the call): the build adds it to the program.
func usesFmtIO(p *check.Package) bool {
	if p.Info == nil {
		return false
	}
	// The names of the calls that pass a fmt.State, which need no io.
	skip := make(map[*ast.Ident]bool)
	for _, f := range p.Load.Files {
		if f.AST == nil {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) == 0 {
				return true
			}
			var id *ast.Ident
			switch fun := ast.Unparen(c.Fun).(type) {
			case *ast.SelectorExpr:
				id = fun.Sel
			case *ast.Ident:
				id = fun
			default:
				return true
			}
			if t := p.Info.TypeOf(c.Args[0]); t != nil {
				if nt, ok := types.Unalias(t).(*types.Named); ok && nt.Obj().Pkg() != nil &&
					nt.Obj().Pkg().Path() == "fmt" && nt.Obj().Name() == "State" {
					skip[id] = true
				}
			}
			return true
		})
	}
	for id, obj := range p.Info.Uses {
		if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" && fmtIOFuncs[fn.Name()] && !skip[id] {
			return true
		}
	}
	return false
}
