package cppgen

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Where break and continue go (Phase 02 README §2.4).
//
// Each Go for, range and switch statement becomes one C++ for, while or switch, or a switch
// becomes an if-else chain (control.go). So C++'s own break and continue do what Go's do
// wherever they go to the innermost statement that C++ would pick: break to the innermost for,
// range or switch, continue to the innermost loop. The others are a goto:
//   - break L to a statement further out: goto gp_L_break, a label right after the statement;
//   - continue L to a loop further out: goto gp_L_continue, a label at the end of the loop's
//     body, which is then in a block of its own, so that the goto doesn't jump over the
//     initialization of a variable of the body (C++ forbids that);
//   - any continue of a 3-clause for whose post statement is not one C++ expression (i, j =
//     i+1, j-1) or needs temporaries (order.go): the post then comes at the end of the body,
//     after the same kind of label
//     (gp_L_continue, or gp_cont_N without a label), and a continue must not skip it.
//
// The generated labels start with gp_ and never end in _, so they can't be a user label
// (Escape gives those a _ suffix), and C++ labels have a name space of their own.

// A flow holds where the break and continue statements of one function go, found before any
// of it is written: a loop's body needs its extra block before its first line is written.
type flow struct {
	target map[*ast.BranchStmt]ast.Stmt // break or continue → its for, range or switch
	plain  map[*ast.BranchStmt]bool     // a C++ break or continue does it

	breakTo    map[ast.Stmt]bool // a C++ break goes to the switch: it can't be an if-else chain
	breakLabel map[ast.Stmt]bool // a goto goes to after the statement
	contLabel  map[ast.Stmt]bool // a goto goes to the end of the loop's body

	label map[ast.Stmt]*ast.Ident // the label of a labeled statement
	gotos map[*types.Label]bool   // the labels that a goto uses
}

// newFlow finds where the break and continue statements of body go. go/types checked them:
// each has a statement to go to, and a label names an enclosing one. inHead reports whether a
// 3-clause for writes its post statement in the C++ for's head.
func newFlow(info *types.Info, body *ast.BlockStmt, inHead func(post ast.Stmt) bool) *flow {
	f := &flow{
		target:     make(map[*ast.BranchStmt]ast.Stmt),
		plain:      make(map[*ast.BranchStmt]bool),
		breakTo:    make(map[ast.Stmt]bool),
		breakLabel: make(map[ast.Stmt]bool),
		contLabel:  make(map[ast.Stmt]bool),
		label:      make(map[ast.Stmt]*ast.Ident),
		gotos:      make(map[*types.Label]bool),
	}
	labeled := make(map[*types.Label]ast.Stmt)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false // a function of its own (Phase 03)
		case *ast.LabeledStmt:
			if l, ok := info.Defs[n.Label].(*types.Label); ok {
				labeled[l] = n.Stmt
				f.label[n.Stmt] = n.Label
			}
		}
		return true
	})

	var stack []ast.Node // the nodes that enclose the current one
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case nil:
			stack = stack[:len(stack)-1]
			return true
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			f.branch(info, n, stack, labeled)
		}
		stack = append(stack, n)
		return true
	})

	for b, t := range f.target {
		if loop, ok := t.(*ast.ForStmt); ok && b.Tok == token.CONTINUE && loop.Post != nil && !inHead(loop.Post) {
			f.plain[b] = false
		}
	}
	for b, t := range f.target {
		switch {
		case b.Tok == token.BREAK && f.plain[b]:
			f.breakTo[t] = true
		case b.Tok == token.BREAK:
			f.breakLabel[t] = true // a goto, which an if-else chain allows too
		case !f.plain[b]:
			f.contLabel[t] = true
		}
	}
	return f
}

// branch finds where the break, continue or goto b goes; stack holds the nodes around it.
func (f *flow) branch(info *types.Info, b *ast.BranchStmt, stack []ast.Node, labeled map[*types.Label]ast.Stmt) {
	var l *types.Label
	if b.Label != nil {
		l, _ = info.Uses[b.Label].(*types.Label)
	}
	switch b.Tok {
	case token.GOTO:
		f.gotos[l] = true
		return
	case token.FALLTHROUGH:
		return
	}
	// The innermost statement that C++'s own break or continue goes to.
	var inner ast.Stmt
	for i := len(stack) - 1; i >= 0 && inner == nil; i-- {
		switch s := stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			inner = s.(ast.Stmt)
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if b.Tok == token.BREAK {
				inner = s.(ast.Stmt)
			}
		}
	}
	t := inner
	if l != nil {
		t = labeled[l]
	}
	if t == nil {
		return // go/types reported it; branchStmt reports an internal error
	}
	f.target[b] = t
	f.plain[b] = t == inner
}

// labelName returns the name of the label of the labeled statement s for kind (break or
// continue): gp_L_break.
func (f *flow) labelName(s ast.Stmt, kind string) string {
	return "gp_" + f.label[s].Name + "_" + kind
}
