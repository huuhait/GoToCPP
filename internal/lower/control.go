package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// if, for, range and switch (Phase 02 README §2.4).
//
//	Go                          C++
//	if x := f(); x > 0 {...}    { int64_t x = f(); if (x > 0) {...} }  (Go's scope of x)
//	} else if c {...}           } else if (c) {...}
//	} else if y := g(); c {     } else { int64_t y = g(); if (c) {...} }
//	for {...}                   for (;;) {...}
//	for c {...}                 while (c) {...}
//	for f(x, g()) {...}         for (;;) { auto gp_t0 = g(); if (!f(x, gp_t0)) break; ...}
//	} else if f(x, g()) {       } else { auto gp_t0 = g(); if (f(x, gp_t0)) {...} }
//	for i := 0; i < n; i++ {    for (int64_t i = 0; i < n; i++) {
//	for ...; i, j = i+1, j-1 {  for (...;) { {body} gp_cont_0:; { auto gp_t0 = i + 1; ... } }
//	for i := range n {...}      for (int64_t gp_i0 = 0, gp_n0 = n; gp_i0 < gp_n0; gp_i0++) {
//	                                int64_t i = gp_i0; ...
//	for i, r := range s {...}   for (auto gp_it0 = gp::range_string(s); gp::range_next(gp_it0);) {
//	                                int64_t i = gp_it0.gp_i; int32_t r = gp_it0.gp_r; ...
//	for i, v := range sl {...}  for (auto gp_it0 = gp::range_slice(sl); gp::range_next(gp_it0);) {
//	                                int64_t i = gp_it0.gp_i; int64_t v = gp_it0.gp_v(); ...
//	for i, v := range arr {...} for (auto gp_it0 = gp::range_array_copy(arr); ...) (arrays.go)
//	for i := range arr {...}    for (int64_t gp_i0 = 0; gp_i0 < 3; gp_i0++) {
//	switch (3 shapes)           see switchStmt
//
// Scopes: a C++ for doesn't allow a name of its init statement to be declared again in the
// outermost block of its body (Go does: the body is a block of its own). The range variable
// is declared in the C++ body, which is the same. Such a declaration shadows the loop's
// variable, so it starts a C++ block of its own (declLines, stmt.go), and both variables keep
// their Go names (funcNames):
//
//	for (int64_t gp_i0 = 0; gp_i0 < n; gp_i0++) {
//	    int64_t i = gp_i0;
//	    int64_t gp_t0 = i * 10; { int64_t i = gp_t0;
//	        ...
//	    }
//	}

// cond orders and translates a condition: of an if or an else if, a loop, a case of a
// switch. It returns the temporaries that must run right before the condition (order.go),
// which each caller puts in its own place: a loop at the top of its body, followed by
// if (!c) break; an if-else chain in a block of the else branch (chain).
func (fg *funcGen) cond(e ast.Expr) ([]cppir.Stmt, cppir.Expr) {
	pre := fg.order(false, e)
	return pre, fg.expr(e)
}

// oneLine returns the statements pre as one line, or none. Its place is set where it is
// added.
func oneLine(pre []cppir.Stmt) []*cppir.Line {
	if len(pre) == 0 {
		return nil
	}
	return []*cppir.Line{{Stmts: pre}}
}

// An arm is one branch of an if-else chain (chain).
type arm struct {
	pos  token.Pos                          // where its if (or its else) is
	init ast.Stmt                           // the statement before the condition, or nil
	cond func() ([]*cppir.Line, cppir.Expr) // lines of temporaries, the condition; nil: the else
	body func()                             // lowers the body
}

// chain lowers an if-else chain of the arms, the last of which may be an else. It
// translates each condition right before its if, so the conditions are translated in the
// order they run. An arm with an init statement or with temporaries for its condition starts
// a block, the scope of the init, that holds the rest of the chain: { init; if (c) ... }, or
// } else { init; if (c) ... } in the middle. end is the place of the closing brace.
func (fg *funcGen) chain(arms []arm, end token.Pos) {
	blocks := 0        // the blocks that the chain opened, which close at end
	var prev *cppir.If // the if of the arm before, whose else the next arm is
	for _, a := range arms {
		p := fg.g.pos(a.pos)
		elseOf := prev // the arm is the else of this if, until it opens a block
		opened := false
		open := func() {
			blk := &cppir.Block{Pos: p}
			if elseOf != nil {
				elseOf.Else = blk
				fg.fresh = false
			} else {
				fg.add(blk)
			}
			fg.push(&blk.Stmts, &blk.End)
			fg.names.openScope()
			fg.fresh = true
			blocks++
			elseOf = nil
			opened = true
		}
		// attach puts the arm's if or block where it goes.
		attach := func(s cppir.Stmt) {
			if elseOf != nil {
				elseOf.Else = s
				fg.fresh = false
			} else {
				fg.add(s)
			}
		}
		if a.init != nil {
			open()
			mark := fg.shadows
			fg.stmt(a.init)
			blocks += fg.initShadows(mark) // closed with the blocks
		}
		var body *cppir.Block
		switch {
		case a.cond != nil:
			lines, c := a.cond()
			if len(lines) > 0 && !opened {
				open()
			}
			for _, l := range lines {
				l.Pos = p
				fg.add(l)
			}
			body = &cppir.Block{}
			s := &cppir.If{Pos: p, Cond: c, Then: body}
			attach(s)
			prev = s
		default:
			// The else, or only an else: a switch with only a default.
			body = &cppir.Block{Pos: p}
			attach(body)
		}
		fg.inner(&body.Stmts, &body.End, end, a.body)
	}
	for range blocks {
		fg.names.closeScope()
		fg.pop(end)
	}
}

func (fg *funcGen) ifStmt(s *ast.IfStmt) {
	var arms []arm
	end := s.Body.Rbrace
	for s != nil {
		cond, body := s.Cond, s.Body
		arms = append(arms, arm{
			pos:  s.If,
			init: s.Init,
			cond: func() ([]*cppir.Line, cppir.Expr) {
				pre, c := fg.cond(cond)
				return oneLine(pre), c
			},
			body: func() { fg.stmts(body.List, body.Rbrace) },
		})
		end = body.Rbrace
		next := s.Else
		s = nil
		switch e := next.(type) {
		case *ast.IfStmt:
			s = e
		case *ast.BlockStmt:
			arms = append(arms, arm{pos: e.Lbrace, body: func() { fg.stmts(e.List, e.Rbrace) }})
			end = e.Rbrace
		}
	}
	fg.chain(arms, end)
}

// A loop is what loopBody needs to know about a for or range statement.
type loop struct {
	stmt  ast.Stmt
	head  cppir.Stmt     // the C++ loop, a *cppir.For or *cppir.While, without its body
	check *cppir.Line    // the condition with its temporaries at the top of the body, or nil
	vars  func()         // lowers the declarations at the start of each iteration, or nil
	body  *ast.BlockStmt // the Go body
	post  ast.Stmt       // a post statement that is not one C++ expression, or nil
	renew *cppir.Line    // the new boxes of the loop's variables for the next iteration, or nil
}

// loopBody lowers a loop from its head to its closing brace, and the label after it that a
// break L needs (flow.go). A condition with temporaries (check) comes first, in the loop's
// own block, so continue goes to it as to the head's condition. The body is the C++ loop's
// own block, or, if a goto goes to the end of the body (continue), a block inside it,
// followed by the label and the post statement: for (...) { { body } gp_cont_0:; post; }.
func (fg *funcGen) loopBody(l loop) {
	shaped := fg.flow.contLabel[l.stmt] || l.post != nil || l.renew != nil
	if fg.flow.contLabel[l.stmt] {
		if fg.flow.label[l.stmt] != nil {
			fg.contLabels[l.stmt] = fg.flow.labelName(l.stmt, "continue")
		} else {
			fg.contLabels[l.stmt] = fg.names.temp("cont_")
		}
	}
	body := &cppir.Block{}
	switch h := l.head.(type) {
	case *cppir.For:
		h.Body = body
	case *cppir.While:
		h.Body = body
	}
	fg.add(l.head)
	fg.push(&body.Stmts, &body.End)
	if l.check != nil {
		fg.add(l.check)
	}
	if l.vars != nil {
		l.vars()
	}
	if shaped {
		fg.block(l.body)
		if label := fg.contLabels[l.stmt]; label != "" {
			fg.add(&cppir.Label{Pos: fg.g.pos(l.body.Rbrace), Name: label})
		}
		if l.renew != nil {
			fg.add(l.renew)
		}
		if l.post != nil {
			fg.stmt(l.post)
		}
	} else {
		// The C++ body has the loop's own variables (see the top), so it is fresh only for a
		// loop without them (declLines).
		fg.fresh = l.vars == nil && l.check == nil && fg.g.info.Scopes[l.stmt].Len() == 0
		fg.stmts(l.body.List, l.body.Rbrace)
	}
	fg.pop(l.body.Rbrace)
}

// breakLabel adds the label after the statement s that a break L goes to, if one does.
func (fg *funcGen) breakLabel(s ast.Stmt, end token.Pos) {
	if fg.flow.breakLabel[s] {
		fg.add(&cppir.Label{Pos: fg.g.pos(end), Name: fg.flow.labelName(s, "break")})
	}
}

// open starts a block, a scope, at pos (for an init statement or temporaries).
func (fg *funcGen) open(pos token.Pos) {
	blk := &cppir.Block{Pos: fg.g.pos(pos)}
	fg.add(blk)
	fg.push(&blk.Stmts, &blk.End)
	fg.names.openScope()
	fg.fresh = true
}

// close ends the block of open, at pos.
func (fg *funcGen) close(pos token.Pos) {
	fg.names.closeScope()
	fg.pop(pos)
}

func (fg *funcGen) forStmt(s *ast.ForStmt) {
	end := s.Body.Rbrace
	fg.names.openScope() // the init and the body (see the top)
	wrapped := false
	shadows := 0 // the Scopes that the init opened for shadowing variables (declLines)
	var init cppir.Stmt
	if s.Init != nil {
		if st, ok := fg.forInit(s.Init); ok {
			init = st
		} else {
			// { a, b := f(); for (; ...) {...} }
			blk := &cppir.Block{Pos: fg.g.pos(s.For)}
			fg.add(blk)
			fg.push(&blk.Stmts, &blk.End)
			fg.fresh = true
			wrapped = true
			mark := fg.shadows
			fg.loopInit = true // bind: the loop's variables are its own copies for gc
			fg.stmt(s.Init)
			fg.loopInit = false
			shadows = fg.initShadows(mark)
		}
	}
	l := loop{stmt: s, body: s.Body}
	var cond cppir.Expr
	if s.Cond != nil {
		pre, c := fg.cond(s.Cond)
		if len(pre) > 0 {
			// The temporaries run before every evaluation of the condition.
			stop := &cppir.If{Cond: &cppir.Unary{Op: "!", X: c}, Then: &cppir.Break{}}
			l.check = &cppir.Line{Pos: fg.g.pos(s.Cond.Pos()), Stmts: append(pre, stop)}
		} else {
			cond = c
		}
	}
	var post cppir.Expr
	if renew := fg.loopBoxes(s); renew != nil {
		// Go 1.22: each iteration has its own variables, which a pointer (or, from step
		// 3.11, a closure) can keep. The next iteration's variables are new boxes with the
		// values of the last ones, made before the post statement, which changes them
		// (the spec, "For statements with for clause").
		line := &cppir.Line{Pos: fg.g.pos(s.For)}
		if s.Post != nil {
			line.Pos = fg.g.pos(s.Post.Pos())
		}
		for _, v := range renew {
			name := fg.cname(v, s.For)
			typ := fg.g.cxxType(v.Type(), v.Pos())
			line.Stmts = append(line.Stmts, &cppir.ExprStmt{X: &cppir.Assign{Op: "=", X: cxxName(name), Y: newBox(typ, deref(cxxName(name)))}})
		}
		l.renew = line
		l.post = s.Post
	} else if s.Post != nil {
		if fg.postInHead(s.Post) {
			var pre []cppir.Stmt
			pre, post = fg.simple(s.Post)
			if len(pre) > 0 {
				fg.g.errorf(s.Post.Pos(), "internal error: a post statement with temporaries in the head")
			}
		} else {
			l.post = s.Post
		}
	}
	p := fg.g.pos(s.For)
	if init == nil && post == nil && cond != nil {
		l.head = &cppir.While{Pos: p, Cond: cond}
	} else {
		l.head = &cppir.For{Pos: p, Init: init, Cond: cond, Post: post} // for (;;) without any
	}
	fg.loopBody(l)
	if wrapped {
		fg.shadows += shadows
		fg.closeShadows(fg.shadows-shadows, end)
		fg.pop(end)
	}
	fg.names.closeScope()
	fg.breakLabel(s, end)
}

// loopBoxes returns the boxed variables that the init statement of the 3-clause for s
// declares (box.go), which each iteration gets a new box of (forStmt).
func (fg *funcGen) loopBoxes(s *ast.ForStmt) []*types.Var {
	a, ok := s.Init.(*ast.AssignStmt)
	if !ok || a.Tok != token.DEFINE {
		return nil
	}
	var vars []*types.Var
	for _, e := range a.Lhs {
		if v, ok := fg.g.info.Defs[e.(*ast.Ident)].(*types.Var); ok && fg.isBoxed(v) {
			vars = append(vars, v)
		}
	}
	return vars
}

// postAtEnd reports whether the 3-clause for s has its post statement at the end of its body
// (and a continue must go there, flow.go): a post statement that can't be in the C++ for's
// head, or the new boxes of its variables (loopBoxes).
func (fg *funcGen) postAtEnd(s *ast.ForStmt) bool {
	return s.Post != nil && !fg.postInHead(s.Post) || fg.loopBoxes(s) != nil
}

// forInit returns the C++ init statement of a 3-clause for, if the Go init statement is one:
// a simple statement that is one C++ expression, or a := whose values all go into new
// variables of one C++ type (int64_t i = 0, j = n), without temporaries (order.go, bind). It
// returns false for the others, which go before the loop.
func (fg *funcGen) forInit(s ast.Stmt) (cppir.Stmt, bool) {
	if oneExpr(s) {
		if fg.simpleNeedsPre(s) {
			return nil, false
		}
		_, x := fg.simple(s)
		return &cppir.ExprStmt{X: x}, true
	}
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.DEFINE || len(a.Lhs) != len(a.Rhs) || fg.needsPre(true, a.Rhs...) {
		return nil, false
	}
	var typ cppir.Type // of the C++ variables: gp::ptr<T> for boxed ones
	for _, e := range a.Lhs {
		obj, ok := fg.g.info.Defs[e.(*ast.Ident)].(*types.Var)
		if !ok || isBlank(e) {
			return nil, false // _ or a variable that exists
		}
		t := fg.g.cxxType(obj.Type(), e.Pos())
		if fg.isBoxed(obj) {
			t = ptrType(t)
		}
		if typ != nil && cppir.TypeString(t) != cppir.TypeString(typ) {
			return nil, false
		}
		typ = t
	}
	// A value that reads the name of its own variable or of one before it (for i := i; ...)
	// needs a temporary (bind): the init goes before the loop.
	for i, e := range a.Rhs {
		used := fg.usedNames([]ast.Expr{e})
		for _, id := range a.Lhs[:i+1] {
			if used[Escape(id.(*ast.Ident).Name)] {
				return nil, false
			}
		}
	}
	targets := fg.newTargets(a.Lhs)
	for _, t := range targets {
		// A shadowing variable is in the for's own scope already: the init is all there is
		// before the loop's variables in it (declLines).
		if t.shadow && !fg.names.enter(t.name) {
			fg.g.errorf(s.Pos(), "internal error: %s is declared twice in one C++ block", t.name)
		}
	}
	fg.order(true, a.Rhs...) // the declarators are evaluated in order; nothing goes in front
	d := &cppir.VarDecl{Type: typ}
	for i, t := range targets {
		init := fg.expr(a.Rhs[i])
		if t.box {
			init = newBox(t.typ, init)
		}
		d.Vars = append(d.Vars, cppir.Declarator{Name: t.name, Init: init})
	}
	return d, true
}

// rangeStmt lowers a range loop. Phase 02 has range over an integer n: a hidden counter goes
// from 0 to n-1, n is evaluated once (with its temporaries, if it needs any, in a block in
// front), and the range variable is a copy of the counter, so assigning to it in the body
// doesn't change the iteration (Go 1.22). A variable that exists (for i = range n) is
// assigned the counter at the start of each iteration. A range over an array (step 3.6) without
// a value variable is the same loop with n = N, where the array is not evaluated unless it has
// calls (gp::len(f()), arrays.go: rangeCount).
func (fg *funcGen) rangeStmt(s *ast.RangeStmt) {
	xt := fg.g.info.TypeOf(s.X)
	b := underBasic(xt) // also a named string or integer type (step 3.9)
	switch {
	case b != nil && b.Info()&types.IsString != 0:
		fg.rangeIter(s, "gp::range_string", func(it cppir.Expr) cppir.Expr { return &cppir.Member{X: it, Name: "gp_r"} })
		return
	case isSlice(xt):
		fg.rangeIter(s, "gp::range_slice", func(it cppir.Expr) cppir.Expr { return &cppir.Call{Fun: &cppir.Member{X: it, Name: "gp_v"}} })
		return
	case isMap(xt):
		fg.rangeIter(s, "gp::range_map", func(it cppir.Expr) cppir.Expr { return &cppir.Call{Fun: &cppir.Member{X: it, Name: "gp_value"}} })
		return
	case arrayOf(xt) != nil:
		if fg.rangeArray(s) { // arrays.go: with a value variable
			return
		}
		xt, b = types.Typ[types.Int], types.Typ[types.Int] // a counter to len(x)
	}
	if b == nil || b.Info()&types.IsInteger == 0 {
		fg.g.unsupported(s.X.Pos(), rangeKind(xt))
		return
	}
	// The type of the counter (the spec, "For statements with range clause"): n's type, or
	// for an untyped constant n the type of a variable that exists, or else int.
	t := xt
	var key *types.Var // a new range variable
	assigned := false  // the key is a variable that exists, or another target (s[i], m[k])
	if s.Key != nil && !isBlank(s.Key) {
		if s.Tok == token.DEFINE {
			key, _ = fg.g.info.Defs[s.Key.(*ast.Ident)].(*types.Var)
		} else {
			assigned = true
		}
	}
	if b.Info()&types.IsUntyped != 0 {
		switch {
		case key != nil:
			t = key.Type()
		case assigned:
			t = fg.g.info.TypeOf(s.Key)
		default:
			t = types.Typ[types.Int]
		}
	}
	ct := fg.g.cxxType(t, s.X.Pos())

	pre := fg.order(false, s.X)
	if len(pre) > 0 {
		fg.open(s.For)
		fg.add(&cppir.Line{Pos: fg.g.pos(s.X.Pos()), Stmts: pre})
	}
	fg.names.openScope() // the range variable and the body (see the top)
	i := fg.names.temp("i")
	init := varDecl(cppir.Pos{}, ct, i, fg.g.constant(constant.MakeInt64(0), t, s.X.Pos(), false)) // N(0) for a named N
	var limit cppir.Expr
	count := fg.constValue(s.X)
	if count == nil {
		count = fg.rangeCount(s) // N for an array that is not evaluated (arrays.go)
	}
	if count != nil {
		limit = at(fg.g.constant(count, t, s.X.Pos(), false), precShift)
	} else {
		n := fg.names.temp("n")
		init.Vars = append(init.Vars, cppir.Declarator{Name: n, Init: fg.rangeLimit(s.X)})
		limit = cxxName(n)
	}
	head := &cppir.For{
		Pos:  fg.g.pos(s.For),
		Init: init,
		Cond: &cppir.Binary{Op: "<", X: cxxName(i), Y: limit},
		Post: &cppir.Postfix{X: cxxName(i), Op: "++"},
	}
	l := loop{stmt: s, head: head, body: s.Body}
	switch {
	case key != nil && fg.isBoxed(key):
		l.vars = func() {
			fg.add(varDecl(fg.g.pos(s.Key.Pos()), ptrType(ct), fg.names.declare(key), newBox(ct, cxxName(i))))
		}
	case key != nil:
		l.vars = func() { fg.add(varDecl(fg.g.pos(s.Key.Pos()), ct, fg.names.declare(key), cxxName(i))) }
	case assigned:
		// As by s.Key = gp_i0 at the start of each iteration: the target's operands first (with
		// their own order), then the store (gp::set for a map entry).
		l.vars = func() {
			line := &cppir.Line{Pos: fg.g.pos(s.Key.Pos())}
			line.Stmts = append(line.Stmts, fg.order(false, fg.targetOperands([]ast.Expr{s.Key})...)...)
			line.Stmts = append(line.Stmts, &cppir.ExprStmt{X: fg.lhs(s.Key).assignExpr(cxxName(i))})
			fg.add(line)
		}
	}
	fg.loopBody(l)
	fg.names.closeScope()
	if len(pre) > 0 {
		fg.close(s.Body.Rbrace)
	}
	fg.breakLabel(s, s.Body.Rbrace)
}

// rangeIter lowers a range loop over a string (Phase 03 step 3.4) or a slice (step 3.5). The
// range expression is evaluated once, into the loop's iterator (with its temporaries, if it
// needs any, in a block in front, as for range over an integer), which holds it:
//
//	for i, r := range s {       for (auto gp_it0 = gp::range_string(s); gp::range_next(gp_it0);) {
//	                                int64_t i = gp_it0.gp_i; int32_t r = gp_it0.gp_r;
//	for i, v := range sl {      for (auto gp_it0 = gp::range_slice(sl); gp::range_next(gp_it0);) {
//	                                int64_t i = gp_it0.gp_i; int64_t v = gp_it0.gp_v();
//
// For a string, gp::range_next steps from rune to rune, decoding UTF-8 like Go: an invalid byte
// is U+FFFD and steps one byte. For a slice, the iterator holds a copy of the slice, so the loop
// covers the original length and array also when the body appends to it or assigns it, and
// gp_v() reads the element when the iteration starts (a write to a later element in the body is
// seen, as in Go). The range variables are copies of the hidden counters, so assigning to them
// in the body doesn't change the iteration (Go 1.22), and each iteration has its own. Every form
// is the same loop. A variable that exists (for i, r = range s) is assigned at the start of each
// iteration, as by i, r = gp_i, gp_r: the operands of a target that is not a variable (for i,
// s[i] = range ...) are evaluated before the stores, each iteration (saveOperands); a blank one,
// or one that is left out (for range s, for _, r := range s), is neither.
//
// A map (step 3.7) is the same loop, gp::range_map(m), with the key gp_it0.gp_key() (rangeKey)
// and the value gp_it0.gp_value(): the iterator holds the map and a snapshot of its entries
// (gocpp/map.hpp), so a key deleted before it is reached is skipped, its value is read when it
// is reached, and a key added during the loop is not produced (Go may or may not produce it).
func (fg *funcGen) rangeIter(s *ast.RangeStmt, start string, value func(it cppir.Expr) cppir.Expr) {
	pre := fg.order(false, s.X)
	if len(pre) > 0 {
		fg.open(s.For)
		fg.add(&cppir.Line{Pos: fg.g.pos(s.X.Pos()), Stmts: pre})
	}
	fg.names.openScope() // the range variables and the body (see the top)
	it := fg.names.temp("it")
	xt := fg.g.info.TypeOf(s.X)
	head := &cppir.For{
		Pos:  fg.g.pos(s.For),
		Init: varDecl(cppir.Pos{}, typeName("auto"), it, call(start, arrayPtrValue(xt, fg.expr(s.X)))),
		Cond: call("gp::range_next", cxxName(it)),
	}
	// The types of the key and the value, which Go converts implicitly to the types of targets
	// that exist (for k, v = range x: step 3.9, named.go).
	keyType, valueType := rangeTypes(xt)
	// The line at the start of each iteration: a declaration or an assignment of each range
	// variable, made from its counter (as in Go, the key first), on one C++ line.
	type rangeVar struct {
		e     ast.Expr
		obj   *types.Var // a new variable
		typ   cppir.Type // its C++ type
		value cppir.Expr
	}
	var vars []rangeVar
	key := rangeKey(start, cxxName(it)) // gp_i, or a map's gp_key() (maps.go)
	for _, v := range []rangeVar{{e: s.Key, value: key}, {e: s.Value, value: value(cxxName(it))}} {
		if v.e == nil || isBlank(v.e) {
			continue
		}
		if s.Tok == token.DEFINE {
			v.obj, _ = fg.g.info.Defs[v.e.(*ast.Ident)].(*types.Var)
			if v.obj == nil {
				fg.g.errorf(v.e.Pos(), "internal error: a range variable without an object")
				continue
			}
			v.typ = fg.g.cxxType(v.obj.Type(), v.e.Pos())
		}
		vars = append(vars, v)
	}
	l := loop{stmt: s, head: head, body: s.Body}
	if len(vars) > 0 {
		l.vars = func() {
			line := &cppir.Line{Pos: fg.g.pos(vars[0].e.Pos())}
			if s.Tok != token.DEFINE {
				// An assignment of the counters to the targets: their operands first.
				lhs := make([]ast.Expr, len(vars))
				for i, v := range vars {
					lhs[i] = v.e
				}
				if start == "gp::range_map" {
					// The key and the value are references into the map's entry, which a call
					// in the targets' operands can delete (maps.go).
					for i, v := range vars {
						line.Stmts = append(line.Stmts, fg.rangeMapVar(fg.targetOperands(lhs), v.e == s.Key, cxxName(it), &vars[i].value)...)
					}
				}
				line.Stmts = append(line.Stmts, fg.order(false, fg.targetOperands(lhs)...)...)
				targets := make([]target, len(vars))
				for i, v := range vars {
					targets[i] = fg.lhs(v.e)
				}
				for i := range targets {
					line.Stmts = append(line.Stmts, fg.saveOperands(cppir.Pos{}, targets, i)...)
				}
				for i, t := range targets {
					from := valueType
					if vars[i].e == s.Key {
						from = keyType
					}
					value := fg.g.convertTo(from, fg.g.info.TypeOf(vars[i].e), vars[i].value, vars[i].e.Pos())
					line.Stmts = append(line.Stmts, &cppir.ExprStmt{X: t.assignExpr(value)})
				}
				fg.add(line)
				return
			}
			for _, v := range vars {
				if fg.isBoxed(v.obj) {
					line.Stmts = append(line.Stmts, varDecl(cppir.Pos{}, ptrType(v.typ), fg.names.declare(v.obj), newBox(v.typ, v.value)))
				} else {
					line.Stmts = append(line.Stmts, varDecl(cppir.Pos{}, v.typ, fg.names.declare(v.obj), v.value))
				}
			}
			fg.add(line)
		}
	}
	fg.loopBody(l)
	fg.names.closeScope()
	if len(pre) > 0 {
		fg.close(s.Body.Rbrace)
	}
	fg.breakLabel(s, s.Body.Rbrace)
}

// rangeKind names what a range over a value of type t is, for "not supported yet".
func rangeKind(t types.Type) string {
	switch types.Unalias(t).Underlying().(type) {
	case *types.Signature:
		return "range over functions (Phase 07)"
	case *types.Chan:
		return "range over channels (Phase 08)"
	}
	return "range over " + types.TypeString(t, nil) + " (Phase 03)"
}

// withLead returns s, with the statements lead in front of it on its first line (a Line), if
// there are any.
func withLead(pos cppir.Pos, lead []cppir.Stmt, s cppir.Stmt) cppir.Stmt {
	if len(lead) == 0 {
		return s
	}
	return &cppir.Line{Pos: pos, Stmts: append(lead, s)}
}

// switchStmt lowers a switch statement. Go evaluates the tag once, then the case expressions
// one by one in source order, only until one matches; default runs if none does, wherever it
// is. There are three shapes:
//
//  1. An integer tag and only constant cases: a C++ switch on the tag itself. go/types
//     rejects duplicate constant cases, as C++ does.
//
//  2. No fallthrough, and no break to the switch: an if-else chain of the cases in source
//     order, with the default as the final else.
//
//     if (gp_tag0 == "a"_gs || gp_tag0 == "b"_gs) {...} else if (...) {...} else {...}
//
//  3. Otherwise the index of the case that matches, from the same chain, then a C++ switch on
//     it, with the cases in source order, so fallthrough is [[fallthrough]] and break is
//     break:
//
//     int gp_case0 = -1;
//     if (gp_tag0 == 1) { gp_case0 = 0; } else if (...) { gp_case0 = 1; }
//     switch (gp_case0) { case 0: {...} default: {...} case 1: {...} }
//
// The tag goes into a temporary (gp_tag0) unless it is a constant or a local variable that
// a case can't change (isLocal: not a boxed one). An init statement or temporaries go into a
// block first, { ... }; so do the temporaries of the tag's own order (order.go). Those of a case expression are lazy, like
// the case: in the chain, in a block of the else branch, before its if (caseCond). In the
// C++ switches, each case is a block, so that a case label doesn't jump over the
// initialization of a variable of another case.
func (fg *funcGen) switchStmt(s *ast.SwitchStmt) {
	end := s.Body.Rbrace
	wrapped := false
	wrap := func() {
		if !wrapped {
			fg.open(s.Switch)
			wrapped = true
		}
	}
	shadows := 0 // the Scopes that the init opened for shadowing variables (declLines)
	if s.Init != nil {
		wrap()
		mark := fg.shadows
		fg.stmt(s.Init)
		shadows = fg.initShadows(mark)
	}
	var tagPre []cppir.Stmt // the temporaries of the tag, in front of the line that evaluates it
	if s.Tag != nil {
		if pre := fg.order(false, s.Tag); len(pre) > 0 {
			wrap()
			tagPre = pre
		}
	}
	clauses := make([]*ast.CaseClause, len(s.Body.List))
	cases := false // a clause other than default
	for i, c := range s.Body.List {
		clauses[i] = c.(*ast.CaseClause)
		cases = cases || clauses[i].List != nil
	}

	p := fg.g.pos(s.Switch)
	if fg.integerSwitch(s, clauses) {
		// A tag of a named integer type switches on its value, with the constants of the
		// underlying type (C++ rejects a class type in a switch: step 3.9).
		tt := fg.g.info.TypeOf(s.Tag)
		sw := &cppir.Switch{Pos: p, Tag: valueOf(tt, fg.expr(s.Tag))}
		fg.add(withLead(p, tagPre, sw))
		for _, c := range clauses {
			var values []cppir.Expr // nil: default
			for _, e := range c.List {
				values = append(values, fg.g.constant(fg.constValue(e), under(tt), e.Pos(), false))
			}
			fg.caseBody(sw, c, values)
		}
		sw.End = fg.g.pos(end)
	} else {
		var tag cppir.Expr // nil: no tag
		if s.Tag != nil {
			tp := fg.g.pos(s.Tag.Pos())
			switch {
			case fg.constValue(s.Tag) != nil || fg.isLocal(s.Tag):
				tag = at(fg.expr(s.Tag), precShift)
			case !cases:
				fg.add(withLead(tp, tagPre, &cppir.ExprStmt{Pos: tp, X: voidCast(fg.expr(s.Tag))}))
			default:
				wrap()
				t := fg.names.temp("tag")
				tag = cxxName(t)
				fg.add(withLead(tp, tagPre, varDecl(tp, typeName("auto"), t, fg.exactExpr(s.Tag))))
			}
		}
		// A bool tag's value, for the constant cases (that of a named bool type: step 3.9).
		var boolTag cppir.Expr
		if b := underBasic(fg.g.info.TypeOf(s.Tag)); tag != nil && b != nil && b.Info()&types.IsBoolean != 0 {
			boolTag = valueOf(fg.g.info.TypeOf(s.Tag), tag)
		}
		caseCond := func(c *ast.CaseClause) func() ([]*cppir.Line, cppir.Expr) {
			return func() ([]*cppir.Line, cppir.Expr) { return fg.caseCond(tag, boolTag, c.List) }
		}
		var arms []arm
		var def *ast.CaseClause
		if !fallsThrough(clauses) && !fg.flow.breakTo[s] {
			for _, c := range clauses {
				if c.List == nil {
					def = c
					continue
				}
				arms = append(arms, arm{pos: c.Case, cond: caseCond(c), body: func() { fg.stmts(c.Body, clauseEnd(c)) }})
			}
			if def != nil {
				arms = append(arms, arm{pos: def.Case, body: func() { fg.stmts(def.Body, clauseEnd(def)) }})
			}
			if arms != nil {
				fg.chain(arms, end)
			}
		} else {
			wrap()
			index := fg.names.temp("case")
			fg.add(varDecl(p, typeName("int"), index, &cppir.Unary{Op: "-", X: lit("1")}))
			n := 0
			values := make([][]cppir.Expr, len(clauses))
			for i, c := range clauses {
				if c.List == nil {
					continue // default
				}
				k := strconv.Itoa(n)
				n++
				values[i] = []cppir.Expr{lit(k)}
				arms = append(arms, arm{pos: c.Case, cond: caseCond(c), body: func() {
					fg.add(&cppir.ExprStmt{Pos: fg.g.pos(c.Case), X: &cppir.Assign{Op: "=", X: cxxName(index), Y: lit(k)}})
				}})
			}
			if arms != nil {
				fg.chain(arms, arms[len(arms)-1].pos)
			}
			sw := &cppir.Switch{Pos: p, Tag: cxxName(index)}
			fg.add(sw)
			for i, c := range clauses {
				fg.caseBody(sw, c, values[i])
			}
			sw.End = fg.g.pos(end)
		}
	}
	if wrapped {
		fg.shadows += shadows
		fg.closeShadows(fg.shadows-shadows, end)
		fg.close(end)
	}
	fg.breakLabel(s, end)
}

// integerSwitch reports whether the switch s can be a C++ switch on its tag (shape 1): an
// integer tag that is not a constant (clang warns about a constant one that no case matches),
// and only constant case expressions.
func (fg *funcGen) integerSwitch(s *ast.SwitchStmt, clauses []*ast.CaseClause) bool {
	if s.Tag == nil || fg.constValue(s.Tag) != nil {
		return false
	}
	if b := underBasic(fg.g.info.TypeOf(s.Tag)); b == nil || b.Info()&types.IsInteger == 0 {
		return false
	}
	for _, c := range clauses {
		for _, e := range c.List {
			if fg.constValue(e) == nil {
				return false
			}
		}
	}
	return true
}

// isLocal reports whether e is a variable of the function (a parameter, a result or a local
// variable) that no call can change: not a package variable, and not boxed (box.go), which
// a call can change through a pointer.
func (fg *funcGen) isLocal(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	obj, ok := fg.g.info.Uses[id].(*types.Var)
	if !ok || fg.isBoxed(obj) {
		return false
	}
	_, ok = fg.names.names[obj]
	return ok
}

// caseCond returns the C++ condition of a case with the expressions list: each one compared
// with the tag (nil for a switch without one), joined with ||, which evaluates them in order
// and only until one matches, as Go does. It returns the lines of temporaries that go before
// the condition too. An expression after the first that needs temporaries would run them too
// early, so from there on the condition is a bool temporary that each expression sets only if
// the ones before it didn't match, as gc does for ||, one line for each:
//
//	bool gp_t0 = gp_tag0 == 1;
//	if (!gp_t0) { auto gp_t1 = g(); gp_t0 = gp_tag0 == f(x, gp_t1); }
func (fg *funcGen) caseCond(tag, boolTag cppir.Expr, list []ast.Expr) ([]*cppir.Line, cppir.Expr) {
	lazy := -1 // the first expression after the first one that needs temporaries
	for i, e := range list {
		if i > 0 && lazy < 0 && fg.needsPre(false, e) {
			lazy = i
		}
	}
	parts := make([]cppir.Expr, len(list))
	pres := make([][]cppir.Stmt, len(list))
	t := ""
	for i, e := range list {
		if i == lazy {
			t = fg.names.temp("t") // before the temporaries that it guards
		}
		if tag == nil {
			var c cppir.Expr
			pres[i], c = fg.cond(e)
			if len(list) > 1 {
				parts[i] = at(c, precBitOr) // (a && b) || c: g++ -Wall asks for them
			} else {
				parts[i] = c
			}
		} else if v := fg.constValue(e); boolTag != nil && v != nil {
			// A bool tag and a constant case: b or !b, not b == true (boolTag is the tag's
			// bool, b.gp_value() for a named bool type).
			parts[i] = boolTag
			if !constant.BoolVal(v) {
				parts[i] = &cppir.Unary{Op: "!", X: boolTag}
			}
		} else {
			pres[i] = fg.order(false, e)
			parts[i] = &cppir.Binary{Op: "==", X: tag, Y: at(fg.expr(e), precShift)}
		}
	}
	if lazy < 0 {
		return oneLine(pres[0]), orList(parts)
	}
	lines := oneLine(append(pres[0], varDecl(cppir.Pos{}, typeName("bool"), t, orList(parts[:lazy]))))
	for i := lazy; i < len(list); i++ {
		set := &cppir.ExprStmt{X: &cppir.Assign{Op: "=", X: cxxName(t), Y: parts[i]}}
		guard := &cppir.If{
			Cond: &cppir.Unary{Op: "!", X: cxxName(t)},
			Then: &cppir.Block{Stmts: append(pres[i], set), Inline: true},
		}
		lines = append(lines, &cppir.Line{Stmts: []cppir.Stmt{guard}})
	}
	return lines, cxxName(t)
}

// orList returns the expressions joined with ||: a || b || c.
func orList(list []cppir.Expr) cppir.Expr {
	x := list[0]
	for _, y := range list[1:] {
		x = &cppir.Binary{Op: "||", X: x, Y: y}
	}
	return x
}

// fallsThrough reports whether a clause ends with fallthrough.
func fallsThrough(clauses []*ast.CaseClause) bool {
	for _, c := range clauses {
		if endsWithFallthrough(c) {
			return true
		}
	}
	return false
}

func endsWithFallthrough(c *ast.CaseClause) bool {
	b, ok := lastStmt(c.Body).(*ast.BranchStmt)
	return ok && b.Tok == token.FALLTHROUGH
}

// endsWithJump reports whether the last statement of a clause is a return, break, continue,
// goto or fallthrough, after which a break would be dead code.
func endsWithJump(c *ast.CaseClause) bool {
	switch lastStmt(c.Body).(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	}
	return false
}

// lastStmt returns the last statement of list that is not empty, without its labels (Go
// allows `L: fallthrough` and `fallthrough; ;` at the end of a case), or nil.
func lastStmt(list []ast.Stmt) ast.Stmt {
	for i := len(list) - 1; i >= 0; i-- {
		s := list[i]
		for {
			l, ok := s.(*ast.LabeledStmt)
			if !ok {
				break
			}
			s = l.Stmt
		}
		if _, empty := s.(*ast.EmptyStmt); !empty {
			return s
		}
		if s != list[i] {
			return s // a labeled empty statement: L: at the end
		}
	}
	return nil
}

// caseBody adds the clause c to the C++ switch sw, with the case values (nil: default): its
// body is a block, which is followed by break unless it ends with a jump (fallthrough falls
// into the next case). The printer writes them on one line, } break;, so that the end of a
// case needs no more #line.
func (fg *funcGen) caseBody(sw *cppir.Switch, c *ast.CaseClause, values []cppir.Expr) {
	cs := &cppir.Case{Pos: fg.g.pos(c.Case), Values: values, Break: !endsWithJump(c)}
	sw.Cases = append(sw.Cases, cs)
	fg.fresh = false
	fg.inner(&cs.Body, &cs.End, clauseEnd(c), func() { fg.stmts(c.Body, clauseEnd(c)) })
}

// clauseEnd returns the place of the end of a clause: its last statement, or the colon.
func clauseEnd(c *ast.CaseClause) token.Pos {
	if len(c.Body) == 0 {
		return c.Colon
	}
	return c.Body[len(c.Body)-1].End()
}
