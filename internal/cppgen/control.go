package cppgen

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// if, for, range and switch (Phase 02 README §2.4).
//
//	Go                          C++
//	if x := f(); x > 0 {...}    { int64_t x = f(); if (x > 0) {...} }  (Go's scope of x)
//	} else if c {...}           } else if (c) {...}
//	} else if y := g(); c {     } else { int64_t y = g(); if (c) {...} }
//	for {...}                   for (;;) {...}
//	for c {...}                 while (c) {...}
//	for i := 0; i < n; i++ {    for (int64_t i = 0; i < n; i++) {
//	for ...; i, j = i+1, j-1 {  for (...;) { {body} gp_cont_0:; { auto gp_t0 = i + 1; ... } }
//	for i := range n {...}      for (int64_t gp_i0 = 0, gp_n0 = n; gp_i0 < gp_n0; gp_i0++) {
//	                                int64_t i = gp_i0; ...
//	switch (3 shapes)           see switchStmt
//
// Scopes: a C++ for doesn't allow a name of its init statement to be declared again in the
// outermost block of its body (Go does: the body is a block of its own). So the init and the
// body are one scope of funcNames, which gives a body variable of the same name a new name
// (i_1). The range variable is declared in the body, the same way.

// cond returns the C++ of a condition: of an if or an else if, a loop, a case of a switch.
// Step 2.5 gives a condition with temporaries its own shape here (in a loop, a block inside
// the loop; in an else if, a nested if, as for an else if with an init statement).
func (fg *funcGen) cond(e ast.Expr) cexpr {
	return fg.expr(e)
}

// An arm is one branch of an if-else chain (chain).
type arm struct {
	pos  token.Pos     // where its if (or its else) is
	init ast.Stmt      // the statement before the condition, or nil
	cond func() string // returns the C++ of the condition; nil for the final else
	body func()        // writes the body
}

// chain writes an if-else chain of the arms, the last of which may be an else. It
// translates each condition right before its if line, so the conditions are translated in
// the order they run. An arm with an init statement starts a block, the scope of the init,
// that holds the rest of the chain: { init; if (c) ... }, or } else { init; if (c) ... } in
// the middle. end is the place of the closing brace.
func (fg *funcGen) chain(arms []arm, end token.Pos) {
	blocks := 0
	for i, a := range arms {
		head := "} else if ("
		if i == 0 {
			head = "if ("
		}
		if a.init != nil {
			if i == 0 {
				fg.line(a.pos, "{")
			} else {
				fg.line(a.pos, "} else {")
			}
			fg.g.w.indent()
			fg.names.openScope()
			blocks++
			fg.stmt(a.init)
			head = "if ("
		}
		switch {
		case a.cond != nil:
			fg.line(a.pos, head+a.cond()+") {")
		case i == 0:
			fg.line(a.pos, "{") // only an else: a switch with only a default
		default:
			fg.line(a.pos, "} else {")
		}
		fg.inner(a.body)
	}
	fg.line(end, "}")
	for range blocks {
		fg.names.closeScope()
		fg.g.w.dedent()
		fg.line(end, "}")
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
			cond: func() string { return fg.cond(cond).text },
			body: func() { fg.stmts(body.List) },
		})
		end = body.Rbrace
		next := s.Else
		s = nil
		switch e := next.(type) {
		case *ast.IfStmt:
			s = e
		case *ast.BlockStmt:
			arms = append(arms, arm{pos: e.Lbrace, body: func() { fg.stmts(e.List) }})
			end = e.Rbrace
		}
	}
	fg.chain(arms, end)
}

// A loop is what loopBody needs to know about a for or range statement.
type loop struct {
	stmt ast.Stmt
	pos  token.Pos      // the for keyword
	head string         // the C++ head: for (...) {, while (c) {
	vars func()         // writes the declarations at the start of each iteration, or nil
	body *ast.BlockStmt // the Go body
	post ast.Stmt       // a post statement that is not one C++ expression, or nil
}

// loopBody writes a loop from its head to its closing brace, and the label after it that a
// break L needs (flow.go). The body is the C++ loop's own block, or, if a goto goes to the
// end of the body (continue), a block inside it, followed by the label and the post
// statement: for (...) { { body } gp_cont_0:; post; }.
func (fg *funcGen) loopBody(l loop) {
	shaped := fg.flow.contLabel[l.stmt] || l.post != nil
	if fg.flow.contLabel[l.stmt] {
		if fg.flow.label[l.stmt] != nil {
			fg.contLabels[l.stmt] = fg.flow.labelName(l.stmt, "continue")
		} else {
			fg.contLabels[l.stmt] = fg.names.temp("cont_")
		}
	}
	fg.line(l.pos, l.head)
	fg.g.w.indent()
	if l.vars != nil {
		l.vars()
	}
	if shaped {
		fg.block(l.body)
		if label := fg.contLabels[l.stmt]; label != "" {
			fg.line(l.body.Rbrace, label+":;")
		}
		if l.post != nil {
			fg.stmt(l.post)
		}
	} else {
		fg.stmts(l.body.List)
	}
	fg.g.w.dedent()
	fg.line(l.body.Rbrace, "}")
}

// breakLabel writes the label after the statement s that a break L goes to, if one does.
func (fg *funcGen) breakLabel(s ast.Stmt, end token.Pos) {
	if fg.flow.breakLabel[s] {
		fg.line(end, fg.flow.labelName(s, "break")+":;")
	}
}

// open starts a block, a scope, at pos: { (for an init statement or temporaries).
func (fg *funcGen) open(pos token.Pos) {
	fg.line(pos, "{")
	fg.g.w.indent()
	fg.names.openScope()
}

// close ends the block of open.
func (fg *funcGen) close(pos token.Pos) {
	fg.names.closeScope()
	fg.g.w.dedent()
	fg.line(pos, "}")
}

func (fg *funcGen) forStmt(s *ast.ForStmt) {
	end := s.Body.Rbrace
	fg.names.openScope() // the init and the body (see the top)
	wrapped := false
	init := ""
	if s.Init != nil {
		if text, ok := fg.forInit(s.Init); ok {
			init = text
		} else {
			// { a, b := f(); for (; ...) {...} }
			fg.line(s.For, "{")
			fg.g.w.indent()
			wrapped = true
			fg.stmt(s.Init)
		}
	}
	cond := ""
	if s.Cond != nil {
		cond = fg.cond(s.Cond).text
	}
	l := loop{stmt: s, pos: s.For, body: s.Body}
	post := ""
	if s.Post != nil {
		if oneExpr(s.Post) {
			post = fg.simpleExpr(s.Post)
		} else {
			l.post = s.Post
		}
	}
	switch {
	case init == "" && post == "" && cond == "":
		l.head = "for (;;) {"
	case init == "" && post == "":
		l.head = "while (" + cond + ") {"
	default:
		l.head = "for (" + init + ";"
		if cond != "" {
			l.head += " " + cond
		}
		l.head += ";"
		if post != "" {
			l.head += " " + post
		}
		l.head += ") {"
	}
	fg.loopBody(l)
	if wrapped {
		fg.g.w.dedent()
		fg.line(end, "}")
	}
	fg.names.closeScope()
	fg.breakLabel(s, end)
}

// forInit returns the C++ init statement of a 3-clause for, without the ;, if the Go init
// statement is one: a simple statement that is one C++ expression, or a := whose values all
// go into new variables of one C++ type (int64_t i = 0, j = n). It returns false for the
// others, which go before the loop.
func (fg *funcGen) forInit(s ast.Stmt) (string, bool) {
	if oneExpr(s) {
		return fg.simpleExpr(s), true
	}
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.DEFINE || len(a.Lhs) != len(a.Rhs) {
		return "", false
	}
	typ := ""
	for _, e := range a.Lhs {
		obj, ok := fg.g.info.Defs[e.(*ast.Ident)].(*types.Var)
		if !ok || isBlank(e) {
			return "", false // _ or a variable that exists
		}
		t := fg.g.cxxType(obj.Type(), e.Pos())
		if typ != "" && t != typ {
			return "", false
		}
		typ = t
	}
	targets := fg.newTargets(a.Lhs, a.Rhs)
	parts := make([]string, len(targets))
	for i, t := range targets {
		parts[i] = t.name + " = " + fg.expr(a.Rhs[i]).text
	}
	return typ + " " + strings.Join(parts, ", "), true
}

// rangeStmt writes a range loop. Phase 02 has range over an integer n: a hidden counter goes
// from 0 to n-1, n is evaluated once, and the range variable is a copy of the counter, so
// assigning to it in the body doesn't change the iteration (Go 1.22). A variable that exists
// (for i = range n) is assigned the counter at the start of each iteration.
func (fg *funcGen) rangeStmt(s *ast.RangeStmt) {
	xt := fg.g.info.TypeOf(s.X)
	b := basicOf(xt)
	if b == nil || b.Info()&types.IsInteger == 0 {
		fg.g.unsupported(s.X.Pos(), rangeKind(xt))
		return
	}
	// The type of the counter (the spec, "For statements with range clause"): n's type, or
	// for an untyped constant n the type of a variable that exists, or else int.
	t := xt
	var key *types.Var // a new range variable
	assigned := ""     // the C++ of a variable that exists
	if s.Key != nil && !isBlank(s.Key) {
		if s.Tok == token.DEFINE {
			key, _ = fg.g.info.Defs[s.Key.(*ast.Ident)].(*types.Var)
		} else {
			assigned = fg.expr(s.Key).text
		}
	}
	if b.Info()&types.IsUntyped != 0 {
		switch {
		case key != nil:
			t = key.Type()
		case assigned != "":
			t = fg.g.info.TypeOf(s.Key)
		default:
			t = types.Typ[types.Int]
		}
	}
	ct := fg.g.cxxType(t, s.X.Pos())

	fg.names.openScope() // the range variable and the body (see the top)
	i := fg.names.temp("i")
	head := "for (" + ct + " " + i + " = 0"
	if v := fg.constValue(s.X); v != nil {
		head += "; " + i + " < " + fg.g.constant(v, t, s.X.Pos(), false).at(precShift)
	} else {
		n := fg.names.temp("n")
		head += ", " + n + " = " + fg.expr(s.X).text + "; " + i + " < " + n
	}
	head += "; " + i + "++) {"
	l := loop{stmt: s, pos: s.For, head: head, body: s.Body}
	switch {
	case key != nil:
		l.vars = func() { fg.line(s.Key.Pos(), ct+" "+fg.names.declare(key, nil)+" = "+i+";") }
	case assigned != "":
		l.vars = func() { fg.line(s.Key.Pos(), assigned+" = "+i+";") }
	}
	fg.loopBody(l)
	fg.names.closeScope()
	fg.breakLabel(s, s.Body.Rbrace)
}

// rangeKind names what a range over a value of type t is, for "not supported yet".
func rangeKind(t types.Type) string {
	if b := basicOf(t); b != nil && b.Info()&types.IsString != 0 {
		return "range over strings (Phase 03)"
	}
	switch types.Unalias(t).Underlying().(type) {
	case *types.Signature:
		return "range over functions (Phase 07)"
	case *types.Chan:
		return "range over channels (Phase 08)"
	}
	return "range over " + types.TypeString(t, nil) + " (Phase 03)"
}

// switchStmt writes a switch statement. Go evaluates the tag once, then the case expressions
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
// temporary then). An init statement or temporaries go into a block first, { ... }. In the
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
	if s.Init != nil {
		wrap()
		fg.stmt(s.Init)
	}
	clauses := make([]*ast.CaseClause, len(s.Body.List))
	cases := false // a clause other than default
	for i, c := range s.Body.List {
		clauses[i] = c.(*ast.CaseClause)
		cases = cases || clauses[i].List != nil
	}

	if fg.integerSwitch(s, clauses) {
		fg.line(s.Switch, "switch ("+fg.expr(s.Tag).text+") {")
		for _, c := range clauses {
			label := "default:"
			if c.List != nil {
				labels := make([]string, len(c.List))
				for i, e := range c.List {
					labels[i] = "case " + fg.expr(e).text + ":"
				}
				label = strings.Join(labels, " ")
			}
			fg.caseBody(c, label)
		}
		fg.line(end, "}")
	} else {
		tag := ""
		if s.Tag != nil {
			switch {
			case fg.constValue(s.Tag) != nil || fg.isLocal(s.Tag):
				tag = fg.expr(s.Tag).at(precShift)
			case !cases:
				fg.line(s.Tag.Pos(), "static_cast<void>("+fg.expr(s.Tag).text+");")
			default:
				wrap()
				tag = fg.names.temp("tag")
				fg.line(s.Tag.Pos(), "auto "+tag+" = "+fg.exactExpr(s.Tag).text+";")
			}
		}
		boolTag := s.Tag != nil && basicOf(fg.g.info.TypeOf(s.Tag)) != nil &&
			basicOf(fg.g.info.TypeOf(s.Tag)).Info()&types.IsBoolean != 0
		caseCond := func(c *ast.CaseClause) func() string {
			return func() string { return fg.caseCond(tag, boolTag, c.List) }
		}
		var arms []arm
		var def *ast.CaseClause
		if !fallsThrough(clauses) && !fg.flow.breakTo[s] {
			for _, c := range clauses {
				if c.List == nil {
					def = c
					continue
				}
				arms = append(arms, arm{pos: c.Case, cond: caseCond(c), body: func() { fg.stmts(c.Body) }})
			}
			if def != nil {
				arms = append(arms, arm{pos: def.Case, body: func() { fg.stmts(def.Body) }})
			}
			if arms != nil {
				fg.chain(arms, end)
			}
		} else {
			wrap()
			index := fg.names.temp("case")
			fg.line(s.Switch, "int "+index+" = -1;")
			n := 0
			labels := make([]string, len(clauses))
			for i, c := range clauses {
				if c.List == nil {
					labels[i] = "default:"
					continue
				}
				k := strconv.Itoa(n)
				n++
				labels[i] = "case " + k + ":"
				arms = append(arms, arm{pos: c.Case, cond: caseCond(c), body: func() { fg.line(c.Case, index+" = "+k+";") }})
			}
			if arms != nil {
				fg.chain(arms, arms[len(arms)-1].pos)
			}
			fg.line(s.Switch, "switch ("+index+") {")
			for i, c := range clauses {
				fg.caseBody(c, labels[i])
			}
			fg.line(end, "}")
		}
	}
	if wrapped {
		fg.close(end)
	}
	fg.breakLabel(s, end)
}

// integerSwitch reports whether the switch s can be a C++ switch on its tag (shape 1): an
// integer tag, and only constant case expressions.
func (fg *funcGen) integerSwitch(s *ast.SwitchStmt, clauses []*ast.CaseClause) bool {
	if s.Tag == nil {
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
// with the tag (the C++ of the tag, "" for a switch without one), joined with ||, which
// evaluates them in order and only until one matches, as Go does.
func (fg *funcGen) caseCond(tag string, boolTag bool, list []ast.Expr) string {
	parts := make([]string, len(list))
	for i, e := range list {
		if tag == "" {
			c := fg.cond(e)
			if len(list) > 1 {
				parts[i] = c.at(precBitOr) // (a && b) || c: g++ -Wall asks for them
			} else {
				parts[i] = c.text
			}
		} else if v := fg.constValue(e); boolTag && v != nil {
			// A bool tag and a constant case: b or !b, not b == true.
			parts[i] = tag
			if !constant.BoolVal(v) {
				parts[i] = "!" + tag
			}
		} else {
			parts[i] = tag + " == " + fg.expr(e).at(precShift)
		}
	}
	return strings.Join(parts, " || ")
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

// caseBody writes a clause of a C++ switch: its labels, and its body as a block, which ends
// with break unless it ends with a jump (fallthrough falls into the next case).
func (fg *funcGen) caseBody(c *ast.CaseClause, label string) {
	fg.line(c.Case, label+" {")
	fg.inner(func() {
		fg.stmts(c.Body)
		if !endsWithJump(c) {
			fg.line(clauseEnd(c), "break;")
		}
	})
	fg.line(clauseEnd(c), "}")
}

// clauseEnd returns the place of the end of a clause: its last statement, or the colon.
func clauseEnd(c *ast.CaseClause) token.Pos {
	if len(c.Body) == 0 {
		return c.Colon
	}
	return c.Body[len(c.Body)-1].End()
}
