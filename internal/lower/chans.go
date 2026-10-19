package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Channels (Phase 08 steps 8.3 and 8.4; the runtime is gocpp/chan.hpp):
//
//	Go                       C++
//	chan T, <-chan T,        gp::chan<T>  (one C++ type for the three directions; the Go types
//	chan<- T                 differ in their type descriptors, typedesc.go, and in what the checker
//	                         accepts)
//	var ch chan T, nil       gp::chan<T> ch{};  ch = nullptr  (the nil channel)
//	ch == nil, ch == ch2     ch == nullptr, ch == ch2  (the same channel object)
//	make(chan T)             gp::make_chan<T>()
//	make(chan T, n)          gp::make_chan<T>(n)
//	ch <- v                  gp::send(ch, v);
//	<-ch                     gp::recv(ch)
//	<-ch as a statement      gp::recv_drop(ch);
//	v, ok := <-ch            auto gp_t0 = gp::recv2(ch); T v = std::get<0>(gp_t0); bool ok = ...
//	close(ch)                gp::close(ch)
//	len(ch), cap(ch)         gp::len(ch), gp::cap(ch)
//	type Pipe chan int       struct Pipe : gp::newtype<Pipe, gp::chan<int64_t>> (named.go)
//	for v := range ch        for (auto gp_it0 = gp::range_chan(ch); gp::range_next(gp_it0);) { T v = gp_it0.gp_v(); ...
//	                         (control.go: rangeIter; gp::range_chan_drop for a loop without a variable)
//	select { ... }           a block of case objects and a switch on gp::select(...): see selectStmt below
//
// A send or a receive that blocks parks the goroutine's thread on its waiter (ADR-007), so they are
// calls for the order of evaluation: gc moves a receive into a temporary in its place among the
// calls (walk/order.go, ORECV), and `ch <- v` evaluates the channel and then the value before it
// communicates (order.go collects both as the arguments of a call). The value of a send is
// converted to the element type like an argument (named.go: noteConversions), and a C/C++ value is
// copied or moved into the channel as it is into a deferred call (deferArg, cbox.go).

// chanType returns gp::chan<elem>, the C++ type of a channel of elem.
func chanType(elem cppir.Type) cppir.Type {
	return &cppir.TypeName{Name: "gp::chan", Args: []cppir.Node{elem}}
}

// isChan reports whether t is a channel type (through names and aliases: type Pipe chan int).
func isChan(t types.Type) bool {
	_, ok := under(t).(*types.Chan)
	return ok
}

// chanDir returns gp::chan_dir::both, recv or send for the channel type t (typedesc.go).
func chanDir(t *types.Chan) cppir.Expr {
	switch t.Dir() {
	case types.RecvOnly:
		return cxxName("gp::chan_dir::recv")
	case types.SendOnly:
		return cxxName("gp::chan_dir::send")
	}
	return cxxName("gp::chan_dir::both")
}

// sameChanUnder reports whether a and b are channel types that differ only in their direction
// (or not at all), the assignability of a bidirectional channel to a directional one.
func sameChanUnder(a, b types.Type) bool {
	ca, ok1 := under(a).(*types.Chan)
	cb, ok2 := under(b).(*types.Chan)
	return ok1 && ok2 && types.Identical(ca.Elem(), cb.Elem())
}

// chanRecv returns the C++ of the receive <-ch: gp::recv(ch), or gp::recv2(ch) where it gives two
// values (v, ok := <-ch: go/types records a tuple type for it; a channel of C/C++ values has none:
// commaRecvs).
func (fg *funcGen) chanRecv(e *ast.UnaryExpr) cppir.Expr {
	fun := "gp::recv"
	if isTuple(fg.g.info.TypeOf(e)) || fg.commaRecvs[e] {
		fun = "gp::recv2"
	}
	fg.g.noteChan(e.Pos(), fun) // why.go
	return call(fun, fg.expr(e.X))
}

// isRecv returns e as the receive <-ch if it is one, or nil.
func isRecv(e ast.Expr) *ast.UnaryExpr {
	u, ok := ast.Unparen(e).(*ast.UnaryExpr)
	if !ok || u.Op != token.ARROW {
		return nil
	}
	return u
}

// sendStmt lowers ch <- v: gp::send(ch, v);. The channel and the value are evaluated first, in
// the order of a call's arguments, and then the goroutine communicates.
func (fg *funcGen) sendStmt(s *ast.SendStmt) {
	fg.g.noteChan(s.Pos(), "gp::send") // why.go
	p := fg.g.pos(s.Pos())
	pre := fg.order(false, s.Chan, s.Value)
	var v cppir.Expr
	if fg.g.pkg.Opaque.IsOpaque(s.Value) && fg.constValue(s.Value) == nil {
		v = fg.deferArg(s.Value) // a C/C++ value: copied if it is a variable, moved if it is a temporary
	} else {
		v = fg.expr(s.Value)
	}
	fg.add(withPre(p, pre, &cppir.ExprStmt{Pos: p, X: call("gp::send", fg.expr(s.Chan), v)}))
}

// chanBuiltin returns the C++ of a call of make or close on channels. make(chan T, n) takes the
// size as an int: a negative or too big size panics ("makechan: size out of range").
func (fg *funcGen) chanBuiltin(c *ast.CallExpr, name string) cppir.Expr {
	arg := c.Args[0]
	switch name {
	case "make":
		ct := under(fg.g.info.TypeOf(arg)).(*types.Chan)
		var elem cppir.Node
		if at, ok := ast.Unparen(arg).(*ast.ChanType); ok && !complete(ct) {
			elem = fg.g.typeExpr(at.Value) // make(chan C.int): the C/C++ part from the type expression (ctypes.go)
		} else {
			elem = fg.g.cxxType(ct.Elem(), arg.Pos())
		}
		fg.g.noteChan(c.Pos(), "gp::make_chan") // why.go
		if len(c.Args) == 1 {
			return templateCall("gp::make_chan", []cppir.Node{elem})
		}
		return templateCall("gp::make_chan", []cppir.Node{elem}, fg.intExpr(c.Args[1]))
	case "close":
		fg.g.noteChan(c.Pos(), "gp::close") // why.go
		return call("gp::close", fg.expr(arg))
	}
	fg.g.errorf(c.Pos(), "internal error: %s of a channel (a construct the checker should have rejected)", name)
	return placeholder
}

// chanConversion returns the C++ of a conversion to a channel type: chan T(nil) is gp::chan<T>(),
// and a channel of the same C++ type (every direction of a chan T) is itself.
func (fg *funcGen) chanConversion(c *ast.CallExpr, to, from types.Type, x cppir.Expr) cppir.Expr {
	arg := c.Args[0]
	typ := fg.g.cxxType(to, c.Pos())
	switch {
	case fg.g.info.Types[arg].IsNil():
		return construct(typ)
	case isChan(from) && cppir.TypeString(typ) == cppir.TypeString(fg.g.cxxType(from, arg.Pos())):
		return x
	}
	qual := types.RelativeTo(fg.g.pkg.Types)
	fg.g.unsupported(c.Pos(), "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual))
	return placeholder
}

// selectStmt lowers a select statement (Phase 08 step 8.5; the runtime is gocpp/select.hpp):
//
//	select {                              {
//	case v := <-a:                            gp::sel_recv gp_s0{a};
//	    use(v)                                gp::sel_send gp_s1{b, f()};
//	case b <- f():                            gp::sel_recv gp_s2{c, gp::sel_drop};
//	case <-c:                                 switch (gp::select(gp_s0, gp_s1, gp_s2, gp::sel_default)) {
//	default:                                  case 0: { int64_t v = gp_s0.gp_take(); use(v); } break;
//	}                                         case 1: ... break;
//	                                          case 2: ... break;
//	                                          default: ... }
//	                                      }
//	select {}                             gp::select();
//
// Go evaluates the channel expressions and the values to send once, in source order, when the
// statement starts, whether or not a case is chosen (the objects are declared in that order, each
// after the temporaries of its own calls, order.go, and before the calls of the next case, as gc
// copies each channel and value into a temporary in source order); the left-hand sides of a receive case are
// evaluated only if it is chosen (the assignment is the first statement of the case: bindVars, as
// for a range variable). A channel that is a local variable is held by reference (nothing can
// change it during the statement); any other channel expression is copied (a handle), so that a
// later case's call can't change or free it. break leaves the C++ switch, like Go's select;
// continue and the labelled jumps are the loop's (flow.go).
func (fg *funcGen) selectStmt(s *ast.SelectStmt) {
	fg.g.noteSelect(s) // why.go
	if len(s.Body.List) == 0 {
		p := fg.g.pos(s.Select)
		fg.add(&cppir.ExprStmt{Pos: p, X: call("gp::select")})
		return
	}
	clauses := make([]*ast.CommClause, len(s.Body.List))
	for i, c := range s.Body.List {
		clauses[i] = c.(*ast.CommClause)
	}
	end := s.Body.Rbrace
	p := fg.g.pos(s.Select)
	fg.open(s.Select)
	// The cases in source order, each declared (its channel read, its value copied) before the next
	// case's calls run, as gc copies each channel and value into a temporary in source order
	// (walk/order.go, OSELECT): a call in a later case that changes a variable does not change the
	// channel or the value of an earlier one.
	names := make([]string, len(clauses)) // the object of each case ("" for default)
	args := make([]cppir.Expr, len(clauses))
	for i, c := range clauses {
		if c.Comm == nil {
			args[i] = cxxName("gp::sel_default")
			continue
		}
		names[i] = fg.names.temp("s")
		args[i] = cxxName(names[i])
		cp := fg.g.pos(c.Case)
		decl := func(typ string, elems ...cppir.Expr) {
			fg.add(&cppir.VarDecl{Pos: cp, Type: typeName(typ), Vars: []cppir.Declarator{{Name: names[i], Init: &cppir.InitList{Elems: elems}}}})
		}
		pre := func(es ...ast.Expr) {
			if pre := fg.order(false, es...); len(pre) > 0 {
				fg.add(&cppir.Line{Pos: cp, Stmts: pre})
			}
		}
		switch comm := c.Comm.(type) {
		case *ast.SendStmt:
			pre(comm.Chan)
			ch := fg.selectChan(comm.Chan)
			if !fg.isLocal(comm.Chan) && fg.needsPre(false, comm.Value) {
				// The value's calls run after the channel is read (a copy of the handle).
				t := fg.names.temp("c")
				fg.add(varDecl(cp, typeName("auto"), t, ch))
				ch = call("std::move", cxxName(t))
			}
			pre(comm.Value)
			var v cppir.Expr
			if fg.g.pkg.Opaque.IsOpaque(comm.Value) && fg.constValue(comm.Value) == nil {
				v = fg.deferArg(comm.Value) // a C/C++ value: copied if it is a variable, moved if it is a temporary
			} else {
				v = fg.expr(comm.Value)
			}
			decl("gp::sel_send", ch, v)
		case *ast.ExprStmt:
			x := selectRecv(comm).X
			pre(x)
			decl("gp::sel_recv", fg.selectChan(x), cxxName("gp::sel_drop"))
		case *ast.AssignStmt:
			x := selectRecv(comm).X
			pre(x)
			elems := []cppir.Expr{fg.selectChan(x)}
			if len(comm.Lhs) == 0 || isBlank(comm.Lhs[0]) {
				elems = append(elems, cxxName("gp::sel_drop"))
			}
			decl("gp::sel_recv", elems...)
		}
	}
	sw := &cppir.Switch{Pos: p, Tag: call("gp::select", args...)}
	fg.add(sw)
	hasDefault, allJump := false, true
	for i, c := range clauses {
		hasDefault = hasDefault || c.Comm == nil
		allJump = allJump && fg.endsInJump(c.Body)
		var values []cppir.Expr // nil: default
		if c.Comm != nil {
			values = []cppir.Expr{lit(strconv.Itoa(i))}
		}
		cs := &cppir.Case{Pos: fg.g.pos(c.Case), Values: values, Break: !endsWithJumpList(c.Body)}
		sw.Cases = append(sw.Cases, cs)
		fg.fresh = false
		fg.inner(&cs.Body, &cs.End, commEnd(c), func() {
			if a, ok := c.Comm.(*ast.AssignStmt); ok {
				fg.selectBind(a, names[i])
			}
			fg.stmts(c.Body, commEnd(c))
		})
	}
	sw.End = fg.g.pos(end)
	if !hasDefault && !fg.flow.breakTo[s] && !fg.flow.breakLabel[s] && allJump {
		// A select statement without default whose cases all end in a jump is a terminating
		// statement in Go: a function can end with it. C++ does not know that the switch has no
		// other value (-Wreturn-type).
		fg.add(&cppir.ExprStmt{Pos: fg.g.pos(end), X: call("gp::select_unreachable")})
	}
	fg.close(end)
	fg.breakLabel(s, end)
}

// selectRecv returns the receive <-ch of a receive case: `<-ch`, `v := <-ch`, `v, ok = <-ch`.
func selectRecv(comm ast.Stmt) *ast.UnaryExpr {
	switch c := comm.(type) {
	case *ast.ExprStmt:
		return isRecv(c.X)
	case *ast.AssignStmt:
		return isRecv(c.Rhs[0])
	}
	return nil
}

// selectChan returns the C++ of the channel of a case: a local variable as it is (the case holds it by
// reference), a call's result as it is (the case takes it), any other expression as a copy of the
// handle (gp::chan<T>(x)).
func (fg *funcGen) selectChan(e ast.Expr) cppir.Expr {
	x := fg.expr(e)
	if fg.isLocal(e) {
		return x
	}
	if c, ok := ast.Unparen(e).(*ast.CallExpr); ok && !fg.g.info.Types[c.Fun].IsType() {
		return x
	}
	return construct(fg.g.cxxType(fg.g.info.TypeOf(e), e.Pos()), x)
}

// selectBind adds the line that gives the variables of the receive case a (v := <-c, v, ok = <-c)
// their values, from the case's object name, at the start of the case.
func (fg *funcGen) selectBind(a *ast.AssignStmt, name string) {
	ct, ok := under(fg.g.info.TypeOf(selectRecv(a).X)).(*types.Chan)
	if !ok {
		return
	}
	var vars []rangeVar
	for i, e := range a.Lhs {
		if isBlank(e) {
			continue
		}
		v := rangeVar{e: e, value: &cppir.Call{Fun: &cppir.Member{X: cxxName(name), Name: "gp_take"}}, from: ct.Elem()}
		if i == 1 {
			v.value = &cppir.Call{Fun: &cppir.Member{X: cxxName(name), Name: "gp_ok"}}
			v.from = types.Typ[types.Bool]
		}
		if a.Tok == token.DEFINE {
			v.obj, _ = fg.g.info.Defs[e.(*ast.Ident)].(*types.Var)
			if v.obj == nil {
				fg.g.errorf(e.Pos(), "internal error: a variable of a receive case without an object")
				continue
			}
			v.typ = fg.g.varType(v.obj, e.Pos())
		}
		vars = append(vars, v)
	}
	if len(vars) > 0 {
		fg.bindVars(a.Tok, vars, false, nil)
	}
}

// commEnd returns the place of the end of a select case: its last statement, or the colon.
func commEnd(c *ast.CommClause) token.Pos {
	if len(c.Body) == 0 {
		return c.Colon
	}
	return c.Body[len(c.Body)-1].End()
}

// endsWithJumpList is endsWithJump for a list of statements.
func endsWithJumpList(list []ast.Stmt) bool {
	switch lastStmt(list).(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	}
	return false
}

// endsInJump reports whether control cannot fall off the end of the statement list: its last
// statement is terminating (the spec's "Terminating statements": return, goto, a call of panic, a
// block, an if with else, a for without condition and without break, a switch or select with
// every clause terminating and no break, a labeled statement), or a continue, which also leaves
// the select's switch for good. A select without default whose cases all end like this is a
// terminating statement in Go, so a function can end with it, and C++ needs to be told that the
// switch has no other value.
func (fg *funcGen) endsInJump(list []ast.Stmt) bool {
	return fg.noFallOff(lastStmt(list))
}

func (fg *funcGen) noFallOff(st ast.Stmt) bool {
	switch x := st.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return x.Tok == token.GOTO || x.Tok == token.CONTINUE
	case *ast.ExprStmt:
		c, ok := ast.Unparen(x.X).(*ast.CallExpr)
		return ok && fg.isBuiltin(c, "panic")
	case *ast.BlockStmt:
		return fg.endsInJump(x.List)
	case *ast.LabeledStmt:
		return fg.noFallOff(x.Stmt)
	case *ast.IfStmt:
		if x.Else == nil || !fg.endsInJump(x.Body.List) {
			return false
		}
		return fg.noFallOff(x.Else)
	case *ast.ForStmt:
		return x.Cond == nil && !fg.breaksOut(x, x.Body)
	case *ast.SwitchStmt:
		return fg.clausesNoFallOff(x, x.Body)
	case *ast.TypeSwitchStmt:
		return fg.clausesNoFallOff(x, x.Body)
	case *ast.SelectStmt:
		if fg.breaksOut(x, x.Body) {
			return false
		}
		for _, c := range x.Body.List {
			if !fg.endsInJump(c.(*ast.CommClause).Body) {
				return false
			}
		}
		return true
	}
	return false
}

// clausesNoFallOff reports whether the switch has a default, no break to it, and every clause ends
// in a terminating statement or fallthrough.
func (fg *funcGen) clausesNoFallOff(sw ast.Stmt, body *ast.BlockStmt) bool {
	if fg.breaksOut(sw, body) {
		return false
	}
	hasDefault := false
	for _, c := range body.List {
		cc := c.(*ast.CaseClause)
		hasDefault = hasDefault || cc.List == nil
		if b, ok := lastStmt(cc.Body).(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
			continue
		}
		if !fg.endsInJump(cc.Body) {
			return false
		}
	}
	return hasDefault
}

// breaksOut reports whether a break statement in body goes to the statement st (flow.go).
func (fg *funcGen) breaksOut(st ast.Stmt, body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch b := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if b.Tok == token.BREAK && fg.flow.target[b] == st {
				found = true
			}
		}
		return !found
	})
	return found
}
