package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Statements (Phase 02 README §2.4). Each statement becomes cppir statements whose lines are
// mapped to the statement's .gp line. This file has the simple statements, declarations,
// assignments, return, blocks, labels and the branch statements; control.go has if, for and
// switch, and flow.go finds where each break and continue goes.
//
//	Go                       C++
//	x := e, var x = e        T x = e;  (an explicit type; only gp_ temporaries are auto)
//	v := v + 1 (shadowing)   T gp_t0 = v + 1; { T v = gp_t0; ...rest of the block... }
//	                         (v keeps its Go name: declLines)
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
// before it translates them, and puts the temporaries that this returns in front of its own
// C++, in the { } of a statement that is not a declaration. A condition (if, else if, loop,
// case) has its own shapes: fg.cond, the arms of chain and the loops (control.go).
//
// Building the tree: the statements go into the innermost open statement list (fg.add), the
// body of a block, loop, case or Scope that fg.push opened; fg.pop closes it and sets the
// place of its closing brace.

// A frame is an open statement list: the body of a node that is being built.
type frame struct {
	list *[]cppir.Stmt
	end  *cppir.Pos // where the node's closing brace goes, or nil
}

// add adds s to the innermost open statement list.
func (fg *funcGen) add(s cppir.Stmt) {
	f := fg.out[len(fg.out)-1]
	*f.list = append(*f.list, s)
	fg.fresh = false
}

// push opens list, the body of a node: the next statements go into it. end is the node's
// field for the place of its closing brace (nil if it has none), which pop sets.
func (fg *funcGen) push(list *[]cppir.Stmt, end *cppir.Pos) {
	fg.out = append(fg.out, frame{list, end})
}

// pop closes the innermost open statement list, whose closing brace is at end.
func (fg *funcGen) pop(end token.Pos) {
	f := fg.out[len(fg.out)-1]
	if f.end != nil {
		*f.end = fg.g.pos(end)
	}
	fg.out = fg.out[:len(fg.out)-1]
	fg.fresh = false
}

// stmts lowers the statements of a block, which ends at end: there it closes the blocks
// that the declarations of shadowing variables opened (declLines).
func (fg *funcGen) stmts(list []ast.Stmt, end token.Pos) {
	mark := fg.shadows
	for _, s := range list {
		fg.stmt(s)
	}
	fg.closeShadows(mark, end)
}

// closeShadows closes the Scopes that the declarations of shadowing variables opened since
// fg.shadows was mark, at end (the printer closes them on one line, } } }).
func (fg *funcGen) closeShadows(mark int, end token.Pos) {
	for ; fg.shadows > mark; fg.shadows-- {
		fg.names.closeScope()
		fg.pop(end)
	}
}

// initShadows returns the number of Scopes that the init statement of an if, for or switch
// opened for shadowing variables since fg.shadows was mark (declLines), and takes them over:
// the statement closes them where it closes its own block.
func (fg *funcGen) initShadows(mark int) int {
	n := fg.shadows - mark
	fg.shadows = mark
	return n
}

func (fg *funcGen) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.EmptyStmt:
	case *ast.BlockStmt:
		fg.block(s)
	case *ast.ExprStmt, *ast.IncDecStmt:
		pre, x := fg.simple(s)
		p := fg.g.pos(s.Pos())
		fg.add(withPre(p, pre, &cppir.ExprStmt{Pos: p, X: x}))
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

// block lowers a Go block to a C++ block, which is a scope in both languages.
func (fg *funcGen) block(b *ast.BlockStmt) {
	blk := &cppir.Block{Pos: fg.g.pos(b.Lbrace)}
	fg.add(blk)
	fg.inner(&blk.Stmts, &blk.End, b.Rbrace, func() { fg.stmts(b.List, b.Rbrace) })
}

// inner lowers what body adds into list, the body of a node whose closing brace (its field
// end) is at endPos, in a new C++ scope.
func (fg *funcGen) inner(list *[]cppir.Stmt, end *cppir.Pos, endPos token.Pos, body func()) {
	fg.push(list, end)
	fg.names.openScope()
	fg.fresh = true
	body()
	fg.names.closeScope()
	fg.pop(endPos)
}

// simple orders and translates a simple statement that is one C++ expression (simpleExpr): it
// returns the temporaries that go in front of it, and its C++ expression.
func (fg *funcGen) simple(s ast.Stmt) ([]cppir.Stmt, cppir.Expr) {
	fg.forceTarget(s)
	pre := fg.order(false, simpleScope(s)...)
	return pre, fg.simpleExpr(s)
}

// forceTarget makes the calls in the target of x op= y go into temporaries (fg.force): x op= y
// is x = x op y, which evaluates x twice (*f() += 1 must call f once).
func (fg *funcGen) forceTarget(s ast.Stmt) {
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok == token.ASSIGN || a.Tok == token.DEFINE || len(a.Lhs) != 1 {
		return
	}
	ast.Inspect(a.Lhs[0], func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok && !fg.g.info.Types[c.Fun].IsType() {
			fg.force[c] = true
		}
		return true
	})
}

// simpleScope returns the expressions that a simple statement that is one C++ expression
// evaluates, in any order: x op= y reads x too, and x = y evaluates the operands of its target
// (targetOperands) before y, as gc does.
func simpleScope(s ast.Stmt) []ast.Expr {
	switch s := s.(type) {
	case *ast.ExprStmt:
		return []ast.Expr{s.X}
	case *ast.IncDecStmt:
		return []ast.Expr{s.X}
	case *ast.AssignStmt:
		if s.Tok == token.ASSIGN {
			return append(targetOperands(s.Lhs), s.Rhs...)
		}
		return []ast.Expr{s.Lhs[0], s.Rhs[0]}
	}
	return nil
}

// targetOperands returns the operands of the targets of an assignment that are evaluated with
// its values (the spec's "operands of index expressions and pointer indirections on the left"):
// p of *p. (a[i] and p.f add theirs with their steps.) gc evaluates their calls before those
// of the values, in order, and reads them where the stores are.
func targetOperands(lhs []ast.Expr) []ast.Expr {
	var ops []ast.Expr
	for _, e := range lhs {
		if x, ok := ast.Unparen(e).(*ast.StarExpr); ok {
			ops = append(ops, x.X)
		}
	}
	return ops
}

// postInHead reports whether the post statement of a 3-clause for goes into the C++ for's
// head: when it is one C++ expression without temporaries.
func (fg *funcGen) postInHead(post ast.Stmt) bool {
	return oneExpr(post) && !fg.simpleNeedsPre(post)
}

// simpleNeedsPre reports whether simple(s) would return temporaries, without writing anything.
func (fg *funcGen) simpleNeedsPre(s ast.Stmt) bool {
	fg.forceTarget(s)
	return fg.needsPre(false, simpleScope(s)...)
}

// simpleExpr returns the C++ expression of a simple statement that is one: an expression
// statement, x++ or x--, and an assignment with one target and one value (x = e, _ = e,
// x op= e). oneExpr says which statements are. The statement's expressions were ordered
// (simple).
func (fg *funcGen) simpleExpr(s ast.Stmt) cppir.Expr {
	switch s := s.(type) {
	case *ast.ExprStmt:
		return fg.expr(s.X)
	case *ast.IncDecStmt:
		return &cppir.Postfix{X: fg.expr(s.X), Op: s.Tok.String()}
	case *ast.AssignStmt:
		if oneExpr(s) {
			lhs, rhs := s.Lhs[0], s.Rhs[0]
			if s.Tok == token.ASSIGN {
				var t target // _
				if !isBlank(lhs) {
					t = fg.lhs(lhs)
				}
				return t.assignExpr(fg.expr(rhs))
			}
			// x op= y is x = x op y: x is a variable (Phase 02 has no other targets),
			// so reading it twice is reading it once.
			op := s.Tok + (token.ADD - token.ADD_ASSIGN)
			return &cppir.Assign{Op: "=", X: fg.expr(lhs), Y: fg.binaryOp(s.TokPos, op, lhs, rhs)}
		}
	}
	fg.g.errorf(s.Pos(), "internal error: %s is not one C++ expression", nodeKind(s))
	return placeholder
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

// returnStmt lowers return. Several results are a std::tuple, which `return {a, b};` makes
// from its values left to right; a bare return in a function with named results returns them.
func (fg *funcGen) returnStmt(s *ast.ReturnStmt) {
	p := fg.g.pos(s.Pos())
	switch {
	case len(s.Results) == 0 && len(fg.fn.results) == 0:
		fg.add(&cppir.Return{Pos: p})
	case len(s.Results) == 0:
		res := fg.fn.obj.Signature().Results()
		names := make([]cppir.Expr, len(fg.fn.results))
		for i, n := range fg.fn.results {
			names[i] = cxxName(n)
			if fg.isBoxed(res.At(i)) {
				names[i] = &cppir.Paren{X: deref(names[i])}
			}
		}
		fg.add(&cppir.Return{Pos: p, X: braceList(names)})
	case len(s.Results) == 1:
		// One result, or a call whose results are the function's: return f();
		pre := fg.order(false, s.Results[0])
		fg.add(withPre(p, pre, &cppir.Return{Pos: p, X: fg.expr(s.Results[0])}))
	default:
		// C++ evaluates the values of a braced list in order.
		pre := fg.order(true, s.Results...)
		fg.add(withPre(p, pre, &cppir.Return{Pos: p, X: &cppir.InitList{Elems: fg.exprs(s.Results, false)}}))
	}
}

// braceList returns the values of a return: x, or {x, y}.
func braceList(values []cppir.Expr) cppir.Expr {
	if len(values) == 1 {
		return values[0]
	}
	return &cppir.InitList{Elems: values}
}

func (fg *funcGen) assignStmt(s *ast.AssignStmt) {
	switch {
	case s.Tok == token.DEFINE:
		fg.declare(s.Pos(), s.Lhs, s.Rhs)
	case oneExpr(s):
		pre, x := fg.simple(s)
		p := fg.g.pos(s.Pos())
		fg.add(withPre(p, pre, &cppir.ExprStmt{Pos: p, X: x}))
	default: // a, b = b, a and a, b = f()
		fg.bind(s.Pos(), nil, s.Lhs, s.Rhs, true)
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

// A target is the left side of one value in a declaration or an assignment. The zero target
// is _.
type target struct {
	x    cppir.Expr // the C++ of the left side, or nil for _
	typ  cppir.Type // the C++ type of a new variable (of its value if boxed), or nil (an existing one, _)
	name string     // the C++ name of the variable that x is (reads), or "" for _ or another target

	shadow bool // a new variable that shadows a variable or function of an outer scope
	class  bool // a new variable of a C++ class type (gp::string), which a temporary is moved into
	box    bool // a new variable that is boxed (box.go): gp::ptr<T> x = gp::new_<T>(value);

	// mem: storing to the target writes memory that other expressions can read: a boxed or a
	// package variable, or a target that is not a variable (*p).
	mem bool
	// ptr is the pointer operand p of a target *p, which bind evaluates into a temporary
	// first when an assignment before it can change it (gc's ascompatee).
	ptr ast.Expr
}

// varTarget returns the target of the existing variable whose C++ name is name.
func varTarget(name string) target {
	return target{x: cxxName(name), name: name}
}

// lhs returns the target of the left side e of an assignment, which is not _.
func (fg *funcGen) lhs(e ast.Expr) target {
	t := target{x: fg.expr(e), mem: true}
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		if v, ok := fg.g.info.Uses[x].(*types.Var); ok {
			if n, ok := fg.names.names[v]; ok {
				t.name = n
			} else {
				t.name = fg.g.names[v]
			}
			t.mem = isPkgVar(v) || fg.isBoxed(v)
		}
	case *ast.StarExpr:
		t.ptr = x.X
	}
	return t
}

// blank reports whether t is _.
func (t target) blank() bool { return t.x == nil }

// assign returns the C++ statement at pos that gives t the value: T x = value; for a new
// variable, x = value; for an existing one, and for _ a statement that only evaluates it.
func (t target) assign(pos cppir.Pos, value cppir.Expr) cppir.Stmt {
	if t.typ != nil && t.box {
		return varDecl(pos, ptrType(t.typ), t.name, newBox(t.typ, value))
	}
	if t.typ != nil {
		return varDecl(pos, t.typ, t.name, value)
	}
	return &cppir.ExprStmt{Pos: pos, X: t.assignExpr(value)}
}

// assignExpr returns the C++ expression that gives the existing target t (or _) the value.
func (t target) assignExpr(value cppir.Expr) cppir.Expr {
	if t.blank() {
		return voidCast(value)
	}
	return &cppir.Assign{Op: "=", X: t.x, Y: value}
}

// declare lowers a := or a var declaration of the names lhs with the values (none for var x
// T). For :=, some of the names may be variables that exist already: they are assigned.
//
// Each new variable gets an explicit C++ type: x := e is `int64_t x = e;`, and var x T is
// `T x{};`, the zero value. A new variable keeps its Go name also when it shadows another one
// (v := v): bind puts a value that reads the name into a temporary first, and the declaration
// starts a C++ block (declLines, funcNames).
//
// The temporaries of a declaration are not in { }, which would end the scope of its
// variables too: Go doesn't allow a goto over a declaration anyway.
func (fg *funcGen) declare(pos token.Pos, lhs, values []ast.Expr) {
	targets := fg.newTargets(lhs)
	if len(values) == 0 {
		p := fg.g.pos(pos)
		var lines []cppir.Stmt
		open := -1
		for _, t := range targets {
			if !t.blank() {
				if t.shadow && open < 0 {
					open = len(lines)
				}
				if t.box {
					lines = append(lines, varDecl(p, ptrType(t.typ), t.name, newBox(t.typ, nil)))
				} else {
					lines = append(lines, varDecl(p, t.typ, t.name, &cppir.InitList{}))
				}
			}
		}
		fg.declLines(pos, lines, open, false, targets)
		return
	}
	fg.bind(pos, targets, nil, values, false)
}

// declLines adds the statements of a declaration (joined: as one line), and puts its new
// variables into the scope of funcNames. open is the index of the statement that declares the
// first variable that shadows another one (fg.shadowing), or -1.
//
// A shadowing variable starts a C++ block of its own, a Scope, which ends where its Go block
// ends (stmts, or the if, for or switch of an init statement):
//
//	int64_t gp_t0 = v + 1; { int64_t v = gp_t0;
//	    ...the rest of the Go block...
//	}
//
// C++ debug info puts a variable into the scope of its whole block, also above its
// declaration. So without the block, a debugger would show the inner v, not yet set, on the
// lines of the block above the declaration and on the declaration's own line; with it, it
// shows the outer v there, and the inner one after the declaration. The statements of the
// declaration before the first shadowing variable (its temporaries) stay outside the block,
// on the same C++ line (a Line that ends with the Scope). A block that has nothing before the
// declaration (fg.fresh) needs no new block. The block is also what lets a loop body declare a
// variable of the loop's init or its range variable again, which C++ forbids in the outermost
// block of the body (control.go).
func (fg *funcGen) declLines(pos token.Pos, lines []cppir.Stmt, open int, joined bool, targets []target) {
	p := fg.g.pos(pos)
	switch {
	case open >= 0 && (open > 0 || !fg.fresh):
		sc := &cppir.Scope{Pos: p, Head: lines[open:]}
		if open > 0 {
			fg.add(&cppir.Line{Pos: p, Stmts: append(lines[:open:open], sc)})
		} else {
			fg.add(sc)
		}
		fg.push(&sc.Stmts, &sc.End)
		fg.names.openScope()
		fg.shadows++
	case joined:
		fg.add(&cppir.Line{Pos: p, Stmts: lines})
	default:
		for _, l := range lines {
			fg.add(l)
		}
	}
	for _, t := range targets {
		if t.shadow && !fg.names.enter(t.name) {
			fg.g.errorf(pos, "internal error: %s is declared twice in one C++ block", t.name)
		}
	}
}

// shadowing reports whether the new variable obj shadows a variable or a function of an outer
// scope (a local variable, a parameter, a result, a package variable or function), which a
// debugger would show by the same name.
func (fg *funcGen) shadowing(obj *types.Var) bool {
	s := obj.Parent()
	if s == nil || s.Parent() == nil {
		return false
	}
	_, outer := s.Parent().LookupParent(obj.Name(), obj.Pos())
	switch outer.(type) {
	case *types.Var, *types.Func:
		return true
	}
	return false
}

// newTargets returns the targets of a declaration of the names lhs, and chooses the C++ names
// of its new variables.
func (fg *funcGen) newTargets(lhs []ast.Expr) []target {
	targets := make([]target, len(lhs))
	for i, e := range lhs {
		id := e.(*ast.Ident)
		if id.Name == "_" {
			continue
		}
		if obj, ok := fg.g.info.Defs[id].(*types.Var); ok {
			t := target{typ: fg.g.cxxType(obj.Type(), id.Pos()), shadow: fg.shadowing(obj), box: fg.isBoxed(obj)}
			t.class = isClass(obj.Type())
			if t.shadow {
				t.name = fg.names.name(obj) // declLines puts it into its scope
			} else {
				t.name = fg.names.declare(obj)
			}
			t.x = cxxName(t.name)
			targets[i] = t
		} else {
			targets[i] = fg.lhs(id) // redeclared by :=
		}
	}
	return targets
}

// bind adds the statements that give the targets their values, with Go's order: all values
// are evaluated, from left to right, before any target changes. For an assignment, targets is
// nil and lhs are its left sides: their operands (targetOperands) are ordered with the values,
// and the targets are made after that. A target *p whose p a store before it can change
// (affected: p, *p = q, 5) gets p in a temporary first, so it stores through the old p, as gc
// does.
//   - One value with several results (a, b := f()): the result tuple goes into a temporary,
//     and each target gets its element.
//   - As many values as targets: in order, directly, if no value reads a target too early
//     (early): in C++ a new variable is in scope already in its own initializer and in those
//     after it (v := v, x, y := 2, x), and a target that exists can change before a later
//     value reads it (a, b = b, a). Otherwise the values go into temporaries first, which
//     read the old variables: int64_t gp_t0 = v + 1; { int64_t v = gp_t0; (the temporary of
//     a new variable has its type, and a class type is moved out of it; the others are auto).
//     None is needed for a constant, or for a value that has no effect (inert: no call, no
//     package variable, no operation that can panic) and reads no target too early: it is
//     evaluated in its own assignment. The value of _ is only evaluated. So a new variable
//     keeps its Go name (funcNames), and a shadowing one starts its block (declLines).
//
// The values are ordered first (order.go), as a list that C++ evaluates in order: the values
// that are evaluated before the assignments are in their order, and the others are inert.
// For an assignment (stmt), the statements are one C++ line, and the temporaries are in { }.
// A declaration whose values need temporaries in front is one line too, except the result
// tuple's (Phase 02 README §2.1).
func (fg *funcGen) bind(pos token.Pos, targets []target, lhs, values []ast.Expr, stmt bool) {
	p := fg.g.pos(pos)
	// The operands of the targets (*p) are evaluated first, but their reads are where the
	// stores are, after the values: then the list is not in C++'s order.
	ops := targetOperands(lhs)
	pre := fg.order(len(values) > 1 && ops == nil, append(ops, values...)...)
	if lhs != nil {
		// The targets of an assignment, translated after the order: their operands may be
		// in temporaries.
		targets = make([]target, len(lhs))
		for i, e := range lhs {
			if !isBlank(e) {
				targets[i] = fg.lhs(e)
			}
		}
	}
	var lines []cppir.Stmt
	temps := len(pre) > 0
	joined := len(pre) > 0
	open := -1 // the statement of the first shadowing variable (declLines)
	assign := func(t target, value cppir.Expr) {
		if t.shadow && open < 0 {
			open = len(lines)
		}
		lines = append(lines, t.assign(p, value))
	}
	// save evaluates the pointer of a target *p into a temporary first if an assignment
	// before it can change p (gc saves it with the values, before the stores).
	save := func(i int) {
		if t := targets[i]; t.ptr != nil && fg.affected(t.ptr, targets[:i]) {
			tmp := fg.names.temp("t")
			temps, joined = true, true
			lines = append(lines, varDecl(p, typeName("auto"), tmp, fg.expr(t.ptr)))
			targets[i].x = deref(cxxName(tmp))
		}
	}
	direct := len(values) == len(targets)
	for i, t := range targets {
		if !direct {
			break
		}
		if !t.blank() && t.typ == nil && len(targets) > 1 {
			direct = false // a value after an old target may read it, also in a call
		}
		if fg.reads(values[i], early(targets, i)) {
			direct = false
		}
	}
	switch {
	case len(values) == 1 && len(targets) > 1:
		value := fg.exactExpr(values[0])
		if !slices.ContainsFunc(targets, func(t target) bool { return !t.blank() }) {
			lines = append(lines, &cppir.ExprStmt{Pos: p, X: voidCast(value)})
			break
		}
		tmp := fg.names.temp("t")
		temps = true
		lines = append(lines, varDecl(p, typeName("auto"), tmp, value))
		for i := range targets {
			save(i)
		}
		for i, t := range targets {
			if !t.blank() {
				get := &cppir.Name{Name: "std::get", Args: []cppir.Node{lit(strconv.Itoa(i))}}
				assign(t, &cppir.Call{Fun: get, Args: []cppir.Expr{cxxName(tmp)}})
			}
		}
	case direct:
		for i, t := range targets {
			assign(t, fg.expr(values[i]))
		}
	default:
		// A value is evaluated in its assignment, after the targets before it changed,
		// if that can't make a difference: a constant, or a value without an effect that
		// doesn't read a target too early.
		tmps := make([]string, len(values))
		for i, v := range values {
			save(i)
			switch {
			case fg.constValue(v) != nil:
			case targets[i].blank():
				lines = append(lines, targets[i].assign(p, fg.expr(v)))
			case fg.inert(v) && !fg.reads(v, early(targets, i)):
			default:
				tmps[i] = fg.names.temp("t")
				temps, joined = true, true
				if typ := targets[i].typ; typ != nil {
					lines = append(lines, varDecl(p, typ, tmps[i], fg.expr(v)))
				} else {
					lines = append(lines, varDecl(p, typeName("auto"), tmps[i], fg.exactExpr(v)))
				}
			}
		}
		for i, t := range targets {
			switch {
			case t.blank():
			case tmps[i] != "" && t.typ != nil && t.class:
				assign(t, call("std::move", cxxName(tmps[i]))) // the temporary is not used again
			case tmps[i] != "":
				assign(t, cxxName(tmps[i]))
			default:
				assign(t, fg.expr(values[i]))
			}
		}
	}
	if open >= 0 {
		open += len(pre)
	}
	lines = append(pre, lines...)
	switch {
	case !stmt:
		fg.declLines(pos, lines, open, joined, targets)
	case temps:
		fg.add(&cppir.Block{Pos: p, Stmts: lines, Inline: true})
	default:
		fg.add(&cppir.Line{Pos: p, Stmts: lines})
	}
}

// early returns the targets that the value of targets[i] must not read when it is evaluated
// in its own assignment: the targets before it, which have changed or are declared by then,
// and targets[i] itself if it is a new variable, whose name C++ puts in scope in its own
// initializer.
func early(targets []target, i int) []target {
	if targets[i].typ != nil {
		return targets[:i+1]
	}
	return targets[:i]
}

// reads reports whether e reads one of the targets, by C++ name: a new variable is in scope
// in C++ under its name, also where Go means another variable of that name (v := v). A
// target that is not a variable (a[i] = ..., *p = ..., from Phase 03 on) counts as read by
// any e, so that its value goes into a temporary rather than in the wrong order.
func (fg *funcGen) reads(e ast.Expr, targets []target) bool {
	used := fg.usedNames([]ast.Expr{e})
	for _, t := range targets {
		switch {
		case t.blank():
		case t.name == "":
			return true
		case used[t.name]:
			return true
		}
	}
	return false
}

// affected reports whether an assignment to one of the targets (which come before the
// expression e in an assignment) can change the value of e, so that e must be evaluated before
// them (gc's ascompatee: affected): e reads one of the targets, or one of them writes memory
// (target.mem) and e reads memory (a boxed or a package variable, *p: !inert).
func (fg *funcGen) affected(e ast.Expr, targets []target) bool {
	used := fg.usedNames([]ast.Expr{e})
	mem := false
	for _, t := range targets {
		switch {
		case t.blank():
		case t.name != "" && used[t.name]:
			return true
		case t.mem:
			mem = true
		}
	}
	return mem && !fg.inert(e)
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

// labeledStmt lowers a labeled statement. The label itself is written only if a goto uses
// it: break L and continue L go to labels of their own (flow.go).
func (fg *funcGen) labeledStmt(s *ast.LabeledStmt) {
	if l, ok := fg.g.info.Defs[s.Label].(*types.Label); ok && fg.flow.gotos[l] {
		fg.add(&cppir.Label{Pos: fg.g.pos(s.Pos()), Name: Escape(s.Label.Name)})
	}
	fg.stmt(s.Stmt)
}

// branchStmt lowers break, continue, goto and fallthrough.
func (fg *funcGen) branchStmt(s *ast.BranchStmt) {
	p := fg.g.pos(s.Pos())
	switch s.Tok {
	case token.GOTO:
		fg.add(&cppir.Goto{Pos: p, Label: Escape(s.Label.Name)})
	case token.FALLTHROUGH:
		// The last non-empty statement of a case, possibly labeled: the C++ switch falls
		// into the next case (control.go keeps the cases in source order).
		fg.add(&cppir.Attr{Pos: p, Name: "fallthrough"})
	case token.BREAK, token.CONTINUE:
		target := fg.flow.target[s]
		switch {
		case target == nil:
			fg.g.errorf(s.Pos(), "internal error: no statement for %s", s.Tok)
		case fg.flow.plain[s] && s.Tok == token.BREAK:
			fg.add(&cppir.Break{Pos: p})
		case fg.flow.plain[s]:
			fg.add(&cppir.Continue{Pos: p})
		case s.Tok == token.BREAK:
			fg.add(&cppir.Goto{Pos: p, Label: fg.flow.labelName(target, "break")})
		default:
			fg.add(&cppir.Goto{Pos: p, Label: fg.contLabels[target]})
		}
	}
}
