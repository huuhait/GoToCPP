// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vet

// The copylocks analyzer is the one of golang.org/x/tools
// (go/analysis/passes/copylock, BSD license, Copyright 2013 The Go
// Authors), without the checks of Go 1.22 loop variables (a loop variable
// is a new variable on each iteration in gocpp, and nothing is copied),
// and with one rule more: the types of sync and sync/atomic that must not
// be copied are known by name. Go finds them by their sync.noCopy field,
// which the declaration stubs of gocpp's standard library leave out, so
// without the names a sync.WaitGroup passed by value would go unnoticed.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var copylocksAnalyzer = &analysis.Analyzer{
	Name: "copylocks",
	Doc: `check for locks erroneously passed by value

Inadvertently copying a value containing a lock, such as sync.Mutex or
sync.WaitGroup, may cause both copies to malfunction. Generally such
values should be referred to through a pointer.`,
	Requires:         []*analysis.Analyzer{inspect.Analyzer},
	RunDespiteErrors: true,
	Run:              runCopylocks,
}

// noCopyTypes are the types that must not be copied, by package path, with
// what Go finds inside them (the innermost first), for the message.
var noCopyTypes = map[string]map[string][]string{
	"sync": {
		"Cond": {"sync.noCopy"}, "Map": {"sync.noCopy"}, "Mutex": {"sync.noCopy"}, "Once": {"sync.noCopy"},
		"Pool": {"sync.noCopy"}, "RWMutex": {"sync.noCopy"}, "WaitGroup": {"sync.noCopy"},
	},
	"sync/atomic": {
		"Bool": {"atomic.noCopy"}, "Int32": {"atomic.noCopy"}, "Int64": {"atomic.noCopy"},
		"Pointer": {"atomic.noCopy"}, "Uint32": {"atomic.noCopy"}, "Uint64": {"atomic.noCopy"},
		"Uintptr": {"atomic.noCopy"}, "Value": {"atomic.noCopy"},
	},
	"testing": {
		"T": {"sync.RWMutex", "testing.common"}, "B": {"sync.RWMutex", "testing.common"},
		"F": {"sync.RWMutex", "testing.common"},
	},
}

func runCopylocks(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.CallExpr)(nil),
		(*ast.CompositeLit)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.GenDecl)(nil),
		(*ast.RangeStmt)(nil),
		(*ast.ReturnStmt)(nil),
	}
	ins.Preorder(nodeFilter, func(node ast.Node) {
		switch node := node.(type) {
		case *ast.RangeStmt:
			checkCopyLocksRange(pass, node)
		case *ast.FuncDecl:
			checkCopyLocksFunc(pass, node.Name.Name, node.Recv, node.Type)
		case *ast.FuncLit:
			checkCopyLocksFunc(pass, "func", nil, node.Type)
		case *ast.CallExpr:
			checkCopyLocksCallExpr(pass, node)
		case *ast.AssignStmt:
			for i, x := range node.Rhs {
				if path := lockPathRhs(pass, x); path != nil && i < len(node.Lhs) {
					pass.ReportRangef(x, "assignment copies lock value to %v: %v", formatNode(pass.Fset, node.Lhs[i]), path)
				}
			}
		case *ast.GenDecl:
			if node.Tok != token.VAR {
				return
			}
			for _, spec := range node.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, x := range vs.Values {
					if path := lockPathRhs(pass, x); path != nil && i < len(vs.Names) {
						pass.ReportRangef(x, "variable declaration copies lock value to %v: %v", vs.Names[i].Name, path)
					}
				}
			}
		case *ast.CompositeLit:
			for _, x := range node.Elts {
				if kv, ok := x.(*ast.KeyValueExpr); ok {
					x = kv.Value
				}
				if path := lockPathRhs(pass, x); path != nil {
					pass.ReportRangef(x, "literal copies lock value from %v: %v", formatNode(pass.Fset, x), path)
				}
			}
		case *ast.ReturnStmt:
			for _, x := range node.Results {
				if path := lockPathRhs(pass, x); path != nil {
					pass.ReportRangef(x, "return copies lock value: %v", path)
				}
			}
		}
	})
	return nil, nil
}

func checkCopyLocksCallExpr(pass *analysis.Pass, ce *ast.CallExpr) {
	var id *ast.Ident
	switch fun := ce.Fun.(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	}
	if id != nil {
		if fun, ok := pass.TypesInfo.Uses[id].(*types.Builtin); ok {
			switch fun.Name() {
			case "len", "cap", "Sizeof", "Offsetof", "Alignof":
				return // the argument is used for its type only
			}
		}
	}
	for _, x := range ce.Args {
		if path := lockPathRhs(pass, x); path != nil {
			pass.ReportRangef(x, "call of %s copies lock value: %v", formatNode(pass.Fset, ce.Fun), path)
		}
	}
}

// checkCopyLocksFunc checks the receiver and the parameters (not the
// results: return T{} is fine, the return statement is checked).
func checkCopyLocksFunc(pass *analysis.Pass, name string, recv *ast.FieldList, typ *ast.FuncType) {
	if recv != nil && len(recv.List) > 0 {
		expr := recv.List[0].Type
		if path := lockPath(pass.TypesInfo.Types[expr].Type, nil); path != nil {
			pass.ReportRangef(expr, "%s passes lock by value: %v", name, path)
		}
	}
	if typ.Params != nil {
		for _, field := range typ.Params.List {
			expr := field.Type
			if path := lockPath(pass.TypesInfo.Types[expr].Type, nil); path != nil {
				pass.ReportRangef(expr, "%s passes lock by value: %v", name, path)
			}
		}
	}
}

func checkCopyLocksRange(pass *analysis.Pass, r *ast.RangeStmt) {
	for _, e := range []ast.Expr{r.Key, r.Value} {
		if e == nil {
			continue
		}
		id, isID := e.(*ast.Ident)
		if isID && id.Name == "_" {
			continue
		}
		var typ types.Type
		if r.Tok == token.DEFINE {
			if !isID {
				continue
			}
			if obj := pass.TypesInfo.Defs[id]; obj != nil {
				typ = obj.Type()
			}
		} else {
			typ = pass.TypesInfo.Types[e].Type
		}
		if typ == nil {
			continue
		}
		if path := lockPath(typ, nil); path != nil {
			pass.Reportf(e.Pos(), "range var %s copies lock: %v", formatNode(pass.Fset, e), path)
		}
	}
}

type typePath []string

// String pretty-prints a typePath, outermost type first.
func (path typePath) String() string {
	var buf bytes.Buffer
	for i := range path {
		if i > 0 {
			fmt.Fprint(&buf, " contains ")
		}
		fmt.Fprint(&buf, path[len(path)-i-1])
	}
	return buf.String()
}

func lockPathRhs(pass *analysis.Pass, x ast.Expr) typePath {
	x = ast.Unparen(x)
	switch x := x.(type) {
	case *ast.CompositeLit:
		return nil
	case *ast.CallExpr:
		return nil // a call may return a zero value
	case *ast.StarExpr:
		if _, ok := ast.Unparen(x.X).(*ast.CallExpr); ok {
			return nil // a call may return a pointer to a zero value
		}
	}
	if tv, ok := pass.TypesInfo.Types[x]; ok && tv.IsValue() {
		return lockPath(tv.Type, nil)
	}
	return nil
}

// lockPath describes where a lock is in typ: nil if there is none.
func lockPath(typ types.Type, seen map[types.Type]bool) typePath {
	if typ == nil || seen[typ] {
		return nil
	}
	if seen == nil {
		seen = make(map[types.Type]bool)
	}
	seen[typ] = true
	if _, ok := types.Unalias(typ).(*types.TypeParam); ok {
		return nil
	}
	for {
		atyp, ok := typ.Underlying().(*types.Array)
		if !ok {
			break
		}
		typ = atyp.Elem()
	}
	if tup, ok := typ.Underlying().(*types.Tuple); ok {
		for v := range tup.Variables() {
			if sub := lockPath(v.Type(), seen); sub != nil {
				return append(sub, typ.String())
			}
		}
		return nil
	}
	// Only structs hold a lock by value: interfaces and pointers are safe.
	styp, ok := typ.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	// A pointer to the type is a sync.Locker, the value is not.
	if types.Implements(types.NewPointer(typ), lockerType) && !types.Implements(typ, lockerType) {
		return []string{typ.String()}
	}
	// The types of sync that have a noCopy field in Go, and print as such:
	// "sync.WaitGroup contains sync.noCopy".
	if named, ok := types.Unalias(typ).(*types.Named); ok {
		if obj := named.Obj(); obj.Pkg() != nil {
			if inner, ok := noCopyTypes[obj.Pkg().Path()][obj.Name()]; ok {
				return append(slices.Clone(inner), typ.String())
			}
		}
	}
	for i := range styp.NumFields() {
		if sub := lockPath(styp.Field(i).Type(), seen); sub != nil {
			return append(sub, typ.String())
		}
	}
	return nil
}

var lockerType = func() *types.Interface {
	nullary := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	return types.NewInterfaceType([]*types.Func{
		types.NewFunc(token.NoPos, nil, "Lock", nullary),
		types.NewFunc(token.NoPos, nil, "Unlock", nullary),
	}, nil).Complete()
}()

// formatNode prints n as Go source, as x/tools does for its messages.
func formatNode(fset *token.FileSet, n ast.Node) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, n); err != nil {
		return ""
	}
	return b.String()
}
