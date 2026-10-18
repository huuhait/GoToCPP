package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

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
//	type T struct{...}       nothing here: the struct is at namespace scope (typedecl.go)
//	a, b := f()              auto gp_t0 = f(); T a = std::get<0>(gp_t0); ...
//	x = e                    x = e;
//	a, b = b, a              { auto gp_t0 = b; auto gp_t1 = a; a = gp_t0; b = gp_t1; }
//	x op= y                  x = x op y, through binaryOp: x = gp::div(x, y), x = uint8_t(x + y)
//	s[i] = v, s[i] op= v     the same, on the element (a T&); s = append(s, x) moves s (slices.go)
//	s, s[0] = t, 5           { auto gp_t0 = s; s = t; gp_t0[0] = 5; } (the old s: saveOperands)
//	a, a[0] = b, 5           a = b; a[0] = 5; (an array: the new a; its index is saved, arrays.go)
//	m[k] = v, m[k] op= v     gp::set(m, k, v);  gp::ref(m, k) += v; and the other forms (maps.go)
//	p, p.f = q, 5            { auto gp_t0 = p; p = q; gp_t0->f = 5; } (the old p: structs.go)
//	n = n.kids[0]            n = Node(n.kids[0]); (a copy first: sharedCopy, structs.go)
//	x++, x--                 x++; x--;  (C++ converts a small integer back with Go's wrapping)
//	return a, b              return {a, b};  (with defers: gp_r0 = a; gp_r1 = b; goto gp_ret0;)
//	defer f(x)               gp_df.gp_push(f, x);  (defer.go)
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
	case *ast.DeferStmt:
		fg.deferStmt(s) // defer.go
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
	list, rank := fg.simpleScope(s)
	sc := &orderScope{rank: rank, copy: fg.simpleKeys(s)}
	if a, ok := s.(*ast.AssignStmt); ok && a.Tok != token.ASSIGN && a.Tok != token.DEFINE && len(a.Lhs) == 1 {
		fg.opTarget = ast.Unparen(a.Lhs[0])
	}
	pre := fg.orderWith(sc, list...)
	fg.opTarget = nil
	if c := fg.appendInPlace(s); c != nil {
		fg.ordered[c] = true // its arguments are in the statement's scope
	}
	pre = append(pre, fg.mapPre(s)...) // m[k] = append(m[k], x), m[k] <<= n (maps.go)
	return pre, fg.simpleExpr(s)
}

// forceTarget makes the calls in the target of x op= y go into temporaries (fg.force): x op= y
// is x = x op y, which evaluates x twice (*f() += 1 must call f once).
func (fg *funcGen) forceTarget(s ast.Stmt) {
	fg.markStores(s) // maps.go
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok == token.ASSIGN || a.Tok == token.DEFINE || len(a.Lhs) != 1 {
		return
	}
	if fg.g.pkg.Opaque.IsOpaque(a.Lhs[0]) {
		return // C++'s x op= y evaluates x once (simpleExpr)
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
// (targetOperands) before y, as gc does. In s = append(s, ...) (appendInPlace), the append is
// not a unit of its own: its arguments are in the statement's scope. rank says when Go evaluates
// the panicking parts of each expression (orderScope): in x = y, the target's operands after y
// except the parts that gc copies first (storeRank: s[a[i]] = t[j] checks j before i, s[a[i]&1] =
// t[j] i before j), in x op= y and x++ x's copied parts, then the rest of x, then y (opRank), and
// in m[k] op= y with an op other than / and %, m[k] after y (gc's safeMapRHS copies y first).
func (fg *funcGen) simpleScope(s ast.Stmt) (list []ast.Expr, rank func(*orderItem) int) {
	switch s := s.(type) {
	case *ast.ExprStmt:
		return []ast.Expr{s.X}, nil
	case *ast.IncDecStmt:
		return []ast.Expr{s.X}, fg.opRank(s.X)
	case *ast.AssignStmt:
		if c := fg.appendInPlace(s); c != nil {
			ops := fg.targetOperands(s.Lhs)
			return append(ops, c.Args...), fg.storeRank(s.Lhs[0], len(ops))
		}
		if fg.mapAppend(s) != nil {
			return nil, nil // m[k] = append(m[k], ...): ordered by mapPre (maps.go)
		}
		if s.Tok == token.ASSIGN {
			ops := fg.targetOperands(s.Lhs)
			return append(ops, s.Rhs...), fg.storeRank(s.Lhs[0], len(ops))
		}
		if fg.mapIndex(s.Lhs[0]) != nil && s.Tok != token.QUO_ASSIGN && s.Tok != token.REM_ASSIGN {
			return []ast.Expr{s.Lhs[0], s.Rhs[0]}, rootRank(1)
		}
		return []ast.Expr{s.Lhs[0], s.Rhs[0]}, fg.opRank(s.Lhs[0])
	}
	return nil, nil
}

// targetOperands returns the operands of the targets of an assignment that are evaluated with
// its values (the spec's "operands of index expressions and pointer indirections (including
// implicit pointer indirections in selectors) on the left"): p of *p, s and i of s[i] (step
// 3.5), p of p.f and those of the place of x in x.f (step 3.8, structs.go). gc evaluates their
// calls before those of the values, in order, and reads them where the stores are.
func (fg *funcGen) targetOperands(lhs []ast.Expr) []ast.Expr {
	var ops []ast.Expr
	for _, e := range lhs {
		if _, ok := ast.Unparen(e).(*ast.Ident); !ok && fg.g.pkg.Opaque.IsOpaque(e) {
			// A place of a C/C++ value (v[i], x.m, *p): the target as a whole, so that order.go
			// sees what its parts are to it (an object whose member it is: objectOf).
			ops = append(ops, e)
			continue
		}
		switch x := ast.Unparen(e).(type) {
		case *ast.StarExpr:
			ops = append(ops, x.X)
		case *ast.IndexExpr:
			ops = append(ops, x.X, x.Index)
		case *ast.SelectorExpr:
			ops = append(ops, fg.selectorOperands(x)...)
		}
	}
	return ops
}

// simpleKeys returns the copy function (orderScope) of the simple statement s (simpleScope):
// the key of a map entry that x = v stores to, which gc may copy first (orderKey). (In m[k] op= v
// and m[k]++, collectMapIndex sees it.)
func (fg *funcGen) simpleKeys(s ast.Stmt) func(int, ast.Expr) bool {
	if a, ok := s.(*ast.AssignStmt); ok && a.Tok == token.ASSIGN && fg.mapAppend(s) == nil {
		return fg.orderKeys(a.Lhs)
	}
	return nil
}

// storeRank returns the rank function (orderScope) of x = v, whose target x has its n operands
// first in the list: an entry of a map waits for v as a whole (safeMapRHS copies v first), another
// target only with the parts that gc doesn't copy first (targetCopies: walkAssign's safeExpr
// copies them, then evaluates v, then the place).
func (fg *funcGen) storeRank(x ast.Expr, n int) func(*orderItem) int {
	if fg.mapIndex(x) != nil {
		return rootRank(n)
	}
	copies := fg.targetCopies(x)
	return func(it *orderItem) int {
		if it.root < n && !inCopies(it, copies) {
			return 1
		}
		return 0
	}
}

// opRank returns the rank function (orderScope) of x op= y and x++, with x at the index 0 of the
// list (nil for a map entry, which maps.go orders): gc rewrites them as x = x op y after its
// safeExpr copied the parts of x (targetCopies), so those come first, then the rest of x, then y.
func (fg *funcGen) opRank(x ast.Expr) func(*orderItem) int {
	copies := fg.targetCopies(x)
	if fg.mapIndex(x) != nil || len(copies) == 0 {
		return nil
	}
	return func(it *orderItem) int {
		switch {
		case it.root > 0:
			return 2
		case inCopies(it, copies):
			return 0
		}
		return 1
	}
}

// postInHead reports whether the post statement of a 3-clause for goes into the C++ for's
// head: when it is one C++ expression without temporaries.
func (fg *funcGen) postInHead(post ast.Stmt) bool {
	return oneExpr(post) && !fg.simpleNeedsPre(post)
}

// simpleNeedsPre reports whether simple(s) would return temporaries, without writing anything.
func (fg *funcGen) simpleNeedsPre(s ast.Stmt) bool {
	fg.forceTarget(s)
	list, rank := fg.simpleScope(s)
	return fg.analyzeWith(&orderScope{rank: rank, copy: fg.simpleKeys(s)}, list).pre || fg.mapNeedsPre(s)
}

// simpleExpr returns the C++ expression of a simple statement that is one: an expression
// statement, x++ or x--, and an assignment with one target and one value (x = e, _ = e,
// x op= e). oneExpr says which statements are. The statement's expressions were ordered
// (simple).
func (fg *funcGen) simpleExpr(s ast.Stmt) cppir.Expr {
	if x := fg.mapSimple(s); x != nil {
		return x // a map entry changed by op=, ++ or append (maps.go)
	}
	switch s := s.(type) {
	case *ast.ExprStmt:
		return fg.expr(s.X)
	case *ast.IncDecStmt:
		if fg.g.pkg.Opaque.IsOpaque(s.X) {
			// C++'s ++it: an iterator's ++ without the copy of it++ (the same for a number).
			return &cppir.Unary{Op: s.Tok.String(), X: fg.expr(s.X)}
		}
		return &cppir.Postfix{X: fg.expr(s.X), Op: s.Tok.String()}
	case *ast.AssignStmt:
		fg.noteStores(s)
		if oneExpr(s) {
			lhs, rhs := s.Lhs[0], s.Rhs[0]
			if s.Tok != token.ASSIGN && (fg.g.pkg.Opaque.IsOpaque(lhs) || fg.g.pkg.Opaque.IsOpaque(rhs)) {
				// C++'s x op= y on a C/C++ value (a class's own operator+=: s += "!"), which
				// evaluates x once; x &^= y is x &= ~y.
				op, not := cAssignOp(s.Tok)
				y := fg.cValueOf(rhs)
				if not {
					y = &cppir.Unary{Op: "~", X: y}
				}
				return &cppir.Assign{Op: op, X: fg.expr(lhs), Y: y}
			}
			if s.Tok == token.ASSIGN {
				var t target // _
				if !isBlank(lhs) {
					t = fg.lhs(lhs)
					fg.markMoves(lhs, rhs) // s = append(s, x)
				}
				return t.assignExpr(fg.storedExpr(rhs))
			}
			// x op= y is x = x op y, which evaluates x twice: a variable, *p or s[i]. The
			// calls in x went into temporaries (forceTarget), so reading it twice is reading
			// it once: nothing between the two reads can change p, s or i (gc's safeExpr
			// rewrites it the same way), and the bounds check of s[i] runs twice with the
			// same result.
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
	fg.retStmt = s // returnMove
	defer func() { fg.retStmt = nil }()
	if fg.fn.defers != nil {
		fg.deferReturn(s) // defer.go: the results, then the exit
		return
	}
	p := fg.g.pos(s.Pos())
	switch {
	case len(s.Results) == 0 && len(fg.fn.results) == 0:
		fg.add(&cppir.Return{Pos: p})
	case len(s.Results) == 0:
		res := fg.fn.sig.Results()
		names := make([]cppir.Expr, len(fg.fn.results))
		for i, n := range fg.fn.results {
			names[i] = fg.resultValue(res.At(i), n)
		}
		fg.add(&cppir.Return{Pos: p, X: braceList(names)})
	case len(s.Results) == 1:
		// One result, or a call whose results are the function's: return f();
		pre := fg.order(false, s.Results[0])
		if r := fg.tupleReturn(p, s.Results[0]); r != nil {
			fg.add(withPre(p, append(pre, r[:len(r)-1]...), r[len(r)-1]))
			break
		}
		fg.add(withPre(p, pre, &cppir.Return{Pos: p, X: fg.returnMove(s.Results[0], fg.expr(s.Results[0]), false)}))
	default:
		// C++ evaluates the values of a braced list in order.
		pre := fg.order(true, s.Results...)
		values := fg.exprs(s.Results, false)
		for i, e := range s.Results {
			values[i] = fg.returnMove(e, values[i], true)
		}
		fg.add(withPre(p, pre, &cppir.Return{Pos: p, X: &cppir.InitList{Elems: values}}))
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
	fg.noteStores(s)
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
	case token.TYPE:
		// Moved to the namespace (typedecl.go): nothing to write here.
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
	x      cppir.Expr // the C++ of the left side, or nil for _
	typ    cppir.Type // the C++ type of a new variable (of its value if boxed), or nil (an existing one, _)
	name   string     // the C++ name of the variable that x is (reads), or "" for _ or another target
	shadow bool       // a new variable that shadows a variable or function of an outer scope
	class  bool       // a new variable of a C++ class type (gp::string), which a temporary is moved into
	box    bool       // a new variable that is boxed (box.go): gp::ptr<T> x = gp::new_<T>(value);
	ret    bool       // a result of a function with defers, stored by a return statement (deferReturn)
	large  bool       // a boxed new variable larger than 128 KiB, built in its box (largeBox, arrays.go)
	cbox   bool       // a boxed new variable that holds a C/C++ object, built in its box (cbox.go)
	hold   cppir.Stmt // after its declaration: the frame holds its box (cbox.go: holdBox), or nil

	// ptr is the pointer operand p of a target *p, index a target s[i] and field a target x.f
	// (step 3.8), whose operands (p; s and i; those of x.f: selectorOperands) bind evaluates into
	// temporaries first when an assignment before it can change them (gc's ascompatee:
	// saveOperands).
	ptr   ast.Expr
	index *ast.IndexExpr
	// mapX and mapK are the C++ of the map and the key of a target m[k] (an index too), which
	// stores with gp::set(m, k, value) (maps.go).
	mapX, mapK cppir.Expr
	field      *ast.SelectorExpr
	// keep is set for a store of a gocpp pointer into a member of a struct that a gocpp pointer
	// owns (n.next = head): gp::cgo::store_keep keeps the object alive (cunsafe.go: keepOf).
	keep *keepInfo

	// gtype is the Go type of the target (nil for _), which a value of another type converts to
	// (bind: one value with several results, step 3.9).
	gtype types.Type
	// src is the left side of an assignment (lhs), nil for a new variable.
	src ast.Expr

	// What a store to the target changes, for gc's ascompatee (affected): the local variable
	// gcVar (the variable itself, or the struct or array value that the target is a part of),
	// memory that other expressions can read (gcMem: a package or a boxed variable, *p, s[i],
	// m[k], p.f), or a result of a function with defers (gcDefer: a later panic shows it to the
	// deferred calls).
	gcVar   *types.Var
	gcMem   bool
	gcDefer bool
}

// varTarget returns the target of the existing variable whose C++ name is name.
func varTarget(name string) target {
	return target{x: cxxName(name), name: name}
}

// lhs returns the target of the left side e of an assignment, which is not _.
func (fg *funcGen) lhs(e ast.Expr) target {
	t := target{x: fg.expr(e), gtype: fg.g.info.TypeOf(e), src: e}
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		if v, ok := fg.g.info.Uses[x].(*types.Var); ok {
			if n, ok := fg.names.names[v]; ok {
				t.name = n
			} else {
				t.name = fg.g.names[v]
			}
		}
	case *ast.StarExpr:
		t.ptr = x.X
	case *ast.IndexExpr:
		t.index = x
		if isMap(fg.g.info.TypeOf(x.X)) {
			fg.mapTarget(&t, x)
		}
	case *ast.SelectorExpr:
		t.field = x
	}
	fg.gcTarget(&t, e)
	if rhs, ok := fg.memberStores[ast.Unparen(e)]; ok {
		t.keep = fg.keepOf(e, rhs)
	}
	return t
}

// gcTarget sets what a store to the target t, the left side e, changes, as gc's ascompatee
// sees it (target.gcVar): it goes through the fields of struct values and the elements of
// arrays to the variable they are in; a variable that is not local or is boxed, and any other
// place, is memory.
func (fg *funcGen) gcTarget(t *target, e ast.Expr) {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.IndexExpr:
			if isArray(fg.g.info.TypeOf(x.X)) {
				e = x.X
				continue
			}
		case *ast.SelectorExpr:
			if s := fg.g.info.Selections[x]; s != nil && s.Kind() == types.FieldVal && !s.Indirect() {
				e = x.X
				continue
			}
		case *ast.Ident:
			if v, ok := fg.g.info.Uses[x].(*types.Var); ok {
				fg.gcVarTarget(t, v)
				return
			}
		}
		t.gcMem = true
		return
	}
}

// gcVarTarget sets what a store to the variable v, the target t, changes (gcTarget).
func (fg *funcGen) gcVarTarget(t *target, v *types.Var) {
	switch {
	case isPkgVar(v) || fg.isBoxed(v):
		t.gcMem = true
	case fg.fn != nil && fg.fn.defers != nil && fg.isResult(v):
		t.gcDefer = true
	default:
		t.gcVar = v
	}
}

// isResult reports whether v is a result of the function.
func (fg *funcGen) isResult(v *types.Var) bool {
	res := fg.fn.sig.Results()
	for i := range res.Len() {
		if res.At(i) == v {
			return true
		}
	}
	return false
}

// saveOperands evaluates the operands of the target targets[i] into temporaries (the
// statements it returns, at pos) if an assignment to one of the targets before it can change
// them, and makes the target use them: in p, *p = q, 5 the store goes through the old p, in
// s, s[0] = t, 5 into the old s, and in i, s[i] = 1, 2 to the old i, as in Go, whose
// assignments evaluate the operands of their targets before any store (gc's ascompatee).
func (fg *funcGen) saveOperands(pos cppir.Pos, targets []target, i int) []cppir.Stmt {
	t := targets[i]
	var lines []cppir.Stmt
	save := func(e ast.Expr, x cppir.Expr) cppir.Expr {
		if !fg.affected(e, targets[:i]) {
			return x
		}
		tmp := fg.names.temp("t")
		lines = append(lines, varDecl(pos, typeName("auto"), tmp, fg.exactExpr(e)))
		fg.temps[ast.Unparen(e)] = tmp // evaluated now (bind's targetCopies skips it)
		return cxxName(tmp)
	}
	// objectSaved reports a place of a C/C++ value (x[k], x.m) whose object x an assignment
	// before it changes (p, p.value = q, 5; also p.next, p.next.value = ...): x would have to be
	// evaluated first, but gocpp can't tell a pointer, which a copy keeps, from an object, which
	// a copy loses. (A change through another pointer follows C++'s rules.)
	opaque := fg.g.pkg.Opaque.IsOpaque
	objectSaved := func(x ast.Expr) bool {
		xs := fg.g.srcText(x)
		for _, b := range targets[:i] {
			if b.src == nil || b.blank() {
				continue
			}
			if bs := fg.g.srcText(b.src); xs == bs || strings.HasPrefix(xs, bs+".") || strings.HasPrefix(xs, bs+"[") {
				fg.g.unsupported(x.Pos(), "an assignment that changes "+bs+" and stores into a member or an element of "+xs+": write two assignments")
				return true
			}
		}
		return false
	}
	switch {
	case t.ptr != nil && opaque(t.ptr):
		// *p of a C/C++ pointer or iterator: a copy of p keeps the place (gp::cgo::star).
		if p := save(t.ptr, nil); p != nil {
			targets[i].x = call("gp::cgo::star", p)
		}
	case t.index != nil && opaque(t.index):
		if !objectSaved(t.index.X) {
			if idx := save(t.index.Index, nil); idx != nil {
				targets[i].x = &cppir.Index{X: fg.expr(t.index.X), Index: idx}
			}
		}
	case t.field != nil && opaque(t.field) && fg.g.info.Selections[t.field] == nil:
		objectSaved(t.field.X) // a member of a C/C++ value
	case t.ptr != nil:
		if p := save(t.ptr, nil); p != nil {
			targets[i].x = deref(p)
		}
	case t.index != nil && fg.inArray(t.index):
		// An element of an array: the operands of the array's place, never the array itself.
		if x := fg.saveArrayElem(t.index, func(e ast.Expr) cppir.Expr { return save(e, nil) }); x != nil {
			targets[i].x = x // arrays.go
		}
	case t.field != nil && fg.embeddedPointer(t.field):
		// A promoted field through an embedded pointer (o.F of an embedded *E): Go evaluates
		// that pointer before the stores, so it goes into a temporary when a store before it can
		// change it (o.E, o.F = e, 5 stores into the old *o.E; structs.go).
		if fg.affected(t.field, targets[:i]) {
			p, typ, rest := fg.embeddedPtr(t.field)
			tmp := fg.names.temp("t")
			lines = append(lines, varDecl(pos, typeName("auto"), tmp, p))
			targets[i].x, _ = fg.fieldSteps(cxxName(tmp), typ, rest, t.field.Pos())
		}
	case t.field != nil:
		// A field: the pointer p of p.f, or the operands of the struct's place.
		if x := fg.saveField(t.field, func(e ast.Expr) cppir.Expr { return save(e, nil) }); x != nil {
			targets[i].x = x // structs.go
		}
	case t.index != nil:
		x, idx := save(t.index.X, nil), save(t.index.Index, nil)
		if x == nil && idx == nil {
			break
		}
		if x == nil {
			x = fg.expr(t.index.X)
		}
		if t.mapX != nil {
			if idx == nil {
				idx = fg.expr(t.index.Index)
			}
			targets[i].mapX, targets[i].mapK = x, idx
			break
		}
		if idx == nil {
			idx = fg.intExpr(t.index.Index)
		}
		targets[i].x = &cppir.Index{X: x, Index: idx}
	}
	return lines
}

// blank reports whether t is _.
func (t target) blank() bool { return t.x == nil }

// assign returns the C++ statement at pos that gives t the value: T x = value; for a new
// variable, x = value; for an existing one, and for _ a statement that only evaluates it.
func (t target) assign(pos cppir.Pos, value cppir.Expr) cppir.Stmt {
	if t.typ != nil && t.cbox {
		return withHold(pos, cBoxDecl(pos, t.typ, t.name, value), t.hold)
	}
	if t.typ != nil && t.box && t.large {
		return varDecl(pos, ptrType(t.typ), t.name, largeBox(t.typ, value))
	}
	if t.typ != nil && t.box {
		return varDecl(pos, ptrType(t.typ), t.name, newBox(t.typ, value))
	}
	if t.typ != nil {
		return varDecl(pos, t.typ, t.name, sameTypeInit(t.typ, value))
	}
	return &cppir.ExprStmt{Pos: pos, X: t.assignExpr(value)}
}

// assignExpr returns the C++ expression that gives the existing target t (or _) the value.
func (t target) assignExpr(value cppir.Expr) cppir.Expr {
	if t.blank() {
		return voidCast(value)
	}
	if t.mapX != nil {
		return call("gp::set", t.mapX, t.mapK, value)
	}
	if t.keep != nil {
		args := []cppir.Expr{t.keep.owner, t.x, value}
		if len(t.keep.path) > 0 {
			args = append(args, templateCall("gp::cgo::keep_path", t.keep.path))
		}
		return call("gp::cgo::store_keep", args...)
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
				if t.cbox {
					lines = append(lines, withHold(p, cBoxDecl(p, t.typ, t.name, nil), t.hold))
				} else if t.box {
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
	fg.holdCopies(pos, values)
}

// holdCopies makes the frame hold the pointer n for the rest of its block when a variable is declared
// as a copy of the C struct that n owns (m := *n): the copy's pointer members are raw, and what they
// point to is kept alive only by n's struct. (A copy that lives longer than the block, in a field or
// a slice, is the program's to keep valid: Limits in docs/spec/interop.md.)
func (fg *funcGen) holdCopies(pos token.Pos, values []ast.Expr) {
	for _, v := range values {
		star, ok := ast.Unparen(v).(*ast.StarExpr)
		if !ok || !fg.g.pkg.Opaque.IsOpaque(star.X) || !fg.g.mayOwn(star.X) || hasCall(fg.g.info, star.X) {
			continue
		}
		p := fg.g.pos(pos)
		fg.add(varDecl(p, typeName("[[maybe_unused]] const auto"), fg.names.temp("hold"), fg.expr(star.X)))
	}
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
			t := target{typ: fg.g.varType(obj, id.Pos()), shadow: fg.shadowing(obj), box: fg.isBoxed(obj), gtype: obj.Type()}
			t.cbox = t.box && cObject(obj.Type())
			if t.box && !t.cbox && cppir.TypeString(t.typ) == "auto" {
				t.typ = fg.boxedAutoType(obj, id)
			}
			t.large = t.box && !t.cbox && largeVar(obj.Type())
			t.class = isClass(obj.Type()) && complete(obj.Type()) // (a C/C++ value may be auto)
			if fg.loopInit {
				// A variable of a 3-clause for that is boxed (its address is taken in the
				// loop) is a per-iteration copy in gc; the init stores to a plain local.
				t.gcVar = obj
			} else {
				fg.gcVarTarget(&t, obj)
			}
			if t.shadow {
				t.name = fg.names.name(obj) // declLines puts it into its scope
			} else {
				t.name = fg.names.declare(obj)
			}
			t.x = cxxName(t.name)
			t.hold = fg.holdBox(obj, t.name)
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
//
// The operations that can panic follow gc's ascompatee (walk/assign.go), which decides which
// of two panics is reported: for each target in order, the operands of the target and then the
// value are evaluated early, before all stores, if a store before them can change them
// (affected: also every value after a store to a result of a function with defers, or after a
// store to memory a value that reads memory); the others are evaluated late, each in its own
// store, after the stores before it (the value first, then the target's place and its check).
// So a, b := s[i], t[j] checks i first, but x, y = s[i], t[j] with a package variable x checks
// j first. The order pass leaves those operations to bind (keepPanics); the parts of one value
// are ordered where it is evaluated (valuePanics).
// A declaration whose values need temporaries in front is one line too, except the result
// tuple's (Phase 02 README §2.1).
func (fg *funcGen) bind(pos token.Pos, targets []target, lhs, values []ast.Expr, stmt bool) {
	p := fg.g.pos(pos)
	// The operands of the targets (*p) are evaluated first, but their reads are where the
	// stores are, after the values: then the list is not in C++'s order.
	ops := fg.targetOperands(lhs)
	pre := fg.orderWith(&orderScope{seq: len(values) > 1 && ops == nil, keepPanics: true, hoistRoots: len(ops), copy: fg.orderKeys(lhs)}, append(ops, values...)...)
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
	// save evaluates the operands of a target *p or s[i] into temporaries first if an
	// assignment before it can change them (gc saves them with the values, before the stores).
	save := func(i int) {
		if saved := fg.saveOperands(p, targets, i); len(saved) > 0 {
			temps, joined = true, true
			lines = append(lines, saved...)
		}
	}
	// copies evaluates the parts of the target lhs[i] that gc copies before a late store's value
	// (targetCopies) and that can panic into temporaries, where that store is, and makes the
	// target read them. (The calls in them are in temporaries already: hoistRoots.)
	copies := func(i int) {
		if lhs == nil || isBlank(lhs[i]) || targets[i].mapX != nil {
			return
		}
		copied := false
		for _, e := range fg.targetCopies(lhs[i]) {
			if fg.panicky(e) {
				temps, joined, copied = true, true, true
				lines = append(lines, fg.valuePanics(e)...)
				tmp := fg.names.temp("t")
				lines = append(lines, varDecl(p, typeName("auto"), tmp, fg.exactExpr(e)))
				fg.temps[ast.Unparen(e)] = tmp
			}
		}
		if copied {
			targets[i] = fg.lhs(lhs[i]) // with the temporaries (also saveOperands')
		}
	}
	// panics orders the operations of the value v that can panic, where it is evaluated.
	panics := func(v ast.Expr) {
		if checks := fg.valuePanics(v); len(checks) > 0 {
			temps, joined = true, true
			lines = append(lines, checks...)
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
		if fg.reads(values[i], early(targets, i)) || fg.affected(values[i], targets[:i]) && !fg.inert(values[i]) {
			direct = false // gc evaluates it before the stores (bind's panics)
		}
	}
	switch {
	case len(values) == 1 && len(targets) > 1:
		if ix := fg.mapIndex(values[0]); ix != nil && fg.g.pkg.Opaque.IsOpaque(ix) {
			fg.commaOK(ix) // v, ok := m[k] of a map of C/C++ values: go/types has no tuple for it
		}
		value := fg.exactExpr(values[0])
		if _, isCall := ast.Unparen(values[0]).(*ast.CallExpr); isCall && fg.g.pkg.Opaque.IsOpaque(values[0]) {
			// a, b := std.minmax(x, y): a C++ pair or tuple (Phase 04 README §2.5), whose
			// members gp::values copies, so that references into temporaries can't dangle:
			// auto gp_t0 = gp::values(::std::minmax(x, y)); auto a = std::get<0>(gp_t0); ...
			value = call("gp::values", value)
		}
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
		tuple, _ := fg.g.info.TypeOf(values[0]).(*types.Tuple)
		for i, t := range targets {
			if !t.blank() {
				get := &cppir.Name{Name: "std::get", Args: []cppir.Node{lit(strconv.Itoa(i))}}
				var tup cppir.Expr = cxxName(tmp)
				if tuple != nil && i < tuple.Len() && cObject(tuple.At(i).Type()) || tuple == nil && fg.g.pkg.Opaque.IsOpaque(values[0]) {
					// The tuple is a temporary of this statement: each C/C++ member moves out of
					// it, once (one that C++ can't copy, a std::unique_ptr, works).
					tup = movedName(tmp)
				}
				var x cppir.Expr = &cppir.Call{Fun: get, Args: []cppir.Expr{tup}}
				if tuple != nil && i < tuple.Len() {
					// Go converts the result implicitly to the target's type (step 3.9).
					x = fg.g.convertTo(tuple.At(i).Type(), t.gtype, x, values[0].Pos())
				}
				assign(t, x)
			}
		}
	case direct:
		for i, t := range targets {
			panics(values[i])
			assign(t, fg.storedValue(t, values[i]))
		}
	default:
		// A value is evaluated in its assignment, after the targets before it changed, if
		// that can't make a difference and gc does it there too (late): a constant, or a
		// value without calls that the stores before it can't change (affected) and that
		// doesn't read a target too early in C++ (a new variable is in scope in its own
		// initializer).
		tmps := make([]string, len(values))
		late := make([]bool, len(values))
		// A value that gc evaluates late but that C++ can't evaluate in its own store, since
		// it reads the name of a new variable declared before (p, q := 1, p+1 with an outer
		// p), goes into a temporary after the early ones, and so do the late values before it
		// (lateTemp): a declaration's stores can't panic, so that keeps gc's order.
		lateTemp := make([]bool, len(values))
		last := -1
		for i, v := range values {
			if fg.constValue(v) == nil && fg.gcLate(v, targets, i) && !fg.lateValue(v, targets, i) {
				last = i
			}
		}
		if !slices.ContainsFunc(values[:last+1], fg.panicky) {
			last = -1 // the order can't be seen: the value alone goes early
		}
		for i, v := range values[:last+1] {
			lateTemp[i] = fg.constValue(v) == nil && fg.gcLate(v, targets, i)
		}
		temp := func(i int) {
			v := values[i]
			panics(v)
			if targets[i].blank() {
				lines = append(lines, targets[i].assign(p, fg.expr(v)))
				return
			}
			tmps[i] = fg.names.temp("t")
			temps, joined = true, true
			if t := targets[i]; t.large {
				// A large boxed variable: the temporary is its box, built in place
				// (largeBox, arrays.go), which the variable takes over.
				lines = append(lines, varDecl(p, ptrType(t.typ), tmps[i], largeBox(t.typ, fg.expr(v))))
			} else if typ := targets[i].typ; typ != nil && targets[i].auto() {
				lines = append(lines, varDecl(p, typ, tmps[i], fg.ownedValue(v, fg.expr(v))))
			} else if typ != nil {
				lines = append(lines, varDecl(p, typ, tmps[i], fg.expr(v)))
			} else if targets[i].ret {
				// A result of a function with defers: a local that is returned is moved (returnMove).
				lines = append(lines, varDecl(p, typeName("auto"), tmps[i], fg.returnMove(v, fg.exactExpr(v), true)))
			} else {
				x := fg.exactExpr(v)
				if _, ok := fg.temps[ast.Unparen(v)]; !ok && fg.g.isMemberRead(v) {
					x = call("gp::cgo::load", x) // a, b = b.next, a.next: the temporaries own the structs
				}
				lines = append(lines, varDecl(p, typeName("auto"), tmps[i], x))
			}
		}
		for i, v := range values {
			save(i)
			switch {
			case lateTemp[i]:
			case fg.constValue(v) != nil:
				late[i] = true
			case fg.lateValue(v, targets, i):
				late[i] = true
			default:
				temp(i)
			}
		}
		for i := range values {
			if lateTemp[i] {
				temp(i)
			}
		}
		for i, t := range targets {
			switch {
			case t.blank() && late[i]:
				panics(values[i])
				lines = append(lines, t.assign(p, fg.expr(values[i])))
			case t.blank():
			case late[i] && t.mapX != nil && fg.panicky(values[i]):
				// gc takes the entry before it evaluates a value that can panic: the key's
				// check and a nil map's panic come first (and the key stays in the map).
				panics(values[i])
				r := fg.names.temp("r")
				lines = append(lines, varDecl(p, typeName("auto&"), r, call("gp::ref", t.mapX, t.mapK)),
					&cppir.ExprStmt{Pos: p, X: &cppir.Assign{Op: "=", X: cxxName(r), Y: fg.expr(values[i])}})
				temps, joined = true, true
			case late[i]:
				copies(i)
				panics(values[i])
				assign(t, fg.storedValue(t, values[i]))
			case tmps[i] != "" && t.large:
				box := t // the temporary is the box: moved into the variable
				box.typ, box.box, box.large = ptrType(t.typ), false, false
				assign(box, moved(box.typ, cxxName(tmps[i])))
			case tmps[i] != "" && t.typ != nil && t.class:
				assign(t, moved(t.typ, cxxName(tmps[i]))) // the temporary is not used again
			case tmps[i] != "" && fg.g.pkg.Opaque.IsOpaque(values[i]):
				assign(t, movedName(tmps[i])) // a C/C++ value: the same
			default:
				assign(t, cxxName(tmps[i]))
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

// lateValue reports whether bind evaluates the value v of targets[i] late, in its own store: gc
// does (gcLate), and it doesn't read a variable that C++ declares or changes before (reads,
// without the targets that are not variables: affected sees those).
func (fg *funcGen) lateValue(v ast.Expr, targets []target, i int) bool {
	if !fg.gcLate(v, targets, i) {
		return false
	}
	var names []target
	for _, t := range early(targets, i) {
		if t.name != "" {
			names = append(names, t)
		}
	}
	return !fg.reads(v, names)
}

// gcLate reports whether gc evaluates the value v of targets[i] late, in its own store (bind):
// it has no call left (order.go put them into temporaries) and no store before it can change
// it (affected).
func (fg *funcGen) gcLate(v ast.Expr, targets []target, i int) bool {
	for _, it := range fg.analyze(false, []ast.Expr{v}).items {
		if it.unit {
			return false
		}
	}
	return !fg.affected(v, targets[:i])
}

// valuePanics returns the statements that order the operations of the value v that can panic
// (planPanics), where bind evaluates it: its calls are in temporaries already.
func (fg *funcGen) valuePanics(v ast.Expr) []cppir.Stmt {
	return fg.orderWith(&orderScope{}, v)
}

// panicky reports whether evaluating e can panic (after the order: calls are in temporaries).
func (fg *funcGen) panicky(e ast.Expr) bool {
	for _, it := range fg.analyze(false, []ast.Expr{e}).items {
		if it.eff.panics || it.eff.call {
			return true
		}
	}
	return false
}

// storedValue returns the C++ of the value v of the target t: for a target that exists, copied
// first if the store could free it (sharedCopy, structs.go).
func (fg *funcGen) storedValue(t target, v ast.Expr) cppir.Expr {
	x := fg.expr(v)
	if t.ret {
		x = fg.returnMove(v, x, true)
	}
	if t.typ == nil && !t.blank() {
		return fg.sharedCopy(v, x)
	}
	if t.auto() {
		return fg.ownedValue(v, x) // p := std.minmax(3, 1) (crange.go)
	}
	return x
}

// auto reports whether t is a new variable declared with auto: from a C/C++ value without a
// type (x := lib.f()).
func (t target) auto() bool {
	n, ok := t.typ.(*cppir.TypeName)
	return ok && n.Name == "auto"
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
	if _, ok := fg.temps[ast.Unparen(e)]; ok {
		return false // a temporary, evaluated already
	}
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
// expression e in an assignment) can change the value of e, so that gc evaluates e before them
// (walk/assign.go's ascompatee: affected): e reads the variable that one of them is or is a
// part of (gcVar: w in w.i, s[w.i] = 1, 3, or a in a[0], x[a[0]] = 1, 2), or one of them
// writes memory and e reads memory (gcReadsMemory), or one of them is a result of a function
// with defers. gc's rule decides, also where the value can't change: it is also the order of
// the panics (bind).
func (fg *funcGen) affected(e ast.Expr, targets []target) bool {
	assigned := make(map[*types.Var]bool)
	mem := false
	for _, t := range targets {
		switch {
		case t.blank():
		case t.gcDefer:
			return true
		case t.gcMem:
			mem = true
		case t.gcVar != nil:
			assigned[t.gcVar] = true
		}
	}
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		if x, ok := n.(ast.Expr); ok && (fg.temps[x] != "" || fg.constValue(x) != nil) {
			return false // a temporary of the order (a local), a constant
		}
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := fg.g.info.Uses[id].(*types.Var); ok {
				found = assigned[v] || mem && (isPkgVar(v) || fg.isBoxed(v))
			}
			return !found
		}
		found = mem && fg.gcReadsMemory(n)
		return !found
	})
	return found
}

// gcReadsMemory reports whether gc's readsMemory says the node n (not a name) may read memory:
// every operation but arithmetic, && and ||, conversions, fields of struct values, literals
// and parentheses (conservatively also comparisons, !, len and composite literals).
func (fg *funcGen) gcReadsMemory(n ast.Node) bool {
	switch x := n.(type) {
	case *ast.BasicLit, *ast.ParenExpr, *ast.KeyValueExpr:
		return false
	case *ast.BinaryExpr:
		switch x.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM, token.AND, token.OR, token.XOR,
			token.AND_NOT, token.SHL, token.SHR, token.LAND, token.LOR:
			return false
		}
	case *ast.UnaryExpr:
		switch x.Op {
		case token.SUB, token.ADD, token.XOR:
			return false
		}
	case *ast.CallExpr:
		if fg.g.info.Types[x.Fun].IsType() && len(x.Args) == 1 {
			// A conversion; string(b), []byte(s) and []rune(s) read the bytes.
			from, to := fg.g.info.TypeOf(x.Args[0]), fg.g.info.TypeOf(x)
			return isString(from) && isSlice(to) || isSlice(from) && isString(to)
		}
	case *ast.SelectorExpr:
		if s := fg.g.info.Selections[x]; s != nil && s.Kind() == types.FieldVal {
			return s.Indirect()
		}
	}
	return true
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
		fg.add(&cppir.Label{Pos: fg.g.pos(s.Pos()), Name: fg.g.escape(s.Label.Name)})
	}
	fg.stmt(s.Stmt)
}

// branchStmt lowers break, continue, goto and fallthrough.
func (fg *funcGen) branchStmt(s *ast.BranchStmt) {
	p := fg.g.pos(s.Pos())
	switch s.Tok {
	case token.GOTO:
		fg.add(&cppir.Goto{Pos: p, Label: fg.g.escape(s.Label.Name)})
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
