package lower

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// The evaluation order (Phase 02 README §2.5).
//
// Go evaluates the calls of an expression from left to right; C++ leaves the arguments of a
// call and the operands of most operators unordered, and g++ and clang++ really pick different
// orders: fmt.Println(next(), next()) prints "2 1" with g++. So the calls that need it are
// moved into gp_ temporaries, in Go's order, before the C++ expression:
//
//	fmt.Println(next(), next())   { auto gp_t0 = next(); gp::fmtshim::Println(gp_t0, next()); }
//
// What the C++ must do is what Go's compiler (gc) does, because that is what the tests
// compare with, also where the spec leaves the order open (g(x, f()) with an f that changes
// the package variable x prints the new x with gc). gc's order pass (cmd/compile's
// walk/order.go) moves every call of a statement into a temporary, in order, and leaves the
// rest of the expression in place, evaluated after all of them: in g(x, f()), f runs first,
// then x is read and g is called. The units that it moves are:
//   - calls, with their own arguments evaluated just before them (a call is a scope of its
//     own: its arguments are ordered the same way, recursively);
//   - len(x), when x could change;
//   - x && y and x || y, as a whole: y's temporaries must not run unless x says so, so gc
//     evaluates them into a temporary of their own (r := x; if r { r = y });
//   - an argument of fmt.Print, Println or Printf (a ...any parameter) of a one-byte type
//     (bool, int8, uint8) that is not a variable: so fmt.Println(s[i], grow()) panics before
//     grow runs, and fmt.Println(x == 1, f()) compares the old x, while fmt.Println(x, f())
//     prints the new x. gc's rule (walk/order.go, OCONVIFACE and addrTemp): a conversion of
//     a value to an interface copies the value into a temporary in its place when the value
//     is not addressable (not a variable; a local variable is copied too, which can't be
//     seen) and its type is not one that the runtime converts by value: a 2-, 4- or 8-byte
//     scalar, a string or a slice (dataWordFuncName). In Phase 02 only fmt's arguments are
//     converted to interfaces, and only bool, int8 and uint8 are copied. Phase 03 must
//     extend this to every conversion to an interface (any parameters, assignments to
//     interface variables, panic) and to the other types gc copies (complex numbers, arrays,
//     structs).
//
// The rest stays where it is, and its order matters only where it can be seen:
//   - a read of a package variable, which any call can change (Phase 03: collect sees only
//     the package's own variables, an identifier; a variable of another package, pkg.V, is a
//     selector, which must become a read too);
//   - an operation that can panic: an integer division or remainder by a non-constant
//     (gp::div), a shift by a non-constant signed count (gp::shl), and s[i] (out of range). gc
//     evaluates them after the calls of their scope: a/b + f() calls f first, then panics.
//
// A divergence (the manager's decision, 2.5 review): gc also evaluates two such operations
// from left to right (with both out of range, s[i] == s[j] reports i), but gocpp doesn't keep
// that order: it would need temporaries in common code (fmt.Println(s[i], s[j])), and the
// only difference is which of the two panic messages is printed. Calls, output and the values
// that are read keep Go's order.
//
// Reads of the function's own variables are not effects: in Phase 02 a local variable can
// change only by an assignment, and an expression has none (no closures, no pointers, no
// methods with pointer receivers yet: Phase 03's lowering pass decides then). A statement
// that assigns several variables (a, b = b, a) keeps its own rule (bind, stmt.go). Constants
// are never effects.
//
// A unit goes into a temporary only where C++ could get the order wrong: where it has a
// neighbour, an operand or an argument next to it, that is not a constant or a local variable,
// and that C++ doesn't evaluate after it. Precisely, for each scope (a statement's
// expressions, or a call's arguments), the units are listed in Go's order; the shortest
// leading part of that list goes into temporaries such that, in the C++ of the rest, every
// two things whose order can be seen (a call and anything else that is an effect: another
// unit, a read of a package variable, a panic) are in Go's order: one is an operand of the
// other, or both are in a list that C++ evaluates in order (the values of return {a, b},
// the declarations of x, y := a, b). A unit that stays in the expression but needs
// temporaries of its own (g(x, f()) in h(k(), g(x, f()))) writes them in front of the
// statement, so the units before it go into temporaries first.
//
// Where the temporaries go:
//   - a simple statement or a return: in front of it, in a block, { auto gp_t0 = f(); ... };
//     a declaration without the block (its names must stay in scope);
//   - the condition of a loop: at the top of the body, followed by if (!cond) break; (the
//     condition is evaluated before every iteration, and continue goes there too);
//   - an else if condition, and a case expression: in a block of the else branch, as for an
//     else if with an init statement (they run only if the conditions before them failed);
//     in a case list, an expression after the first one sets a bool only if the ones before
//     didn't match (caseCond);
//   - the right side of && or ||: inside an if on the left side's value.
//
// Each funcGen.order call orders one scope: it returns the statements of the temporaries,
// in order, and from then on fg.expr writes a temporary's name for its expression. Every
// call that fg.call translates must have been ordered (fg.ordered); a context that forgets
// it is an internal error rather than silently wrong order.

// An effect is what evaluating an expression can do that the rest of its statement can see,
// or what it can see of it.
type effect struct {
	call   bool // it calls a function, which can do anything (print, panic, change variables)
	read   bool // it reads a package variable
	panics bool // one of its operations can panic at run time
}

func (e effect) any() bool { return e.call || e.read || e.panics }

func (e effect) or(f effect) effect {
	return effect{e.call || f.call, e.read || f.read, e.panics || f.panics}
}

// conflict reports whether the order in which a and b are evaluated can be seen, where both
// have an effect: one of them is a call. The order of two panicking operations is not kept
// (see the top of the file).
func conflict(a, b effect) bool {
	return a.call || b.call
}

// An orderItem is a part of a scope whose place in the order matters: a unit (a call, len,
// && / || or a copied fmt argument), a read of a package variable, or an operation that can
// panic.
type orderItem struct {
	e     ast.Expr        // without parentheses
	unit  bool            // evaluated as a whole, in its place among the calls
	eff   effect          // of the whole item (for a unit, with its own scopes)
	root  int             // the index of its expression in the scope's list
	up    *orderStep      // the operators above it, innermost first
	inner []*orderScope   // a unit's own scopes: a call's arguments; x and y of x && y
	block bool            // x && y or x || y whose y needs temporaries: written as an if
	bin   *ast.BinaryExpr // the expression of && and ||
}

// An orderStep is one operator above an item: a linked list up to the top of its expression.
type orderStep struct {
	node ast.Expr
	up   *orderStep
}

// An orderScope is one scope of the order: a list of expressions that C++ evaluates in
// order (seq) or in any order, with the items found in them, in post order.
type orderScope struct {
	seq   bool
	items []*orderItem
	hoist []*orderItem // the units that go into temporaries, in Go's order
	pre   bool         // writing the scope needs statements before it
	calls []ast.Expr   // the calls in the scope that are not units (len of a local): ordered too
}

// effect returns the effect of the whole scope.
func (sc *orderScope) effect() effect {
	var e effect
	for _, it := range sc.items {
		e = e.or(it.eff)
	}
	return e
}

// order orders the scope of the expressions list (seq: C++ evaluates them in order) and
// returns the statements that must run before them, in order. Afterwards fg.expr writes the
// temporaries' names for the expressions that went into them.
func (fg *funcGen) order(seq bool, list ...ast.Expr) []cppir.Stmt {
	var pre []cppir.Stmt
	fg.emitScope(fg.analyze(seq, list), &pre)
	return pre
}

// needsPre reports whether order(seq, list...) would return statements, without writing
// anything.
func (fg *funcGen) needsPre(seq bool, list ...ast.Expr) bool {
	return fg.analyze(seq, list).pre
}

// inert reports whether evaluating e, after its scope was ordered, has no effect and can't
// see one: then it can be evaluated anywhere in its statement.
func (fg *funcGen) inert(e ast.Expr) bool {
	return len(fg.analyze(false, []ast.Expr{e}).items) == 0
}

// analyze finds the items of a scope and chooses its temporaries.
func (fg *funcGen) analyze(seq bool, list []ast.Expr) *orderScope {
	return fg.analyzeArgs(seq, list, len(list))
}

// analyzeArgs is analyze, where the expressions from the index iface on are arguments of
// fmt's ...any (see the top of the file).
func (fg *funcGen) analyzeArgs(seq bool, list []ast.Expr, iface int) *orderScope {
	sc := &orderScope{seq: seq}
	for i, e := range list {
		if i >= iface && fg.copied(e) {
			in := fg.analyze(false, []ast.Expr{e})
			if eff := in.effect(); eff.any() {
				sc.items = append(sc.items, &orderItem{e: ast.Unparen(e), unit: true, eff: eff, root: i, inner: []*orderScope{in}})
			} else {
				sc.calls = append(sc.calls, in.calls...)
			}
			continue
		}
		fg.collect(sc, e, i, nil)
	}
	sc.plan()
	return sc
}

// copied reports whether gc copies the argument e of fmt's ...any into a temporary in its
// place among the calls: a value of a one-byte type that is not a variable (and not a unit
// already).
func (fg *funcGen) copied(e ast.Expr) bool {
	e = ast.Unparen(e)
	if _, ok := fg.temps[e]; ok || fg.constValue(e) != nil {
		return false
	}
	switch x := e.(type) {
	case *ast.Ident:
		return false
	case *ast.CallExpr:
		if !fg.g.info.Types[x.Fun].IsType() {
			return false // a call
		}
	case *ast.BinaryExpr:
		if x.Op == token.LAND || x.Op == token.LOR {
			return false
		}
	}
	b := basicOf(fg.g.info.TypeOf(e))
	if b == nil {
		return false
	}
	switch b.Kind() {
	case types.Bool, types.UntypedBool, types.Int8, types.Uint8:
		return true
	}
	return false
}

// collect adds the items of e, the expression root of the scope's list or an operand below
// the operators up, to the scope, in post order (the operands of an operator before the
// operator).
func (fg *funcGen) collect(sc *orderScope, e ast.Expr, root int, up *orderStep) {
	e = ast.Unparen(e)
	if _, ok := fg.temps[e]; ok || fg.constValue(e) != nil {
		return
	}
	add := func(it *orderItem) {
		it.e, it.root, it.up = e, root, up
		sc.items = append(sc.items, it)
	}
	below := &orderStep{e, up} // for the operands of e
	switch x := e.(type) {
	case *ast.Ident:
		if v, ok := fg.g.info.Uses[x].(*types.Var); ok && isPkgVar(v) {
			add(&orderItem{eff: effect{read: true}})
		}
	case *ast.CallExpr:
		if fg.g.info.Types[x.Fun].IsType() {
			for _, a := range x.Args {
				fg.collect(sc, a, root, up) // a conversion: its operand
			}
			return
		}
		iface := len(x.Args)
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			if fn, ok := fg.g.info.Uses[sel.Sel].(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" {
				iface = 0
				if fn.Name() == "Printf" {
					iface = 1 // the format is a string
				}
			}
		}
		args := fg.analyzeArgs(false, x.Args, iface)
		it := &orderItem{unit: true, inner: []*orderScope{args}, eff: effect{call: true}}
		if fg.isBuiltin(x, "len") {
			// len of a string has no effect of its own, but gc evaluates it in its place
			// among the calls.
			if it.eff = args.effect(); !it.eff.any() {
				sc.calls = append(append(sc.calls, x), args.calls...)
				return
			}
		}
		add(it)
	case *ast.BinaryExpr:
		if x.Op == token.LAND || x.Op == token.LOR {
			l, r := fg.analyze(false, []ast.Expr{x.X}), fg.analyze(false, []ast.Expr{x.Y})
			if eff := l.effect().or(r.effect()); eff.any() {
				add(&orderItem{unit: true, eff: eff, inner: []*orderScope{l, r}, block: r.pre, bin: x})
			} else {
				sc.calls = append(append(sc.calls, l.calls...), r.calls...)
			}
			return
		}
		fg.collect(sc, x.X, root, below)
		fg.collect(sc, x.Y, root, below)
		if fg.binaryPanics(x) {
			add(&orderItem{eff: effect{panics: true}})
		}
	case *ast.UnaryExpr:
		fg.collect(sc, x.X, root, up)
	case *ast.IndexExpr:
		fg.collect(sc, x.X, root, below)
		fg.collect(sc, x.Index, root, below)
		add(&orderItem{eff: effect{panics: true}})
	}
}

// isPkgVar reports whether v is a package variable, which any call can change.
func isPkgVar(v *types.Var) bool {
	return v.Pkg() != nil && v.Parent() == v.Pkg().Scope()
}

// isBuiltin reports whether c calls the builtin name.
func (fg *funcGen) isBuiltin(c *ast.CallExpr, name string) bool {
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := fg.g.info.Uses[id].(*types.Builtin)
	return ok && b.Name() == name
}

// binaryPanics reports whether the operator of e can panic at run time (ops.go): an integer
// division or remainder by a divisor that is not a constant, and a shift by a count that is
// not a constant and has a signed type (a negative count panics).
func (fg *funcGen) binaryPanics(e *ast.BinaryExpr) bool {
	if fg.constValue(e.Y) != nil {
		return false
	}
	switch e.Op {
	case token.QUO, token.REM:
		b := basicOf(fg.g.info.TypeOf(e))
		return b != nil && b.Info()&types.IsInteger != 0
	case token.SHL, token.SHR:
		// An untyped count is a uint (typeOf).
		b := basicOf(fg.g.info.TypeOf(e.Y))
		return b != nil && b.Info()&(types.IsUnsigned|types.IsUntyped) == 0
	}
	return false
}

// plan chooses the temporaries of the scope (see the top of the file): the shortest leading
// part of its units, in Go's order, such that the C++ of the rest evaluates every remaining
// unit before everything that Go evaluates after it and whose order with it can be seen.
// Only units go into temporaries: the other items are evaluated after all units, in Go as
// in the C++ of the rest.
//
// The rest is right exactly when each unit in it is right on its own (C++ evaluates it before
// the later units and the other items that it conflicts with), so the shortest part ends
// after the last unit that isn't. That makes plan O(n^2) in the items of the scope.
func (sc *orderScope) plan() {
	var units, others []*orderItem
	for _, it := range sc.items {
		if it.unit {
			units = append(units, it)
		} else {
			others = append(others, it)
		}
	}
	// An && or || whose right side needs temporaries is an if: it can't stay inside the
	// C++ expression.
	n := 0
	for i, u := range units {
		if u.block {
			n = i + 1
		}
	}
	for i := len(units) - 1; i >= n; i-- {
		if !sc.first(units[i], units[i+1:], others) {
			n = i + 1
			break
		}
	}
	// A unit that stays in the expression writes its own temporaries in front of the
	// statement: the units before it must go first, into temporaries.
	for i := len(units) - 1; i >= n; i-- {
		if units[i].inner[0].pre {
			n = i
			break
		}
	}
	sc.hoist = units[:n]
	sc.pre = n > 0
	for _, u := range units[n:] {
		sc.pre = sc.pre || u.inner[0].pre
	}
}

// first reports whether C++ evaluates the unit u before the later units and the other items,
// wherever Go's order can be seen.
func (sc *orderScope) first(u *orderItem, later, others []*orderItem) bool {
	var above map[ast.Expr]bool // the operators above u
	for st := u.up; st != nil; st = st.up {
		if above == nil {
			above = make(map[ast.Expr]bool)
		}
		above[st.node] = true
	}
	before := func(b *orderItem) bool {
		// An operator above u runs after its operands; a list that C++ evaluates in order
		// keeps the order of its expressions. Everything else is unordered in C++ (or
		// ordered only since C++17, as s[i], which gocpp doesn't rely on).
		return above[b.e] || sc.seq && u.root < b.root
	}
	for _, b := range later {
		if conflict(u.eff, b.eff) && !before(b) {
			return false
		}
	}
	for _, b := range others {
		if conflict(u.eff, b.eff) && !before(b) {
			return false
		}
	}
	return true
}

// emitScope writes the temporaries of the scope to pre, and registers them (fg.temps).
func (fg *funcGen) emitScope(sc *orderScope, pre *[]cppir.Stmt) {
	for _, c := range sc.calls {
		fg.ordered[c] = true
	}
	hoisted := make(map[*orderItem]bool, len(sc.hoist))
	for _, it := range sc.hoist {
		fg.emitItem(it, pre)
		hoisted[it] = true
	}
	for _, it := range sc.items {
		if it.unit && !hoisted[it] {
			// It stays in the expression; its own scopes are ordered in front (plan put the
			// units before it into temporaries).
			fg.ordered[it.e] = true
			for _, in := range it.inner {
				fg.emitScope(in, pre)
			}
		}
	}
}

// emitItem writes the temporary of one item. The statements have no place of their own:
// they go on the line of their statement (a Line or an inline Block).
func (fg *funcGen) emitItem(it *orderItem, pre *[]cppir.Stmt) {
	fg.ordered[it.e] = true
	if it.block {
		// r := x; if r { r = y } (for ||: if !r)
		fg.emitScope(it.inner[0], pre)
		t := fg.names.temp("t")
		*pre = append(*pre, varDecl(cppir.Pos{}, typeName("bool"), t, fg.expr(it.bin.X)))
		var inner []cppir.Stmt
		fg.emitScope(it.inner[1], &inner)
		inner = append(inner, &cppir.ExprStmt{X: &cppir.Assign{Op: "=", X: cxxName(t), Y: fg.expr(it.bin.Y)}})
		var cond cppir.Expr = cxxName(t)
		if it.bin.Op == token.LOR {
			cond = &cppir.Unary{Op: "!", X: cond}
		}
		*pre = append(*pre, &cppir.If{Cond: cond, Then: &cppir.Block{Stmts: inner, Inline: true}})
		fg.temps[it.e] = t
		return
	}
	for _, in := range it.inner {
		fg.emitScope(in, pre)
	}
	typ := "auto"
	if it.bin != nil {
		typ = "bool"
	}
	x := fg.exactExpr(it.e)
	t := fg.names.temp("t")
	*pre = append(*pre, varDecl(cppir.Pos{}, typeName(typ), t, x))
	fg.temps[it.e] = t
}

// withPre returns the C++ statement s with the statements pre in front of it: in an inline
// block at pos, so that a goto doesn't jump over the temporaries' initializations (stmt.go).
func withPre(pos cppir.Pos, pre []cppir.Stmt, s cppir.Stmt) cppir.Stmt {
	if len(pre) == 0 {
		return s
	}
	return &cppir.Block{Pos: pos, Stmts: append(pre, s), Inline: true}
}
