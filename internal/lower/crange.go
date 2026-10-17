package lower

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// range over a C++ container or a C array (Phase 04 README §2.5, step 4.7): a C++ range-based
// for over gp::range, with only the requested bindings copied (gocpp/interop.hpp):
//
//	for i, v := range c         for (auto&& [i, v] : gp::range(c)) {
//	for _, v := range c         for (auto&& v : gp::range_values(c)) {
//	for k := range c            for (auto&& k : gp::range_keys(c)) {
//	for range c                 for ([[maybe_unused]] auto&& gp_k0 : gp::range_skip(c)) {
//	for i, v = range c          for (auto&& [gp_k0, gp_v0] : gp::range(c)) {
//	                                i = static_cast<decltype(gp_k0)&&>(gp_k0); v = ...;
//
// Each step of gp::range gives a new pair: the index (the key for a map, as in Go) and a copy of
// the element (of the mapped value), so the loop variables are copies, and each iteration has its
// own (Go 1.22). A random-access container is walked by index, with its size read once, as Go
// reads a slice's length once: a push_back in the body is not visited, and an element that the
// body removed panics with Go's index error when the walk reaches it. Lists, sets and maps are
// walked with their iterators, with C++'s rules: a change in the body that invalidates the
// current iterator is undefined behavior, as in C++. The index and the key are C/C++ values
// (int64_t for an index, Go's int): gocpp doesn't know if c is a map.
//
// The range expression is evaluated once. A temporary container (range f()) is held inside
// gp::range by value; a C++ range-for keeps only that alive, so the temporaries that the range
// expression uses otherwise go into named temporaries before the loop (rangeOperand): range
// f().items keeps the object of f() alive, which can be the only owner of items (a gocpp
// pointer, gp::deref(f()).items).
func (fg *funcGen) rangeC(s *ast.RangeStmt) {
	fg.rangeLifetimes = true
	pre := fg.order(false, s.X)
	fg.rangeLifetimes = false
	fg.rangeOperand(s.X, &pre)
	if len(pre) > 0 {
		fg.open(s.For)
		fg.add(&cppir.Line{Pos: fg.g.pos(s.X.Pos()), Stmts: pre})
	}
	fg.names.openScope() // the range variables and the body
	var key, value ast.Expr
	if s.Key != nil && !isBlank(s.Key) {
		key = s.Key
	}
	if s.Value != nil && !isBlank(s.Value) {
		value = s.Value
	}
	fn := "gp::range"
	switch {
	case key == nil && value == nil:
		fn = "gp::range_skip"
	case value == nil:
		fn = "gp::range_keys"
	case key == nil:
		fn = "gp::range_values"
	}
	// The C++ names of the loop's variables: the Go names for :=, hidden ones for =, which the
	// body's first line assigns to the targets.
	name := func(e ast.Expr, hidden string) string {
		if e == nil {
			return fg.names.temp(hidden)
		}
		if s.Tok != token.DEFINE {
			return fg.names.temp(hidden)
		}
		obj, _ := fg.g.info.Defs[e.(*ast.Ident)].(*types.Var)
		if obj == nil {
			fg.g.errorf(e.Pos(), "internal error: a range variable without an object")
			return "gp_unsupported"
		}
		if fg.isBoxed(obj) {
			// The guard reports what would box it (a closure, &v) until step 4.8.
			fg.g.errorf(e.Pos(), "internal error: a boxed C/C++ range variable")
		}
		return fg.names.declare(obj)
	}
	k := name(key, "k")
	decl := "auto&& " + k
	var v string
	if value != nil {
		v = name(value, "v")
		decl = "auto&& " + v
		if key != nil {
			decl = "auto&& [" + k + ", " + v + "]"
		}
	}
	if key == nil && value == nil {
		decl = "[[maybe_unused]] " + decl
	}
	head := &cppir.For{Pos: fg.g.pos(s.For), Decl: decl, Range: call(fn, fg.expr(s.X))}
	l := loop{stmt: s, head: head, body: s.Body}
	if s.Tok != token.DEFINE && (key != nil || value != nil) {
		// An assignment of the step's values to the targets, at the start of each iteration,
		// as by key, value = gp_k0, gp_v0: the targets' operands first (with their own order).
		var lhs []ast.Expr
		var from []string
		if key != nil {
			lhs, from = append(lhs, key), append(from, k)
		}
		if value != nil {
			lhs, from = append(lhs, value), append(from, v)
		}
		l.vars = func() {
			line := &cppir.Line{Pos: fg.g.pos(lhs[0].Pos())}
			line.Stmts = append(line.Stmts, fg.order(false, fg.targetOperands(lhs)...)...)
			targets := make([]target, len(lhs))
			for i, e := range lhs {
				targets[i] = fg.lhs(e)
			}
			for i := range targets {
				line.Stmts = append(line.Stmts, fg.saveOperands(cppir.Pos{}, targets, i)...)
			}
			for i, t := range targets {
				// The step's value is the loop's own copy: moved into the target.
				line.Stmts = append(line.Stmts, &cppir.ExprStmt{X: t.assignExpr(movedName(from[i]))})
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

// rangeOperand names the temporaries of the range expression x of a range over a C/C++ value
// that the loop must keep alive (rangeC): the temporary that a place is in (f().items, f()[0]:
// namedOrPlace), and the callee and temporary arguments, including constructor backing
// arguments (nameOperands), which the call's result can refer to (v.sorted() of a temporary v, std.max(a(), b())). The call itself
// stays in gp::range, which holds its result by value if it is a temporary. A name, a place of
// a variable, and any other expression stay as they are.
func (fg *funcGen) rangeOperand(x ast.Expr, pre *[]cppir.Stmt) {
	if fg.namedOrPlace(x, pre) {
		return
	}
	fg.nameOperands(x, pre)
}

// cRange reports whether s is a range over a C/C++ value that is not a gocpp container: a C++
// container, a view or a C array (rangeC).
func (g *gen) cRange(s *ast.RangeStmt) bool {
	return g.pkg.Opaque != nil && g.pkg.Opaque.IsOpaque(s.X) && !gocppContainer(g.info.TypeOf(s.X))
}

// ownedValue owns reference members of an opaque call's pair/tuple result before the call's
// argument temporaries die. The call itself must stay in the selected branch: a value-taking
// wrapper would lose guaranteed prvalue elision (std::mutex, deleted move/copy constructors),
// and a callback returning a reference pair would destroy its backing arguments too early.
// The generic lambda makes the values<R> branch dependent, so it is not instantiated for an
// ordinary result. R is the type of the call, given by the lambda's argument
// std::type_identity<decltype(call)>: decltype never evaluates the call, and it stays outside
// the lambda, because a func literal that captures a local, in a lambda's default template
// argument, is not capturable for clang. Exactly one branch evaluates the call, once.
func (fg *funcGen) ownedValue(e ast.Expr, x cppir.Expr) cppir.Expr {
	if _, ok := fg.temps[ast.Unparen(e)]; ok || !fg.g.ownedCall(e) {
		return x // a temporary of the order is owned already (emitItem)
	}
	r := fg.names.temp("R")
	cond := &cppir.Name{Name: "gp::detail::is_values", Args: []cppir.Node{typeName(r)}}
	owned := &cppir.Call{Fun: &cppir.Name{Name: "gp::values", Args: []cppir.Node{typeName(r)}}, Args: []cppir.Expr{x}}
	tag := &cppir.TypeName{Name: "std::type_identity", Args: []cppir.Node{call("decltype", x)}}
	return &cppir.Call{
		Fun: &cppir.Lambda{
			Default:  "&",
			Template: []cppir.Param{{Type: typeName("typename"), Name: r}},
			Params:   []cppir.Param{{Type: &cppir.TypeName{Name: "std::type_identity", Args: []cppir.Node{typeName(r)}}}},
			Body: &cppir.Block{Inline: true, Stmts: []cppir.Stmt{
				&cppir.If{Constexpr: true, Cond: cond,
					Then: &cppir.Block{Inline: true, Stmts: []cppir.Stmt{&cppir.Return{X: owned}}},
					Else: &cppir.Block{Inline: true, Stmts: []cppir.Stmt{&cppir.Return{X: &cppir.Paren{X: x}}}},
				},
			}},
		},
		Args: []cppir.Expr{construct(tag)},
	}
}

// ownedCall reports whether e is a call whose result gocpp doesn't know a type for and that is
// not a new object: a call of a C/C++ function (also a function-like macro or a static member),
// of a member of a C/C++ value or of a C++ callable. A constructor, a conversion (C.int(x)) and a
// gocpp function with C/C++ results give a value of their own type.
func (g *gen) ownedCall(e ast.Expr) bool {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || g.pkg.Opaque == nil || !g.pkg.Opaque.IsOpaque(c) || g.info.Types[c.Fun].IsType() ||
		g.pkg.Conversion(c) != nil || g.calledSig(c) != nil {
		return false
	}
	if r, ok := g.cRefOf(c.Fun); ok {
		return r.kind != interop.Type && r.kind != interop.Namespace
	}
	return g.opaqueCallee(c)
}
