package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"sync"
)

// Helpers of the rewrites of the ! forms (must.go, mustdrop.go,
// mustlower.go): a walk over the mustChildren of an AST node that can replace
// an expression, a check for terminating statements, and the nodes of the
// code that the rewrites write.

var (
	mustExprType = reflect.TypeFor[ast.Expr]()
	mustNodeType = reflect.TypeFor[ast.Node]()
)

// A mustFieldKind says what a field of an AST node holds, for mustChildren.
type mustFieldKind int

const (
	mustFieldExpr      mustFieldKind = iota // an ast.Expr
	mustFieldNode                           // another node (a statement, a block, a field list...)
	mustFieldExprSlice                      // a []ast.Expr
	mustFieldNodeSlice                      // a slice of nodes
)

type mustFieldPlan struct {
	index int
	kind  mustFieldKind
}

var mustFieldPlans sync.Map // reflect.Type (of a struct) -> []mustFieldPlan

// mustPlan returns the fields of the struct type t that hold nodes, in the
// order of the declaration, which is the order of the source for every
// node of go/ast: the evaluation order of the operands of an expression
// follows it.
func mustPlan(t reflect.Type) []mustFieldPlan {
	if p, ok := mustFieldPlans.Load(t); ok {
		return p.([]mustFieldPlan)
	}
	var plans []mustFieldPlan
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		ft := f.Type
		isNode := func(t reflect.Type) bool {
			return (t.Kind() == reflect.Interface || t.Kind() == reflect.Pointer) &&
				t.Implements(mustNodeType) && t != commentType
		}
		switch {
		case ft == mustExprType:
			plans = append(plans, mustFieldPlan{i, mustFieldExpr})
		case isNode(ft):
			plans = append(plans, mustFieldPlan{i, mustFieldNode})
		case ft.Kind() == reflect.Slice && ft.Elem() == mustExprType:
			plans = append(plans, mustFieldPlan{i, mustFieldExprSlice})
		case ft.Kind() == reflect.Slice && isNode(ft.Elem()):
			plans = append(plans, mustFieldPlan{i, mustFieldNodeSlice})
		}
	}
	mustFieldPlans.Store(t, plans)
	return plans
}

// mustChildren calls fn for each child node of n, in source order. When the
// child is an expression that is held in a variable of the type ast.Expr
// (a field of n or an element of a slice of n), slot points at that
// variable, so that fn can replace the expression; otherwise slot is nil.
func mustChildren(n ast.Node, fn func(child ast.Node, slot *ast.Expr)) {
	v := reflect.ValueOf(n)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}
	for _, p := range mustPlan(v.Type()) {
		fv := v.Field(p.index)
		switch p.kind {
		case mustFieldExpr:
			if !fv.IsNil() {
				slot := fv.Addr().Interface().(*ast.Expr)
				fn(*slot, slot)
			}
		case mustFieldNode:
			if !fv.IsNil() {
				fn(fv.Interface().(ast.Node), nil)
			}
		case mustFieldExprSlice:
			for i := range fv.Len() {
				slot := fv.Index(i).Addr().Interface().(*ast.Expr)
				if *slot != nil {
					fn(*slot, slot)
				}
			}
		case mustFieldNodeSlice:
			for i := range fv.Len() {
				if e := fv.Index(i); !e.IsNil() {
					fn(e.Interface().(ast.Node), nil)
				}
			}
		}
	}
}

// mustInspectOwn calls f for every node in the tree n that belongs to the
// function that n is part of: it does not go into function literals (they
// are functions of their own, with their own results). It stops going down
// into a node when f returns false.
func mustInspectOwn(n ast.Node, f func(ast.Node) bool) {
	ast.Inspect(n, func(c ast.Node) bool {
		if c == nil {
			return true
		}
		if _, ok := c.(*ast.FuncLit); ok && c != n {
			return false
		}
		return f(c)
	})
}

// The nodes that the rewrites write. They have no position (token.NoPos),
// so that no tool takes them for the code the user wrote, except where a
// position is needed for go/types to report an error at the right place
// (the helper calls, see must.go).

func mIdent(name string) *ast.Ident { return &ast.Ident{Name: name} }

func mInt(v string) *ast.BasicLit { return &ast.BasicLit{Kind: token.INT, Value: v} }

func mCall(fun ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{Fun: fun, Args: args}
}

func mNotNil(name string) ast.Expr {
	return &ast.BinaryExpr{X: mIdent(name), Op: token.NEQ, Y: mIdent("nil")}
}

func mBlock(list ...ast.Stmt) *ast.BlockStmt { return &ast.BlockStmt{List: list} }

func mDefine(lhs []ast.Expr, rhs ...ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: lhs, Tok: token.DEFINE, Rhs: rhs}
}

func mAssign(lhs, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: []ast.Expr{lhs}, Tok: token.ASSIGN, Rhs: []ast.Expr{rhs}}
}

func mIdents(names ...string) []ast.Expr {
	es := make([]ast.Expr, len(names))
	for i, n := range names {
		es[i] = mIdent(n)
	}
	return es
}

// mNot returns !e, with parentheses when e needs them.
func mNot(e ast.Expr) ast.Expr {
	switch e.(type) {
	case *ast.Ident, *ast.CallExpr, *ast.ParenExpr, *ast.SelectorExpr, *ast.IndexExpr:
	default:
		e = &ast.ParenExpr{X: e}
	}
	return &ast.UnaryExpr{Op: token.NOT, X: e}
}

// mustTerminates reports whether the statement s is a terminating statement
// (Go's spec, "Terminating statements"): a function whose list of
// statements ends with it needs no return.
func mustTerminates(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return s.Tok == token.GOTO
	case *ast.ExprStmt:
		c, ok := s.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := c.Fun.(*ast.Ident)
		return ok && id.Name == "panic"
	case *ast.BlockStmt:
		return mustTerminatesList(s.List)
	case *ast.IfStmt:
		return s.Else != nil && mustTerminatesList(s.Body.List) && mustTerminates(s.Else)
	case *ast.ForStmt:
		return s.Cond == nil && !mustHasBreak(s.Body, "", true)
	case *ast.RangeStmt:
		return false
	case *ast.LabeledStmt:
		return mustTerminatesLabeled(s.Stmt, s.Label.Name)
	case *ast.SwitchStmt:
		return mustTerminatesSwitch(s.Body, "")
	case *ast.TypeSwitchStmt:
		return mustTerminatesSwitch(s.Body, "")
	case *ast.SelectStmt:
		if mustHasBreak(s.Body, "", true) {
			return false
		}
		for _, c := range s.Body.List {
			if !mustTerminatesList(c.(*ast.CommClause).Body) {
				return false
			}
		}
		return true
	}
	return false
}

func mustTerminatesLabeled(s ast.Stmt, label string) bool {
	switch s := s.(type) {
	case *ast.ForStmt:
		return s.Cond == nil && !mustHasBreak(s.Body, "", true) && !mustHasBreak(s.Body, label, false)
	case *ast.SwitchStmt:
		return mustTerminatesSwitch(s.Body, label)
	case *ast.TypeSwitchStmt:
		return mustTerminatesSwitch(s.Body, label)
	case *ast.SelectStmt:
		if mustHasBreak(s.Body, "", true) || mustHasBreak(s.Body, label, false) {
			return false
		}
		for _, c := range s.Body.List {
			if !mustTerminatesList(c.(*ast.CommClause).Body) {
				return false
			}
		}
		return true
	}
	return mustTerminates(s)
}

func mustTerminatesList(list []ast.Stmt) bool {
	for i := len(list) - 1; i >= 0; i-- {
		if _, ok := list[i].(*ast.EmptyStmt); ok {
			continue
		}
		return mustTerminates(list[i])
	}
	return false
}

func mustTerminatesSwitch(body *ast.BlockStmt, label string) bool {
	hasDefault := false
	for _, c := range body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil {
			hasDefault = true
		}
		if mustHasBreak(&ast.BlockStmt{List: cc.Body}, "", true) || label != "" && mustHasBreak(&ast.BlockStmt{List: cc.Body}, label, false) {
			return false
		}
		if n := len(cc.Body); n > 0 {
			if b, ok := cc.Body[n-1].(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
				continue
			}
		}
		if !mustTerminatesList(cc.Body) {
			return false
		}
	}
	return hasDefault
}

// mustHasBreak reports whether the statements in n contain a break that leaves
// the statement that holds them: with implicit, an unlabeled break that is
// not inside a nested for, switch or select; else a break with the label.
func mustHasBreak(n ast.Node, label string, implicit bool) bool {
	found := false
	var walk func(n ast.Node, nested bool)
	walk = func(n ast.Node, nested bool) {
		ast.Inspect(n, func(c ast.Node) bool {
			if found || c == nil || c == n {
				return !found
			}
			switch c := c.(type) {
			case *ast.FuncLit:
				return false
			case *ast.BranchStmt:
				if c.Tok == token.BREAK {
					if c.Label == nil && implicit && !nested || c.Label != nil && !implicit && c.Label.Name == label {
						found = true
					}
				}
			case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				walk(c, true)
				return false
			}
			return true
		})
	}
	walk(n, false)
	return found
}

// mustIsError reports whether t is exactly the type error.
func mustIsError(t types.Type) bool {
	return types.Identical(t, mustErrorType)
}

var mustErrorType = types.Universe.Lookup("error").Type()
