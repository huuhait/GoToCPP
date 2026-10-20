package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// Prune returns src without the top-level declarations that main and c do not need (a test made
// from a generated program has only the types and values of its cases), and without the
// imports they do not use. The reachability is by name, so it may keep more than needed.
func Prune(src string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return "", err
	}
	type decl struct {
		node  ast.Node
		names []string // declared names (for a method: none)
		recv  string   // receiver base type of a method
		keep  bool
	}
	var ds []*decl
	byName := map[string][]*decl{}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			for _, sp := range d.Specs {
				x := &decl{node: sp}
				switch sp := sp.(type) {
				case *ast.TypeSpec:
					x.names = []string{sp.Name.Name}
				case *ast.ValueSpec:
					for _, n := range sp.Names {
						if n.Name != "_" {
							x.names = append(x.names, n.Name)
						}
					}
				}
				ds = append(ds, x)
				for _, n := range x.names {
					byName[n] = append(byName[n], x)
				}
			}
		case *ast.FuncDecl:
			x := &decl{node: d}
			if d.Recv == nil {
				x.names = []string{d.Name.Name}
				byName[d.Name.Name] = append(byName[d.Name.Name], x)
			} else {
				x.recv = recvBase(d.Recv.List[0].Type)
			}
			ds = append(ds, x)
		}
	}
	var work []*decl
	mark := func(x *decl) {
		if !x.keep {
			x.keep = true
			work = append(work, x)
		}
	}
	for _, n := range []string{"main", "c"} {
		for _, x := range byName[n] {
			mark(x)
		}
	}
	for len(work) > 0 {
		x := work[len(work)-1]
		work = work[:len(work)-1]
		ast.Inspect(x.node, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				for _, y := range byName[id.Name] {
					mark(y)
				}
			}
			return true
		})
		// the methods of a kept type come with it
		for _, n := range x.names {
			for _, y := range ds {
				if y.recv == n {
					mark(y)
				}
			}
		}
		if len(work) == 0 {
			// a method whose type is kept may have been added after the type was scanned
			for _, y := range ds {
				if y.recv != "" && !y.keep {
					for _, t := range byName[y.recv] {
						if t.keep {
							mark(y)
						}
					}
				}
			}
		}
	}
	used := map[string]bool{}
	var kept []*decl
	for _, x := range ds {
		if x.keep {
			kept = append(kept, x)
			ast.Inspect(x.node, func(n ast.Node) bool {
				if se, ok := n.(*ast.SelectorExpr); ok {
					if id, ok := se.X.(*ast.Ident); ok {
						used[id.Name] = true
					}
				}
				return true
			})
		}
	}
	var imps []string
	for _, im := range f.Imports {
		p := strings.Trim(im.Path.Value, `"`)
		name := p[strings.LastIndex(p, "/")+1:]
		if used[name] {
			imps = append(imps, p)
		}
	}
	sort.Strings(imps)
	var b strings.Builder
	b.WriteString("package main\n\n")
	switch len(imps) {
	case 0:
	case 1:
		fmt.Fprintf(&b, "import %q\n\n", imps[0])
	default:
		b.WriteString("import (\n")
		for _, p := range imps {
			fmt.Fprintf(&b, "\t%q\n", p)
		}
		b.WriteString(")\n\n")
	}
	for _, x := range kept {
		start, end := x.node.Pos(), x.node.End()
		text := src[fset.Position(start).Offset:fset.Position(end).Offset]
		switch x.node.(type) {
		case *ast.TypeSpec:
			text = "type " + text
		case *ast.ValueSpec:
			text = "var " + text
		}
		b.WriteString(text + "\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func recvBase(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return recvBase(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return recvBase(e.X)
	case *ast.IndexListExpr:
		return recvBase(e.X)
	}
	return ""
}
