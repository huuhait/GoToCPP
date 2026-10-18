package lower

import (
	"go/ast"
	"go/token"
	"go/types"

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
		if len(c.Args) == 1 {
			return templateCall("gp::make_chan", []cppir.Node{elem})
		}
		return templateCall("gp::make_chan", []cppir.Node{elem}, fg.intExpr(c.Args[1]))
	case "close":
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
