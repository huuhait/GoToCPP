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
}

// loopBody lowers a loop from its head to its closing brace, and the label after it that a
// break L needs (flow.go). A condition with temporaries (check) comes first, in the loop's
// own block, so continue goes to it as to the head's condition. The body is the C++ loop's
// own block, or, if a goto goes to the end of the body (continue), a block inside it,
// followed by the label and the post statement: for (...) { { body } gp_cont_0:; post; }.
func (fg *funcGen) loopBody(l loop) {
	shaped := fg.flow.contLabel[l.stmt] || l.post != nil
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
			fg.stmt(s.Init)
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
	if s.Post != nil {
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

// forInit returns the C++ init statement of a 3-clause for, if the Go init statement is one:
// a simple statement that is one C++ expression, or a := whose values all go into new
// variables of one C++ type (int64_t i = 0, j = n), without temporaries (order.go, bind). It
// returns false for the others, which go before the loop.
func (fg *funcGen) forInit(s ast.Stmt) (cppir.Stmt, bool) {
	if oneExpr(s) {
		if fg.needsPre(false, simpleScope(s)...) {
			return nil, false
		}
		_, x := fg.simple(s)
		return &cppir.ExprStmt{X: x}, true
	}
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.DEFINE || len(a.Lhs) != len(a.Rhs) || fg.needsPre(true, a.Rhs...) {
		return nil, false
	}
	var typ cppir.Type
	for _, e := range a.Lhs {
		obj, ok := fg.g.info.Defs[e.(*ast.Ident)].(*types.Var)
		if !ok || isBlank(e) {
			return nil, false // _ or a variable that exists
		}
		t := fg.g.cxxType(obj.Type(), e.Pos())
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
		d.Vars = append(d.Vars, cppir.Declarator{Name: t.name, Init: fg.expr(a.Rhs[i])})
	}
	return d, true
}

// rangeStmt lowers a range loop. Phase 02 has range over an integer n: a hidden counter goes
// from 0 to n-1, n is evaluated once (with its temporaries, if it needs any, in a block in
// front), and the range variable is a copy of the counter, so assigning to it in the body
// doesn't change the iteration (Go 1.22). A variable that exists (for i = range n) is
// assigned the counter at the start of each iteration.
func (fg *funcGen) rangeStmt(s *ast.RangeStmt) {
	xt := fg.g.info.TypeOf(s.X)
	b := basicOf(xt)
	if b != nil && b.Info()&types.IsString != 0 {
		fg.rangeString(s)
		return
	}
	if b == nil || b.Info()&types.IsInteger == 0 {
		fg.g.unsupported(s.X.Pos(), rangeKind(xt))
		return
	}
	// The type of the counter (the spec, "For statements with range clause"): n's type, or
	// for an untyped constant n the type of a variable that exists, or else int.
	t := xt
	var key *types.Var      // a new range variable
	var assigned cppir.Expr // the C++ of a variable that exists
	if s.Key != nil && !isBlank(s.Key) {
		if s.Tok == token.DEFINE {
			key, _ = fg.g.info.Defs[s.Key.(*ast.Ident)].(*types.Var)
		} else {
			assigned = fg.expr(s.Key)
		}
	}
	if b.Info()&types.IsUntyped != 0 {
		switch {
		case key != nil:
			t = key.Type()
		case assigned != nil:
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
	init := varDecl(cppir.Pos{}, ct, i, lit("0"))
	var limit cppir.Expr
	if v := fg.constValue(s.X); v != nil {
		limit = at(fg.g.constant(v, t, s.X.Pos(), false), precShift)
	} else {
		n := fg.names.temp("n")
		init.Vars = append(init.Vars, cppir.Declarator{Name: n, Init: fg.expr(s.X)})
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
	case key != nil:
		l.vars = func() { fg.add(varDecl(fg.g.pos(s.Key.Pos()), ct, fg.names.declare(key), cxxName(i))) }
	case assigned != nil:
		l.vars = func() {
			fg.add(&cppir.ExprStmt{Pos: fg.g.pos(s.Key.Pos()), X: &cppir.Assign{Op: "=", X: assigned, Y: cxxName(i)}})
		}
	}
	fg.loopBody(l)
	fg.names.closeScope()
	if len(pre) > 0 {
		fg.close(s.Body.Rbrace)
	}
	fg.breakLabel(s, s.Body.Rbrace)
}

// rangeString lowers a range loop over a string (Phase 03 step 3.4). The string is evaluated
// once, into the loop's gp::string_range (with its temporaries, if it needs any, in a block in
// front, as for range over an integer), and gp::range_next steps from rune to rune, decoding
// UTF-8 like Go: an invalid byte is U+FFFD and steps one byte. The range variables are copies
// of its hidden counters, gp_i (where the rune starts) and gp_r (the rune), so assigning to
// them in the body doesn't change the iteration (Go 1.22). Every form is the same loop:
//
//	for i, r := range s {       for (auto gp_it0 = gp::range_string(s); gp::range_next(gp_it0);) {
//	                                int64_t i = gp_it0.gp_i; int32_t r = gp_it0.gp_r;
//
// A variable that exists (for i, r = range s) is assigned at the start of each iteration;
// a blank one, or one that is left out (for range s, for _, r := range s), is neither.
func (fg *funcGen) rangeString(s *ast.RangeStmt) {
	pre := fg.order(false, s.X)
	if len(pre) > 0 {
		fg.open(s.For)
		fg.add(&cppir.Line{Pos: fg.g.pos(s.X.Pos()), Stmts: pre})
	}
	fg.names.openScope() // the range variables and the body (see the top)
	it := fg.names.temp("it")
	head := &cppir.For{
		Pos:  fg.g.pos(s.For),
		Init: varDecl(cppir.Pos{}, typeName("auto"), it, call("gp::range_string", fg.expr(s.X))),
		Cond: call("gp::range_next", cxxName(it)),
	}
	// The line at the start of each iteration: a declaration or an assignment of each range
	// variable, made from its counter (as in Go, the key first), on one C++ line.
	type rangeVar struct {
		e       ast.Expr
		obj     *types.Var // a new variable
		typ     cppir.Type // its C++ type
		target  cppir.Expr // or a variable that exists
		counter string
	}
	var vars []rangeVar
	for _, v := range []rangeVar{{e: s.Key, counter: "gp_i"}, {e: s.Value, counter: "gp_r"}} {
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
		} else {
			v.target = fg.expr(v.e)
		}
		vars = append(vars, v)
	}
	l := loop{stmt: s, head: head, body: s.Body}
	if len(vars) > 0 {
		l.vars = func() {
			line := &cppir.Line{Pos: fg.g.pos(vars[0].e.Pos())}
			for _, v := range vars {
				value := &cppir.Member{X: cxxName(it), Name: v.counter}
				if v.obj != nil {
					line.Stmts = append(line.Stmts, varDecl(cppir.Pos{}, v.typ, fg.names.declare(v.obj), value))
				} else {
					line.Stmts = append(line.Stmts, &cppir.ExprStmt{X: &cppir.Assign{Op: "=", X: v.target, Y: value}})
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
// The tag goes into a temporary (gp_tag0) unless it is a constant or a local variable, which
// a case can't change in Phase 02 (Phase 03's closures can: the lowering pass will keep the
// temporary then). An init statement or temporaries go into a block first, { ... }; so do the
// temporaries of the tag's own order (order.go). Those of a case expression are lazy, like
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
		sw := &cppir.Switch{Pos: p, Tag: fg.expr(s.Tag)}
		fg.add(withLead(p, tagPre, sw))
		for _, c := range clauses {
			var values []cppir.Expr // nil: default
			for _, e := range c.List {
				values = append(values, fg.expr(e)) // a constant
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
		boolTag := s.Tag != nil && basicOf(fg.g.info.TypeOf(s.Tag)) != nil &&
			basicOf(fg.g.info.TypeOf(s.Tag)).Info()&types.IsBoolean != 0
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
	if b := basicOf(fg.g.info.TypeOf(s.Tag)); b == nil || b.Info()&types.IsInteger == 0 {
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
// variable), not a package variable.
func (fg *funcGen) isLocal(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	obj, ok := fg.g.info.Uses[id].(*types.Var)
	if !ok {
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
func (fg *funcGen) caseCond(tag cppir.Expr, boolTag bool, list []ast.Expr) ([]*cppir.Line, cppir.Expr) {
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
		} else if v := fg.constValue(e); boolTag && v != nil {
			// A bool tag and a constant case: b or !b, not b == true.
			parts[i] = tag
			if !constant.BoolVal(v) {
				parts[i] = &cppir.Unary{Op: "!", X: tag}
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
