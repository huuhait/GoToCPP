// gocpp: the names of Phase 10 syntax (../FORK.md, patches 4 and 5): a name
// with a ! (a must call or a must function: F!, os.ReadFile!, func f!) and
// a name with a # (a macro use: #Name). This file is new in the patches.

package parser

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/huuhait/gocpp/internal/syntax/gpast"
)

// checkName reports an error if name is a name with a ! or a # that the
// grammar doesn't allow at pos: must says that a name with a ! is allowed
// (a called function or method, a func declaration), macro that a name with
// a # is (the start of a macro call). Where it is allowed, checkMarkedNames
// checks later that it is called.
func (p *parser) checkName(pos token.Pos, name string, must, macro bool) {
	switch {
	case gpast.IsMust(name):
		if !must {
			p.error(pos, fmt.Sprintf("cannot use %s here: a name with ! can only be a called function or method, or a func declaration", name))
		}
		p.sawMarked = true
	case gpast.IsMacroUse(name):
		if !macro {
			p.error(pos, fmt.Sprintf("cannot use %s here: a name with # can only be the start of a macro call", name))
		}
		p.sawMarked = true
	}
}

// checkMarkedNames reports an error for each name with a ! or a # in the
// tree root that is not called. A name with a ! or a # is a call: its name
// (or a selector or an index expression that starts or ends with it, as in
// os.ReadFile!, #util.Dbg, parse![int]) must be the function of a call. A
// func declaration with a name with a ! is fine too.
func (p *parser) checkMarkedNames(root ast.Node) {
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		if id, ok := n.(*ast.Ident); ok && (gpast.IsMust(id.Name) || gpast.IsMacroUse(id.Name)) && !calledName(stack) {
			p.error(id.Pos(), fmt.Sprintf("%s must be called: write %s(...)", id.Name, id.Name))
		}
		return true
	})
}

// calledName reports whether the identifier at the top of the stack (the
// path from the root to the identifier) is a called name: a name with a ! is
// the called function, or the method (the end of the selector) that is
// called, perhaps with type arguments (os.ReadFile!(p), parse![int](s)); a
// name with a # is the start of the selector that is called, perhaps with
// type arguments (#util.Dbg(x), #Dbg[int](x)); and the name of a func or
// macro declaration is fine too. Nothing else may follow in the selector:
// F![int].x(1) calls x, not F!, and #a[int].b(1) is not a macro call.
func calledName(stack []ast.Node) bool {
	id, ok := stack[len(stack)-1].(*ast.Ident)
	if !ok {
		return false
	}
	var child ast.Node = id
	i := len(stack) - 2
	for ; i >= 0; i-- {
		sel, ok := stack[i].(*ast.SelectorExpr)
		if !ok {
			break
		}
		if gpast.IsMust(id.Name) {
			// os.ReadFile!: the name is the end of the selector, and a
			// selector has only one name at its end.
			if sel.Sel != id || child != ast.Node(id) {
				return false
			}
			child = sel
			i--
			break
		}
		// #util.Dbg: the name is the start of the selector.
		if sel.X != child || gpast.IsMust(sel.Sel.Name) {
			return false
		}
		child = sel
	}
	if i >= 0 {
		switch par := stack[i].(type) {
		case *ast.IndexExpr:
			if par.X == child {
				child = par
				i--
			}
		case *ast.IndexListExpr:
			if par.X == child {
				child = par
				i--
			}
		}
	}
	if i < 0 {
		return false
	}
	switch par := stack[i].(type) {
	case *ast.CallExpr:
		return par.Fun == child
	case *ast.FuncDecl:
		return par.Name == id && child == ast.Node(id)
	}
	return false
}

// parseLazyType parses lazy T, the type of a lazy parameter of a macro, at
// the word lazy: patch 6.
func (p *parser) parseLazyType() ast.Expr {
	pos := p.pos
	p.next() // lazy
	return gpast.NewLazy(pos, p.parseType())
}

// parseOnTopMacro parses a line of macros on top of a declaration, at its
// #: #Trace(), #util.Locked(&s.mu), #Derive(String). It is a call of a macro
// and the end of the line. Patch 6.
func (p *parser) parseOnTopMacro() *ast.GenDecl {
	pos := p.pos
	x := p.parseUnaryExpr() // not parseExpr: "#include <stdio.h>" must not go on to the next lines as a comparison
	call, ok := x.(*ast.CallExpr)
	if !ok || !macroCallee(call.Fun) || p.tok.Precedence() != token.LowestPrec { // a macro call followed by an operator is no line of macros
		p.error(pos, "expected a macro call on top of a declaration: #Name(...)")
		if !ok {
			call = &ast.CallExpr{Fun: x, Lparen: x.End(), Rparen: x.End()}
		}
		for p.tok != token.SEMICOLON && p.tok != token.EOF {
			p.next() // the rest of the line is not a macro call either: one error is enough
		}
	}
	p.expectSemi()
	return gpast.NewOnTop(pos, call)
}

// macroCallee reports whether fun, the function of a call, is a macro: a
// name with a #, perhaps with type arguments, or a selector on a name with
// a #.
func macroCallee(fun ast.Expr) bool {
	for {
		switch x := fun.(type) {
		case *ast.Ident:
			return gpast.IsMacroUse(x.Name)
		case *ast.SelectorExpr:
			fun = x.X
		case *ast.IndexExpr:
			fun = x.X
		case *ast.IndexListExpr:
			fun = x.X
		default:
			return false
		}
	}
}

// attachOnTop checks the lines of macros that were on top of d, and gives
// the doc comment that was above them to d. They must be on top of a func
// (not a macro) or a type declaration; d is nil at the end of the file. doc
// is the comment group above the first line of macros, or nil. noErrors says
// that nothing went wrong since the first line: after an error, the
// declaration that follows may be lost in the recovery, so there is no
// second error.
func (p *parser) attachOnTop(onTop []ast.Decl, doc *ast.CommentGroup, d ast.Decl, noErrors bool) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if !gpast.IsMacroDecl(d) {
			if doc != nil {
				d.Doc = doc
			}
			return
		}
	case *ast.GenDecl:
		if d.Tok == token.TYPE {
			if doc != nil {
				d.Doc = doc
			}
			return
		}
	case *ast.BadDecl:
		return // the error is reported already
	}
	if noErrors {
		p.error(onTop[0].Pos(), "macro calls must be on top of a func or type declaration")
	}
}

// parseMacroDecl parses a macro declaration at the word macro, which only
// parseDecl calls for a package-level declaration: macro Name(params)
// results { body }. It needs a name after the word; if there is none, the
// declaration is a BadDecl.
func (p *parser) parseMacroDecl() ast.Decl {
	if p.trace {
		defer un(trace(p, "MacroDecl"))
	}

	doc := p.leadComment
	pos := p.pos
	p.next() // macro
	if p.tok != token.IDENT {
		p.errorExpected(p.pos, "macro name")
		p.advance(declStart)
		return &ast.BadDecl{From: pos, To: p.pos}
	}
	return p.parseFuncDeclRest(doc, pos, true)
}
