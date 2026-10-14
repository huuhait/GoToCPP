package cppgen

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"
)

// Statements (Phase 02 README §2.4). Each statement is written as C++ lines mapped to the
// statement's .gp line. This file has the simple statements, declarations, assignments,
// return, blocks, labels and the branch statements; control.go has if, for and switch, and
// flow.go finds where each break and continue goes.
//
//	Go                       C++
//	x := e, var x = e        T x = e;  (an explicit type; only gp_ temporaries are auto)
//	g(x, f())                { auto gp_t0 = f(); g(x, gp_t0); }  (Go's order, order.go)
//	var x T                  T x{};  (the zero value)
//	a, b := f()              auto gp_t0 = f(); T a = std::get<0>(gp_t0); ...
//	x = e                    x = e;
//	a, b = b, a              { auto gp_t0 = b; auto gp_t1 = a; a = gp_t0; b = gp_t1; }
//	x op= y                  x = x op y, through binaryOp: x = gp::div(x, y), x = uint8_t(x + y)
//	x++, x--                 x++; x--;  (C++ converts a small integer back with Go's wrapping)
//	return a, b              return {a, b};
//	L: s                     L:; s  (only for a label that a goto uses)
//	goto L                   goto L;
//	fallthrough              [[fallthrough]];
//	break, continue          the same, or a goto to a label (flow.go)
//
// goto: Go forbids a goto over a variable declaration and into a block, and C++ forbids a
// jump over the initialization of a variable that is still in scope at the label. So the
// temporaries of a statement that is not a declaration are inside { } (a, b = b, a above;
// if, for and switch put theirs in an extra block too), and a goto that Go allows never jumps
// over a C++ initialization.
//
// The evaluation order (order.go): each statement orders its expressions (fg.order) right
// before it translates them, and writes the temporaries that this returns in front of its
// lines, in the { } of a statement that is not a declaration. A condition (if, else if, loop,
// case) has its own shapes: fg.cond, the arms of chain and the loops (control.go).

// stmts writes the statements of a block.
func (fg *funcGen) stmts(list []ast.Stmt) {
	for _, s := range list {
		fg.stmt(s)
	}
}

func (fg *funcGen) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.EmptyStmt:
	case *ast.BlockStmt:
		fg.block(s)
	case *ast.ExprStmt, *ast.IncDecStmt:
		pre, text := fg.simple(s)
		fg.line(s.Pos(), withPre(pre, text+";"))
	case *ast.ReturnStmt:
		fg.returnStmt(s)
	case *ast.AssignStmt:
		fg.assignStmt(s)
	case *ast.DeclStmt:
		fg.declStmt(s)
	case *ast.LabeledStmt:
		fg.labeledStmt(s)
	case *ast.BranchStmt:
		fg.branchStmt(s)
	case *ast.IfStmt:
		fg.ifStmt(s)
	case *ast.ForStmt:
		fg.forStmt(s)
	case *ast.RangeStmt:
		fg.rangeStmt(s)
	case *ast.SwitchStmt:
		fg.switchStmt(s)
	default:
		fg.g.unsupported(s.Pos(), nodeKind(s))
	}
}

// line writes one line of C++ mapped to the .gp line of pos.
func (fg *funcGen) line(pos token.Pos, text string) {
	fg.g.w.line(fg.g.pos(pos), text)
}

// block writes a Go block as a C++ block, which is a scope in both languages.
func (fg *funcGen) block(b *ast.BlockStmt) {
	fg.line(b.Lbrace, "{")
	fg.inner(func() { fg.stmts(b.List) })
	fg.line(b.Rbrace, "}")
}

// inner writes what body writes one level deeper, in a new C++ scope.
func (fg *funcGen) inner(body func()) {
	fg.g.w.indent()
	fg.names.openScope()
	body()
	fg.names.closeScope()
	fg.g.w.dedent()
}

// simple orders and translates a simple statement that is one C++ expression (simpleExpr): it
// returns the temporaries that go in front of it, and its C++ expression.
func (fg *funcGen) simple(s ast.Stmt) ([]string, string) {
	pre := fg.order(false, simpleScope(s)...)
	return pre, fg.simpleExpr(s)
}

// simpleScope returns the expressions that a simple statement that is one C++ expression
// evaluates, in any order: x op= y reads x too.
func simpleScope(s ast.Stmt) []ast.Expr {
	switch s := s.(type) {
	case *ast.ExprStmt:
		return []ast.Expr{s.X}
	case *ast.IncDecStmt:
		return []ast.Expr{s.X}
	case *ast.AssignStmt:
		if s.Tok == token.ASSIGN {
			return s.Rhs
		}
		return []ast.Expr{s.Lhs[0], s.Rhs[0]}
	}
	return nil
}

// postInHead reports whether the post statement of a 3-clause for goes into the C++ for's
// head: when it is one C++ expression without temporaries.
func (fg *funcGen) postInHead(post ast.Stmt) bool {
	return oneExpr(post) && !fg.needsPre(false, simpleScope(post)...)
}

// simpleExpr returns the C++ expression of a simple statement that is one: an expression
// statement, x++ or x--, and an assignment with one target and one value (x = e, _ = e,
// x op= e). oneExpr says which statements are. The statement's expressions were ordered
// (simple).
func (fg *funcGen) simpleExpr(s ast.Stmt) string {
	switch s := s.(type) {
	case *ast.ExprStmt:
		return fg.expr(s.X).text
	case *ast.IncDecStmt:
		return fg.expr(s.X).at(precPostfix) + s.Tok.String()
	case *ast.AssignStmt:
		if oneExpr(s) {
			lhs, rhs := s.Lhs[0], s.Rhs[0]
			if s.Tok == token.ASSIGN {
				var t target
				if !isBlank(lhs) {
					t.name = fg.expr(lhs).text
				}
				return t.assignExpr(fg.expr(rhs).text)
			}
			// x op= y is x = x op y: x is a variable (Phase 02 has no other targets),
			// so reading it twice is reading it once.
			op := s.Tok + (token.ADD - token.ADD_ASSIGN)
			return fg.expr(lhs).text + " = " + fg.binaryOp(s.TokPos, op, lhs, rhs).text
		}
	}
	fg.g.errorf(s.Pos(), "internal error: %s is not one C++ expression", nodeKind(s))
	return placeholder.text
}

// oneExpr reports whether the simple statement s is one C++ expression (simpleExpr), which
// can be the post statement of a C++ for.
func oneExpr(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ExprStmt, *ast.IncDecStmt:
		return true
	case *ast.AssignStmt:
		return s.Tok != token.DEFINE && len(s.Lhs) == 1 && len(s.Rhs) == 1
	}
	return false
}

// returnStmt writes return. Several results are a std::tuple, which `return {a, b};` makes
// from its values left to right; a bare return in a function with named results returns them.
func (fg *funcGen) returnStmt(s *ast.ReturnStmt) {
	switch {
	case len(s.Results) == 0 && len(fg.fn.results) == 0:
		fg.line(s.Pos(), "return;")
	case len(s.Results) == 0:
		fg.line(s.Pos(), "return "+braceList(fg.fn.results)+";")
	case len(s.Results) == 1:
		// One result, or a call whose results are the function's: return f();
		pre := fg.order(false, s.Results[0])
		fg.line(s.Pos(), withPre(pre, "return "+fg.expr(s.Results[0]).text+";"))
	default:
		// C++ evaluates the values of a braced list in order.
		pre := fg.order(true, s.Results...)
		fg.line(s.Pos(), withPre(pre, "return {"+fg.exprs(s.Results, false)+"};"))
	}
}

// braceList returns the names as the value of a return: x, or {x, y}.
func braceList(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return "{" + strings.Join(names, ", ") + "}"
}

func (fg *funcGen) assignStmt(s *ast.AssignStmt) {
	switch {
	case s.Tok == token.DEFINE:
		fg.declare(s.Pos(), s.Lhs, s.Rhs)
	case oneExpr(s):
		pre, text := fg.simple(s)
		fg.line(s.Pos(), withPre(pre, text+";"))
	default: // a, b = b, a and a, b = f()
		targets := make([]target, len(s.Lhs))
		for i, e := range s.Lhs {
			if !isBlank(e) {
				targets[i].name = fg.expr(e).text
			}
		}
		fg.bind(s.Pos(), targets, s.Rhs, true)
	}
}

func (fg *funcGen) declStmt(s *ast.DeclStmt) {
	d := s.Decl.(*ast.GenDecl)
	switch d.Tok {
	case token.CONST:
		// Every use of a constant is folded: nothing to write.
	case token.VAR:
		for _, spec := range d.Specs {
			spec := spec.(*ast.ValueSpec)
			lhs := make([]ast.Expr, len(spec.Names))
			for i, id := range spec.Names {
				lhs[i] = id
			}
			fg.declare(spec.Pos(), lhs, spec.Values)
		}
	default:
		fg.g.unsupported(s.Pos(), "local type declarations (Phase 03)")
	}
}

// isBlank reports whether e is the blank identifier _.
func isBlank(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}

// A target is the left side of one value in a declaration or an assignment.
type target struct {
	name string // the C++ name, or "" for _
	typ  string // the C++ type of a new variable, or "" for an existing one
}

// assign returns the C++ statement that gives t the value: T x = value; for a new variable,
// x = value; for an existing one, and for _ a statement that only evaluates it.
func (t target) assign(value string) string {
	if t.typ != "" && t.name != "" {
		return t.typ + " " + t.name + " = " + value + ";"
	}
	return t.assignExpr(value) + ";"
}

// assignExpr returns the C++ expression that gives the existing variable t (or _) the value.
func (t target) assignExpr(value string) string {
	if t.name == "" {
		return "static_cast<void>(" + value + ")"
	}
	return t.name + " = " + value
}

// declare writes a := or a var declaration of the names lhs with the values (none for var x T).
// For :=, some of the names may be variables that exist already: they are assigned.
//
// Each new variable gets an explicit C++ type: x := e is `int64_t x = e;`, and var x T is
// `T x{};`, the zero value. A new variable whose name the values use (v := v) gets a new C++
// name, because in C++ the new name is in scope in its own initializer (funcNames.declare).
//
// The temporaries of a declaration are not in { }, which would end the scope of its
// variables too: Go doesn't allow a goto over a declaration anyway.
func (fg *funcGen) declare(pos token.Pos, lhs, values []ast.Expr) {
	targets := fg.newTargets(lhs, values)
	if len(values) == 0 {
		for _, t := range targets {
			if t.name != "" {
				fg.line(pos, t.typ+" "+t.name+"{};")
			}
		}
		return
	}
	fg.bind(pos, targets, values, false)
}

// newTargets returns the targets of a declaration of the names lhs with the values, and
// chooses the C++ names of its new variables.
func (fg *funcGen) newTargets(lhs, values []ast.Expr) []target {
	used := fg.usedNames(values)
	targets := make([]target, len(lhs))
	for i, e := range lhs {
		id := e.(*ast.Ident)
		if id.Name == "_" {
			continue
		}
		if obj, ok := fg.g.info.Defs[id].(*types.Var); ok {
			targets[i] = target{name: fg.names.declare(obj, used), typ: fg.g.cxxType(obj.Type(), id.Pos())}
		} else {
			targets[i] = target{name: fg.expr(id).text} // redeclared by :=
		}
	}
	return targets
}

// bind writes the lines that give the targets their values, with Go's order: all values are
// evaluated, from left to right, before any target changes.
//   - One value with several results (a, b := f()): the result tuple goes into a temporary,
//     and each target gets its element.
//   - As many values as targets: in order, directly, if every target is new (or _): a value
//     can't use a variable that the same statement declares, which then comes into scope only
//     in C++ (declare renames it). If some targets exist already, the values go into
//     temporaries first, because a value can use an old target. None is needed for a
//     constant, or for a value that has no effect (inert: no call, no package variable, no
//     operation that can panic) and reads none of the targets before it (reads): it is
//     evaluated in its own assignment. The value of _ is only evaluated.
//
// The values are ordered first (order.go), as a list that C++ evaluates in order: the values
// that are evaluated before the assignments are in their order, and the others are inert.
// For an assignment (stmt), the lines are one C++ line, and the temporaries are in { }. A
// declaration whose values need temporaries in front is one line too.
func (fg *funcGen) bind(pos token.Pos, targets []target, values []ast.Expr, stmt bool) {
	pre := fg.order(len(values) > 1, values...)
	var lines []string
	temps := len(pre) > 0
	direct := true
	for _, t := range targets {
		if t.name != "" && t.typ == "" {
			direct = false
		}
	}
	switch {
	case len(values) == 1 && len(targets) > 1:
		value := fg.exactExpr(values[0]).text
		if !slices.ContainsFunc(targets, func(t target) bool { return t.name != "" }) {
			lines = append(lines, "static_cast<void>("+value+");")
			break
		}
		tmp := fg.names.temp("t")
		temps = true
		lines = append(lines, "auto "+tmp+" = "+value+";")
		for i, t := range targets {
			if t.name != "" {
				lines = append(lines, t.assign("std::get<"+strconv.Itoa(i)+">("+tmp+")"))
			}
		}
	case direct || len(targets) == 1:
		for i, t := range targets {
			lines = append(lines, t.assign(fg.expr(values[i]).text))
		}
	default:
		// A value is evaluated in its assignment, after the targets before it changed,
		// if that can't make a difference: a constant, or a value without an effect that
		// doesn't read those targets.
		tmps := make([]string, len(values))
		for i, v := range values {
			switch {
			case fg.constValue(v) != nil:
			case targets[i].name == "":
				lines = append(lines, targets[i].assign(fg.expr(v).text))
			case fg.inert(v) && !fg.reads(v, targets[:i]):
			default:
				tmps[i] = fg.names.temp("t")
				temps = true
				lines = append(lines, "auto "+tmps[i]+" = "+fg.exactExpr(v).text+";")
			}
		}
		for i, t := range targets {
			switch {
			case t.name == "":
			case tmps[i] != "":
				lines = append(lines, t.assign(tmps[i]))
			default:
				lines = append(lines, t.assign(fg.expr(values[i]).text))
			}
		}
	}
	lines = append(pre, lines...)
	switch {
	case !stmt && len(pre) > 0:
		fg.line(pos, strings.Join(lines, " "))
	case !stmt:
		for _, l := range lines {
			fg.line(pos, l)
		}
	case temps:
		fg.line(pos, "{ "+strings.Join(lines, " ")+" }")
	default:
		fg.line(pos, strings.Join(lines, " "))
	}
}

// reads reports whether e reads one of the targets.
func (fg *funcGen) reads(e ast.Expr, targets []target) bool {
	used := fg.usedNames([]ast.Expr{e})
	for _, t := range targets {
		if t.name != "" && used[t.name] {
			return true
		}
	}
	return false
}

// usedNames returns the C++ names of the variables and functions that the expressions use,
// apart from those in temporaries (order.go), which are evaluated already.
func (fg *funcGen) usedNames(list []ast.Expr) map[string]bool {
	used := make(map[string]bool)
	for _, e := range list {
		ast.Inspect(e, func(n ast.Node) bool {
			if e, ok := n.(ast.Expr); ok && fg.temps[e] != "" {
				return false
			}
			if id, ok := n.(*ast.Ident); ok {
				switch obj := fg.g.info.Uses[id].(type) {
				case *types.Var, *types.Func:
					if name, ok := fg.names.names[obj]; ok {
						used[name] = true
					} else if name, ok := fg.g.names[obj]; ok {
						used[name] = true
					}
				}
			}
			return true
		})
	}
	return used
}

// labeledStmt writes a labeled statement. The label itself is written only if a goto uses
// it: break L and continue L go to labels of their own (flow.go). It is a label of an empty
// statement, L:;, because C++20 doesn't allow a label at the end of a block.
func (fg *funcGen) labeledStmt(s *ast.LabeledStmt) {
	if l, ok := fg.g.info.Defs[s.Label].(*types.Label); ok && fg.flow.gotos[l] {
		fg.line(s.Pos(), Escape(s.Label.Name)+":;")
	}
	fg.stmt(s.Stmt)
}

// branchStmt writes break, continue, goto and fallthrough.
func (fg *funcGen) branchStmt(s *ast.BranchStmt) {
	text := ""
	switch s.Tok {
	case token.GOTO:
		text = "goto " + Escape(s.Label.Name) + ";"
	case token.FALLTHROUGH:
		// The last non-empty statement of a case, possibly labeled: the C++ switch falls
		// into the next case (control.go keeps the cases in source order).
		text = "[[fallthrough]];"
	case token.BREAK, token.CONTINUE:
		target := fg.flow.target[s]
		switch {
		case target == nil:
			fg.g.errorf(s.Pos(), "internal error: no statement for %s", s.Tok)
		case fg.flow.plain[s]:
			text = s.Tok.String() + ";"
		case s.Tok == token.BREAK:
			text = "goto " + fg.flow.labelName(target, "break") + ";"
		default:
			text = "goto " + fg.contLabels[target] + ";"
		}
	}
	fg.line(s.Pos(), text)
}
