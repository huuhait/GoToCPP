package lower

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Local closures (step 15.2, ADR-114): the func literals that cannot outlive the statement or the
// block that makes them.
//
// A func literal is a C++ lambda that captures each variable by copy, a variable that it changes
// is boxed so that the closure and the function share it, and the func value is a gp::func that
// owns the lambda (closures.go). That is what a closure needs when it can be stored, returned, sent
// or run by another goroutine. A closure that is only called, right where it is made, needs none
// of it: it can capture by reference, the variables stay on the C++ stack, and the call is a direct
// call of the lambda that the compiler can inline. Two shapes are local:
//
//	func() { ... }()     an immediately called literal (not the call of a go or defer statement:
//	                     it runs later, and a goroutine runs beside the function)
//	f := func() { ... } a literal bound to a new variable f whose every use is a call f(...) that
//	f()                  is not the call of a go or defer statement, and that is not in another
//	                     func literal or in the body of a range over a func (they can run later);
//	                     f is never assigned, address-taken, compared or passed
//
// A literal that calls recover is not local (its func value is aware: aware.hpp). The variables
// that only local literals use are captured by reference when they are declared by a statement of a
// block or by the header of a for or range loop (blockLevelVars: not a parameter or a result), so
// they are not boxed; one that a local literal changes stays "shared memory" for the order of
// evaluation and the lending of arguments: it is boxRefCaptured. A variable that anything else
// captures is boxed as before, and the local literal then captures its box.
//
// The result is decided by the use of the literal and of f alone, not by what the literal does, so
// it needs no summary of any callee.

// A localClosures is the func literals of a function that are local, each with the variable that
// holds it (nil for a literal that is called at once).
type localClosures map[*ast.FuncLit]*types.Var

// findLocalClosures returns the local func literals of the function declaration d, or nil for a
// function that is not analysed: generic, a method of a generic type, or without a body.
func findLocalClosures(info *tinfo, d *ast.FuncDecl) localClosures {
	if d == nil || d.Body == nil || d.Type.TypeParams != nil {
		return nil
	}
	if d.Recv != nil && len(d.Recv.List) == 1 {
		t := info.TypeOf(d.Recv.List[0].Type)
		if p, ok := types.Unalias(t).(*types.Pointer); ok {
			t = p.Elem()
		}
		named, _ := types.Unalias(t).(*types.Named)
		if named == nil || named.TypeParams().Len() > 0 {
			return nil
		}
	}
	var out localClosures
	add := func(lit *ast.FuncLit, v *types.Var) {
		if out == nil {
			out = make(localClosures)
		}
		out[lit] = v
	}
	// The candidates, and the places where each variable is used.
	holders := make(map[*types.Var]*ast.FuncLit)
	var stack []ast.Node
	uses := make(map[*types.Var]bool) // the variables with a use that disqualifies them
	ast.Inspect(d.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch x := n.(type) {
		case *ast.CallExpr:
			lit, ok := ast.Unparen(x.Fun).(*ast.FuncLit)
			if ok && !isGoOrDeferCall(stack, len(stack)-1) && !callsRecover(info, lit) {
				add(lit, nil)
			}
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE || len(x.Lhs) != 1 || len(x.Rhs) != 1 {
				break
			}
			lit, ok := ast.Unparen(x.Rhs[0]).(*ast.FuncLit)
			id, isIdent := x.Lhs[0].(*ast.Ident)
			if !ok || !isIdent || id.Name == "_" || callsRecover(info, lit) {
				break
			}
			if v, ok := info.Defs[id].(*types.Var); ok {
				holders[v] = lit
			}
		case *ast.Ident:
			v, ok := info.Uses[x].(*types.Var)
			if !ok || holders[v] == nil {
				break
			}
			if !localCall(info, stack, v) {
				uses[v] = true
			}
		}
		return true
	})
	for v, lit := range holders {
		if !uses[v] {
			add(lit, v)
		}
	}
	return out
}

// isGoOrDeferCall reports whether the node at stack[k] is the call of a go or defer statement.
func isGoOrDeferCall(stack []ast.Node, k int) bool {
	if k == 0 {
		return false
	}
	switch s := stack[k-1].(type) {
	case *ast.GoStmt:
		return s.Call == stack[k]
	case *ast.DeferStmt:
		return s.Call == stack[k]
	}
	return false
}

// localCall reports whether the use of v at the top of the stack is the callee of a call that
// runs at once: not the call of a go or defer statement, not inside a func literal that v is not
// declared in, and not in the body of a range over a func.
func localCall(info *tinfo, stack []ast.Node, v *types.Var) bool {
	k := len(stack) - 1
	// The callee, possibly in parentheses.
	for k > 0 {
		if _, ok := stack[k-1].(*ast.ParenExpr); ok {
			k--
			continue
		}
		break
	}
	if k == 0 {
		return false
	}
	call, ok := stack[k-1].(*ast.CallExpr)
	if !ok || ast.Unparen(call.Fun) != ast.Unparen(stack[len(stack)-1].(ast.Expr)) {
		return false
	}
	if isGoOrDeferCall(stack, k-1) {
		return false
	}
	for _, n := range stack[:k-1] {
		switch x := n.(type) {
		case *ast.FuncLit:
			if !(x.Pos() <= v.Pos() && v.Pos() < x.End()) {
				return false // another literal: it may run later
			}
		case *ast.RangeStmt:
			use := stack[len(stack)-1].Pos()
			if isRangeFunc(info, x) && x.Body.Pos() <= use && use < x.Body.End() {
				return false // the body of a range over a func is a closure too
			}
		}
	}
	return true
}

// callsRecover reports whether the func literal lit calls the builtin recover (in its own code or
// in a literal inside it).
func callsRecover(info *tinfo, lit *ast.FuncLit) bool {
	found := false
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if b, ok := info.Uses[id].(*types.Builtin); ok && b.Name() == "recover" {
				found = true
			}
		}
		return !found
	})
	return found
}

// blockLevelVars returns the variables that a statement of a block declares (x := ..., var x ...)
// in the body, and the variables of the header of a for or range statement: not a parameter, a
// result, or a variable of the header of an if, switch or select statement.
func blockLevelVars(info *tinfo, body ast.Node) map[*types.Var]bool {
	out := make(map[*types.Var]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		var list []ast.Stmt
		switch n := n.(type) {
		case *ast.ForStmt:
			// The variable of a loop header is a variable per iteration in Go (the boxing copies
			// it into a new box for each one). A local closure runs within its iteration and
			// keeps nothing, so one plain variable shows it the same values.
			if as, ok := n.Init.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
				list = []ast.Stmt{as}
			}
		case *ast.RangeStmt:
			if n.Tok == token.DEFINE && !isRangeFunc(info, n) { // (a yield parameter otherwise)
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if id, ok := e.(*ast.Ident); ok {
						if v, ok := info.Defs[id].(*types.Var); ok {
							out[v] = true
						}
					}
				}
			}
		case *ast.BlockStmt:
			list = n.List
		case *ast.CaseClause:
			list = n.Body
		case *ast.CommClause:
			list = n.Body
		default:
			return true
		}
		for _, s := range list {
			switch s := s.(type) {
			case *ast.AssignStmt:
				if s.Tok == token.DEFINE {
					for _, l := range s.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							if v, ok := info.Defs[id].(*types.Var); ok {
								out[v] = true
							}
						}
					}
				}
			case *ast.DeclStmt:
				if gd, ok := s.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
					for _, spec := range gd.Specs {
						for _, id := range spec.(*ast.ValueSpec).Names {
							if v, ok := info.Defs[id].(*types.Var); ok {
								out[v] = true
							}
						}
					}
				}
			}
		}
		return true
	})
	return out
}

// closureNotes records the decision for each func literal of the function that captures a
// variable: it does not escape (a local closure) or it escapes to heap (a gp::func that owns it).
func (w *escWalker) closureNotes() {
	g := w.g
	ast.Inspect(w.d.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.FuncLit)
		if !ok || !capturesVariable(g.info, lit) {
			return true
		}
		if _, local := g.localLits[lit]; local {
			w.note(lit.Pos(), "func literal does not escape", "")
			return true
		}
		w.note(lit.Pos(), "func literal escapes to heap",
			"it is not only called where it is made: it is stored, returned, passed, compared, "+
				"assigned, run by go or defer, used by another func literal, or calls recover")
		return true
	})
}

// capturesVariable reports whether the func literal lit uses a variable of the function around it.
func capturesVariable(info *tinfo, lit *ast.FuncLit) bool {
	found := false
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			v, ok := info.Uses[id].(*types.Var)
			if ok && !v.IsField() && !isPkgVar(v) && !inside(lit, v.Pos()) {
				found = true
			}
		}
		return !found
	})
	return found
}
