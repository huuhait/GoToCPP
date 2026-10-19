package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Range over functions (Phase 07, step 7.8; docs/spec/cpp-mapping.md "Range over functions"). The
// lowering is a copy of Go's own rewrite (cmd/compile/internal/rangefunc/rewrite.go, whose
// comment at the top is the best description): the body of
//
//	for x := range seq { body }
//
// is the yield function that seq is called with. It is a normal closure, a lambda with an
// explicit capture list, never [&]: an iterator may keep the yield function and call it after the
// loop's function has returned (Go then panics: the state of the loop says so), and a by-reference
// lambda would read a dead stack frame. The variables of the function that the body assigns are
// boxed, as for any closure (box.go: the body is a boxRegion), and each loop has a state object,
// boxed too (runtime/include/gocpp/rangefunc.hpp):
//
//	{
//	    auto gp_rs0 = gp::new_<gp::rangefunc_state>();
//	    seq(gp::func<bool(int64_t)>([gp_rs0]<int = 0>(int64_t x) mutable -> bool {
//	        gp_rs0->gp_enter();                    // READY -> PANIC, or Go's misuse panic
//	        ...the body...
//	        return gp_rs0->gp_continue();          // the end of the body, and continue: READY, true
//	    }));
//	    gp_rs0->gp_finish();                       // PANIC means the body panicked and the
//	}                                              // iterator swallowed it; then EXHAUSTED
//
// Jumps (rewrite.go's rules, line by line: rfBranch, rfReturn, rfChecks):
//   - break: return gp_rs0->gp_exit() (state DONE, false); continue: return gp_rs0->gp_continue();
//   - return: the results are assigned (they are variables of the function, boxed: a named
//     result is one already, an unnamed one becomes one: Go does the same, which shows when a
//     deferred call recovers a panic of the iterator after the body returned), gp_next = -1 and
//     the loop exits; after the call, the code in the function that holds the outermost loop
//     does the return;
//   - break or continue of an outer range-over-func loop, a labeled break or continue of any
//     other statement and goto, to a place outside the body: gp_next is a code, the loop exits,
//     and the code after the call of each loop on the way either passes it on (return false) or,
//     in the function that holds the target, does the jump itself. gp_next lives in the state
//     object of the outermost loop of the nest (one per function, as Go's #next), which the
//     bodies of the loops inside it capture when they use it;
//   - goto to a label in the same body is a goto;
//   - defer in the body is the function's own defer: the body captures the function's frame,
//     gp_df, by reference (safe: the body runs only while the loop's state is READY, and then
//     the function is still running, so a late call finds its state DONE or EXHAUSTED and
//     panics before it touches anything);
//   - recover() in the body is nil, as in Go: the body is a closure that the iterator calls, not
//     the deferred function that a panic runs (even when the function that has the loop is that
//     deferred function). awareBody leaves bodies out, so a function whose only recover() is in
//     a body is not aware; a recover() in a func literal inside the body is that literal's own.
//
// A body lambda carries the template parameter <int = 0>: the stack trace leaves it out (a
// generic helper, trace.cpp), as it leaves out gocpp's other helpers.

// isRangeFunc reports whether the range statement s ranges over a func.
func isRangeFunc(info *tinfo, s *ast.RangeStmt) bool {
	_, ok := under(info.TypeOf(s.X)).(*types.Signature)
	return ok
}

// hasRangeFuncReturn reports whether the body of a function (or func literal) has a return
// statement with values in the body of a range over a func (not in a func literal in it): the
// function's results are then variables that the body's closure sets (signature).
func hasRangeFuncReturn(info *tinfo, body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	found := false
	depth := 0
	var stack []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			if rs, ok := stack[len(stack)-1].(*ast.RangeStmt); ok && isRangeFunc(info, rs) {
				depth--
			}
			stack = stack[:len(stack)-1]
			return true
		}
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.RangeStmt:
			if isRangeFunc(info, x) {
				depth++
			}
		case *ast.ReturnStmt:
			if depth > 0 && len(x.Results) > 0 {
				found = true
			}
		}
		stack = append(stack, n)
		return !found
	})
	return found
}

// hasValueReturn reports whether the body has a return statement with values (not in a func
// literal in it).
func hasValueReturn(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			found = found || len(x.Results) > 0
		}
		return !found
	})
	return found
}

// prepareRangeFuncResults is signature's part for a function with a return with values in the
// body of a range over a func: its results are variables the body's closure sets, boxed: the
// unnamed ones (and _) get names and boxes.
func (f *funcInfo) prepareRangeFuncResults(g *gen) {
	res := f.sig.Results()
	if res.Len() == 0 || !hasRangeFuncReturn(g.info, f.body) {
		return
	}
	if f.results == nil {
		f.results = make([]string, res.Len())
		for i := range f.results {
			f.results[i] = f.names.temp("r")
		}
	}
	for i := range res.Len() {
		r := res.At(i)
		if f.boxed[r] == 0 {
			f.boxed[r] = boxCaptured
		}
		if n := r.Name(); n != "" && n != "_" {
			// The closures capture the result by its C++ name, which a variable of the function
			// with the same Go name (for r := range seq) would hide: a name of ours.
			f.results[i] = f.names.temp("r")
			f.names.names[r] = f.results[i]
		}
	}
}

// An rfTarget is a target of a range clause with =, and the parameter that holds its value.
type rfTarget struct {
	e    ast.Expr
	name string
	from types.Type // the parameter's type
}

// An rfNest holds what the range-over-func loops of one function (or func literal) need across
// their bodies, which are lowered by funcGens of their own (rfBody). Its loops stack is the loops
// whose bodies are being lowered, outermost first.
type rfNest struct {
	fn    *funcInfo // the function
	loops []*rfLoop
	codes map[rfBranch]int // the gp_next code of each jump to a label, from -2 down
}

// An rfBranch is a jump to a label that the loops of a nest must leave first.
type rfBranch struct {
	tok    token.Token
	label  string
	target ast.Stmt // the labeled statement of a break or continue, nil for a goto
}

// An rfLoop is a range-over-func loop that is being lowered (Go's forLoop).
type rfLoop struct {
	stmt  *ast.RangeStmt
	state string // the C++ name of its state box

	// capNext: the closure of its body uses gp_next of the outermost loop's state, so it
	// captures that box.
	capNext bool

	// What the code after its call must check (Go's checkRet, checkBreak, checkContinue and
	// checkBranch).
	checkRet, checkBreak, checkContinue bool
	checkBranch                         []rfBranch
}

// inRFBody reports whether fg lowers the body of a range over a func.
func (fg *funcGen) inRFBody() bool {
	return fg.rf != nil && fg.fn != fg.rf.fn
}

// code returns the gp_next code of the jump b: -1 is a return, the others -2, -3, ...
func (rf *rfNest) code(b rfBranch) int {
	if c, ok := rf.codes[b]; ok {
		return c
	}
	if rf.codes == nil {
		rf.codes = make(map[rfBranch]int)
	}
	c := -2 - len(rf.codes)
	rf.codes[b] = c
	return c
}

// stateCall returns state->method(args...), a call of a member of a state box.
func stateCall(state, method string, args ...cppir.Expr) cppir.Expr {
	return &cppir.Call{Fun: &cppir.Member{X: cxxName(state), Arrow: true, Name: method}, Args: args}
}

// setNext returns the statement that sets gp_next of the nest to v. depth is the index of the
// loop whose body the statement is in (-1: in the function): the closures between the outermost
// loop and it capture the outermost state.
func (fg *funcGen) setNext(pos cppir.Pos, depth, v int) cppir.Stmt {
	return &cppir.ExprStmt{Pos: pos, X: &cppir.Assign{Op: "=", X: fg.nextRef(depth), Y: lit(strconv.Itoa(v))}}
}

// nextRef returns the C++ of gp_next of the nest, as seen from the body of the loop with index
// depth (-1: from the function, after the loop of index 0 has been popped: its state is named
// by the loop that the caller passes, see rfChecks).
func (fg *funcGen) nextRef(depth int) cppir.Expr {
	rf := fg.rf
	for i := 1; i <= depth && i < len(rf.loops); i++ {
		rf.loops[i].capNext = true
	}
	return &cppir.Member{X: cxxName(rf.loops[0].state), Arrow: true, Name: "gp_next"}
}

// rangeFunc lowers a range over a func (see the top of the file).
func (fg *funcGen) rangeFunc(s *ast.RangeStmt) {
	g := fg.g
	if fg.fn == nil {
		g.unsupported(s.Pos(), "range over functions here")
		return
	}
	if fg.rf == nil {
		fg.rf = &rfNest{fn: fg.fn}
	}
	rf := fg.rf
	rsig := under(g.info.TypeOf(s.X)).(*types.Signature)
	if rsig.Params().Len() != 1 {
		g.errorf(s.X.Pos(), "internal error: a range over a func without a yield parameter")
		return
	}
	yieldType := rsig.Params().At(0).Type()
	ysig, _ := under(yieldType).(*types.Signature)
	if ysig == nil {
		g.errorf(s.X.Pos(), "internal error: a range over a func whose parameter is not a func")
		return
	}
	pos := g.pos(s.For)
	pre := fg.order(false, s.X)
	fg.open(s.For)
	if len(pre) > 0 {
		fg.add(&cppir.Line{Pos: g.pos(s.X.Pos()), Stmts: pre})
	}
	depth := len(rf.loops)
	loop := &rfLoop{stmt: s, state: fg.names.temp("rs")}

	var callee cppir.Expr = fg.expr(s.X)
	if !fg.stableCallee(s.X) {
		callee = &cppir.Member{X: callee, Name: "gp_call"}
	}
	rf.loops = append(rf.loops, loop)
	lam := fg.rfBody(s, loop, ysig, depth)
	rf.loops = rf.loops[:depth]

	fg.add(&cppir.Line{Pos: pos, Stmts: []cppir.Stmt{
		varDecl(cppir.Pos{}, typeName("auto"), loop.state, newBox(typeName("gp::rangefunc_state"), nil)),
		&cppir.ExprStmt{X: &cppir.Call{Fun: callee, Args: []cppir.Expr{construct(g.cxxType(yieldType, s.X.Pos()), lam)}}},
	}})
	end := g.pos(s.Body.Rbrace)
	fg.add(&cppir.ExprStmt{Pos: end, X: stateCall(loop.state, "gp_finish")})
	fg.rfChecks(loop, depth, end, s.For)
	fg.close(s.Body.Rbrace)
}

// rfBody returns the lambda that is the body of the loop s (its closure over the function), whose
// statements are lowered by a funcGen of its own. depth is the number of loops around s in the
// function; loop is on the nest's stack.
func (fg *funcGen) rfBody(s *ast.RangeStmt, loop *rfLoop, ysig *types.Signature, depth int) *cppir.Lambda {
	g, rf := fg.g, fg.rf
	f := &funcInfo{sig: ysig, body: s.Body, pos: s.For, top: fg.top, names: fg.names, boxed: fg.boxed}
	lam := &cppir.Lambda{
		Template: []cppir.Param{{Type: typeName("int"), Default: lit("0")}},
		Specs:    []string{"mutable"},
		Result:   typeName("bool"),
		Body:     &cppir.Block{Inline: g.fset.Position(s.For).Line == g.fset.Position(s.Body.Rbrace).Line},
	}
	fg.names.openScope() // the range variables, as the lambda's parameters
	// The parameters: a range variable of :=, else a name of ours (the assignment of = comes
	// first in the body); a variable that is not there is an unnamed parameter.
	var assigns []rfTarget
	vars := []ast.Expr{s.Key, s.Value}
	for i := range ysig.Params().Len() {
		p := ysig.Params().At(i)
		typ := g.varType(p, s.For)
		param := cppir.Param{Type: typ}
		var e ast.Expr
		if i < len(vars) && vars[i] != nil && !isBlank(vars[i]) {
			e = vars[i]
		}
		switch {
		case e == nil:
		case s.Tok == token.DEFINE:
			v, _ := g.info.Defs[e.(*ast.Ident)].(*types.Var)
			if v == nil {
				g.errorf(e.Pos(), "internal error: a range variable without an object")
				break
			}
			if largeVar(p.Type()) && f.boxed[v] == 0 {
				f.boxed[v] = boxLarge // as signature does for a parameter
			}
			if e := g.sigTypes()[p]; e != nil {
				g.sigTypes()[v] = e // a C/C++ type, written in the yield parameter: the variable's too (paramBoxes)
			}
			param.Name = fg.names.declare(v)
			if f.boxed[v] != 0 {
				f.args = append(f.args, v)
			}
		default:
			param.Name = fg.names.temp("p")
			assigns = append(assigns, rfTarget{e, param.Name, p.Type()})
		}
		if largeVar(p.Type()) {
			param.Type = &cppir.Ref{Elem: &cppir.Const{Elem: param.Type}}
		}
		lam.Params = append(lam.Params, param)
	}
	child := newFuncGen(g, fg.names, f)
	child.rf, child.flow = rf, fg.flow
	child.push(&lam.Body.Stmts, &lam.Body.End)
	child.fresh = true
	child.add(&cppir.ExprStmt{Pos: g.pos(s.For), X: stateCall(loop.state, "gp_enter")})
	if len(assigns) > 0 {
		child.rfAssign(assigns)
	}
	boxes := child.paramBoxes()
	child.stmts(s.Body.List, s.Body.Rbrace)
	if boxes {
		child.names.closeScope()
		child.pop(s.Body.Rbrace)
	}
	child.add(&cppir.Return{Pos: g.pos(s.Body.Rbrace), X: stateCall(loop.state, "gp_continue")})
	child.pop(s.Body.Rbrace)
	fg.names.closeScope()

	// The captures: what the body uses of the function, its own state, and the function's frame,
	// ticket and results when the body (or a loop in it) uses them.
	nodes := []ast.Node{s.Body}
	for _, e := range vars {
		if e != nil && s.Tok == token.ASSIGN {
			nodes = append(nodes, e)
		}
	}
	lam.Captures = fg.captureNames(nodes, func(pos token.Pos) bool { return s.Pos() <= pos && pos < s.End() })
	seen := make(map[string]bool)
	for _, c := range lam.Captures {
		seen[c.Name] = true
	}
	capture := func(name string, byRef bool) {
		if !seen[name] {
			seen[name] = true
			lam.Captures = append(lam.Captures, cppir.Capture{Name: name, ByRef: byRef})
		}
	}
	capture(loop.state, false)
	if loop.capNext && depth > 0 {
		capture(rf.loops[0].state, false)
	}
	if hasDefer(s.Body) {
		capture(deferFrame, true)
	}
	if hasValueReturn(s.Body) {
		for _, name := range rf.fn.results {
			capture(name, false)
		}
	}
	return lam
}

// rfAssign adds the assignment of the range clause with = at the start of the body: the targets
// get the parameters' values, in order, as rangeIter does for the other kinds of range.
func (fg *funcGen) rfAssign(assigns []rfTarget) {
	g := fg.g
	line := &cppir.Line{Pos: g.pos(assigns[0].e.Pos())}
	lhs := make([]ast.Expr, len(assigns))
	for i, a := range assigns {
		lhs[i] = a.e
	}
	line.Stmts = append(line.Stmts, fg.order(false, fg.targetOperands(lhs)...)...)
	targets := make([]target, len(lhs))
	for i, e := range lhs {
		targets[i] = fg.lhs(e)
	}
	for i := range targets {
		line.Stmts = append(line.Stmts, fg.saveOperands(cppir.Pos{}, targets, i)...)
	}
	for i, t := range targets {
		value := g.convertTo(assigns[i].from, g.info.TypeOf(lhs[i]), cxxName(assigns[i].name), lhs[i].Pos())
		line.Stmts = append(line.Stmts, &cppir.ExprStmt{X: t.assignExpr(value)})
	}
	fg.add(line)
}

// rfReturn lowers a return statement in the body of a range over a func: it sets the function's
// results, then leaves every loop of the nest, and the function returns after the call of the
// outermost loop (rfChecks).
func (fg *funcGen) rfReturn(s *ast.ReturnStmt) {
	rf := fg.rf
	k := len(rf.loops) - 1
	p := fg.g.pos(s.Pos())
	var lines []cppir.Stmt
	if len(s.Results) > 0 {
		outer := rf.fn
		res := outer.sig.Results()
		targets := make([]target, len(outer.results))
		for i, n := range outer.results {
			targets[i] = varTarget(n)
			targets[i].gtype = res.At(i).Type() // which return f() converts to (step 3.9)
			targets[i].gcMem = true             // a box: other code can read it
			if fg.isBoxed(res.At(i)) {
				targets[i].x = &cppir.Paren{X: deref(cxxName(n))}
			}
		}
		fg.push(&lines, nil)
		fg.bind(s.Pos(), targets, nil, s.Results, true)
		fg.pop(s.Pos())
		if len(lines) == 1 {
			if l, ok := lines[0].(*cppir.Line); ok {
				lines = l.Stmts
			}
		}
	}
	for _, l := range rf.loops {
		l.checkRet = true
	}
	lines = append(lines, fg.setNext(p, k, -1), &cppir.Return{Pos: p, X: stateCall(rf.loops[k].state, "gp_exit")})
	fg.add(&cppir.Line{Pos: p, Stmts: lines})
}

// rfBranch lowers the break, continue or goto s in the body of a range over a func if it leaves
// the body: it reports whether it did. A jump inside the body is left to branchStmt.
func (fg *funcGen) rfBranch(s *ast.BranchStmt) bool {
	rf := fg.rf
	k := len(rf.loops) - 1
	p := fg.g.pos(s.Pos())
	var t ast.Stmt // the statement of a break or continue
	var at token.Pos
	var label string
	switch s.Tok {
	case token.FALLTHROUGH:
		return false
	case token.GOTO:
		l, _ := fg.g.info.Uses[s.Label].(*types.Label)
		if l == nil {
			return false
		}
		at, label = l.Pos(), s.Label.Name
	default:
		t = fg.flow.target[s]
		if t == nil {
			return false
		}
		at = t.Pos()
		if s.Label != nil {
			label = s.Label.Name
		}
	}
	// A break or continue of a loop of the nest.
	for i := k; i >= 0 && t != nil; i-- {
		if t != ast.Stmt(rf.loops[i].stmt) {
			continue
		}
		if i == k {
			// The simple case: break is DONE and false, continue is READY and true.
			method := "gp_exit"
			if s.Tok == token.CONTINUE {
				method = "gp_continue"
			}
			fg.add(&cppir.Return{Pos: p, X: stateCall(rf.loops[k].state, method)})
			return true
		}
		// A labeled break or continue of an outer loop of the nest: "do N breaks and 1
		// continue" (two codes for each loop: 2(i+1) breaks out of it, one less continues it).
		next := 2 * (i + 1)
		if s.Tok == token.CONTINUE {
			next--
			rf.loops[i+1].checkContinue = true
			for j := i + 2; j <= k; j++ {
				rf.loops[j].checkBreak = true
			}
		} else {
			for j := i + 1; j <= k; j++ {
				rf.loops[j].checkBreak = true
			}
		}
		fg.add(&cppir.Line{Pos: p, Stmts: []cppir.Stmt{fg.setNext(p, k, next), &cppir.Return{Pos: p, X: stateCall(rf.loops[k].state, "gp_exit")}}})
		return true
	}
	// The innermost loop of the nest that the statement is in.
	targ := -1
	for i := k; i >= 0; i-- {
		b := rf.loops[i].stmt.Body
		if b.Lbrace < at && at < b.Rbrace {
			targ = i
			break
		}
	}
	if targ == k {
		return false // in this body: the C++ statement does it
	}
	br := rfBranch{tok: s.Tok, label: label, target: t}
	// The loop inside the one that the label is in checks for the code after its call; the loops
	// inside that one pass it on.
	exit := rf.loops[targ+1]
	found := false
	for _, b := range exit.checkBranch {
		found = found || b == br
	}
	if !found {
		exit.checkBranch = append(exit.checkBranch, br)
	}
	for j := targ + 2; j <= k; j++ {
		rf.loops[j].checkRet = true
	}
	fg.add(&cppir.Line{Pos: p, Stmts: []cppir.Stmt{fg.setNext(p, k, rf.code(br)), &cppir.Return{Pos: p, X: stateCall(rf.loops[k].state, "gp_exit")}}})
	return true
}

// rfChecks adds the code after the call of the loop (index j in the function: the loops 0..j-1
// are open, and fg lowers the body of j-1, or the function itself for j = 0): Go's checks of
// gp_next. at is the place of the loop's for.
func (fg *funcGen) rfChecks(loop *rfLoop, j int, end cppir.Pos, at token.Pos) {
	rf := fg.rf
	if !loop.checkRet && !loop.checkBreak && !loop.checkContinue && len(loop.checkBranch) == 0 {
		return
	}
	// gp_next is in the outermost loop's state (this loop's, when it is the outermost).
	outer := loop.state
	if j > 0 {
		outer = rf.loops[0].state
		fg.nextRef(j - 1) // the closures between capture it
	}
	var next cppir.Expr = &cppir.Member{X: cxxName(outer), Arrow: true, Name: "gp_next"}
	num := func(v int) cppir.Expr {
		if v < 0 {
			return &cppir.Unary{Op: "-", X: lit(strconv.Itoa(-v))}
		}
		return lit(strconv.Itoa(v))
	}
	zero := &cppir.ExprStmt{Pos: end, X: &cppir.Assign{Op: "=", X: next, Y: lit("0")}}
	ifNext := func(op string, v int, then ...cppir.Stmt) {
		fg.add(&cppir.If{Pos: end, Cond: &cppir.Binary{Op: op, X: next, Y: num(v)}, Then: &cppir.Block{Stmts: then, Inline: true}})
	}
	ret := func(state string, method string) cppir.Stmt {
		return &cppir.Return{Pos: end, X: stateCall(state, method)}
	}
	for _, br := range loop.checkBranch {
		// The jump itself, from here: a goto (break and continue of a loop or switch that is
		// not a loop of the nest go to the labels that flow.go gave them).
		var jump cppir.Stmt
		switch {
		case br.tok == token.GOTO:
			jump = &cppir.Goto{Pos: end, Label: fg.g.escape(br.label)}
		case br.tok == token.BREAK:
			jump = &cppir.Goto{Pos: end, Label: fg.flow.labelName(br.target, "break")}
		default:
			jump = &cppir.Goto{Pos: end, Label: fg.contLabels[br.target]}
		}
		ifNext("==", rf.code(br), zero, jump)
	}
	if j == 0 {
		if loop.checkRet {
			// return, in the function itself.
			blk := &cppir.Block{}
			fg.push(&blk.Stmts, nil)
			fg.returnStmt(&ast.ReturnStmt{Return: at})
			fg.pop(at)
			fg.add(&cppir.If{Pos: end, Cond: &cppir.Binary{Op: "==", X: next, Y: num(-1)}, Then: blk})
		}
		return
	}
	// In the body of the loop j-1.
	up := rf.loops[j-1].state
	if loop.checkRet {
		ifNext("<", 0, ret(up, "gp_exit")) // return, goto and the like: leave this loop too
	}
	step := 2 * j
	ifNext(">", step, &cppir.ExprStmt{Pos: end, X: call("gp::panic_range_state", lit("0"))})
	if loop.checkContinue {
		ifNext("==", step-1, zero, ret(up, "gp_continue"))
	}
	if loop.checkBreak {
		ifNext("==", step, zero, ret(up, "gp_exit"))
	}
	if loop.checkContinue || loop.checkBreak {
		ifNext(">", 0, ret(up, "gp_exit"))
	}
}
