package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

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
//   - a slice expression s[a:b] (Phase 03), with its bounds: gc copies it into a temporary
//     in its place, so it reads s and panics there, and its bounds that are not a name, a
//     constant or len or cap of a name are copied in their places first (collect, cheap);
//   - an argument of fmt's Print and Sprint functions (a ...any parameter) that is not a
//     variable and whose value gc doesn't convert by value: so fmt.Println(str[i], grow())
//     panics before grow runs, fmt.Println(x == 1, f()) compares the old x and
//     fmt.Println(m[k], f()) of a map of arrays prints the old m[k], while fmt.Println(x, f())
//     prints the new x. gc's rule (walk/order.go, OCONVIFACE and addrTemp): a conversion of a
//     value to an interface copies the value into a temporary in its place when the value is
//     not addressable (not a variable; a local variable is copied too, which only a closure
//     can see: inplace.go approximates gc's rule for it) and its type is not one that the
//     runtime converts by value: a 2-, 4- or 8-byte scalar, a string or a slice
//     (dataWordFuncName; convCopied in structs.go has the rule in Go's sizes): so bools, int8,
//     uint8, structs, arrays, pointers and maps are copied. Only fmt's
//     arguments are converted to interfaces in Phase 03; Phase 05 must extend this to every
//     conversion to an interface (any parameters, assignments to interface variables, panic).
//
// The rest stays where it is, and its order matters only where it can be seen:
//   - a read of a package variable, which any call can change (Phase 03: collect sees only
//     the package's own variables, an identifier; a variable of another package, pkg.V, is a
//     selector, which must become a read too);
//   - a read of a boxed variable (box.go) and a read through a pointer, *p (Phase 03 step
//     3.3): a call can change them through a pointer, and gc reads them after the calls of
//     their scope too, like a package variable: in x + f(), *p + f(), and y, z := x, f(), f
//     runs first and the new value is read (and *p's nil check panics after f ran);
//   - an operation that can panic: an integer division or remainder by a non-constant
//     (gp::div), a shift by a non-constant signed count (gp::shl), and s[i] (out of range). gc
//     evaluates them after the calls of their scope: a/b + f() calls f first, then panics
//     (and two of them in gc's order: see below);
//   - an element of a slice, s[i] (step 3.5), which a call can change through another slice of
//     the same array: a read, like *p, also of a local slice; string(b) reads b's bytes;
//   - an element of an array (step 3.6, arrays.go): a read of the variable it is in (so after
//     the calls for a package or a boxed array, anywhere for a plain local), or through a
//     pointer, p[i], like *p; it panics for an index that is not a constant (and a nil p);
//     [N]T(s) reads s's elements, and both [N]T(s) and (*[N]T)(s) panic for a short s, after
//     the calls;
//   - a field (step 3.8, structs.go): a read of the variable it is in, and through a pointer
//     (p.f, also an embedded pointer), like *p, a read that panics for a nil p.
//
// Slices add units (step 3.5), each evaluated in its place among the calls, as gc copies them
// into temporaries: append, copy and clear (calls: they write elements that other slices can
// see), make (it can panic), cap (like len), []byte(s) and []rune(s) (gc's OSTR2BYTES and
// OSTR2RUNES), min and max (step 3.15, like len), and complex, real and imag (step 3.15c,
// like len). The elements of a composite literal are ordered like the operands of an operator
// (gc reads the variables in them after the calls).
//
// A call of a func value (step 3.11, closures.go) has the func value in its own scope, first.
// gc evaluates a func value that has a call in it (f(), fs[i()], m[k()]) as a whole into a
// temporary before the arguments (typecheck's RewriteNonNameCall), so it is a unit of its own
// there (calleeCopied); any other one (f, fs[i], m[k], s.f) is read after the calls of the
// arguments, like any variable: f(g()) with a g that changes the package variable f calls the
// new f, fs[i()](g()) the old element.
//
// Maps (step 3.7, maps.go) add a read and units: m[k] reads the map's table, which any call can
// change, after the calls of its scope (gc doesn't copy OINDEXMAP), and len(m) and make are
// evaluated in their places among the calls; a map literal is evaluated as a whole in its place
// among the calls, entry by entry (collectMapLit).
//
// Two operations that can panic are evaluated in gc's order too (step 3.12; until then this was
// divergence D22), because the order decides which panic message is printed: with both out of
// range, s[i] == a[j] reports i. gc evaluates them after the units of their scope (a key that
// gc's mapKeyTemp copies is one: orderKey), from left to right (post order), with exceptions
// (goOrder). In one expression, the key of a map read and the operands of an ordering comparison
// of strings come first (walkFirst: walk makes them runtime calls and walkCall1 copies their
// arguments first). And a scope ranks its items (orderScope.rank): the operands of the target
// of x = v come after v (its SSA evaluates v first, then the place), except the parts that
// walkAssign's safeExpr copies first (storeRank: s[a[i]&1] = v checks i before v); x op= y and
// x++ copy those parts, then evaluate the rest of x, then y (opRank); the key of a map store
// m[k] = v and of m[k] op= v for an op other than / and % comes after v (safeMapRHS copies v
// first), also in each entry of a map literal (stored as m[k] = v); and a func value without a
// call comes after its call's arguments (walkCall1 copies the arguments that can panic first),
// but not in a defer statement, which evaluates its func value and receiver first (deferCall).
// Where C++ could evaluate two of them in another order (the arguments of a call, the operands of
// an operator), the first goes before the expression (planPanics), after all the units:
//
//	fmt.Println(s[i], a[j])   { auto gp_t0 = s[i]; gp::fmtshim::Println(gp_t0, a[j]); }
//	s[i] += a[j]              { auto& gp_r0 = s[i]; gp_r0 = gp_r0 + a[j]; }
//
// The target of op= is bound once by reference (emitPanic). Another place that the expression
// uses as one (the operand of &, a sliced array: place) and a value that isn't cheap to copy (an
// array, a struct) is only checked there and evaluated again in the expression, with the same
// result (never for an expression with a call: assertNoCall). An assignment with several targets
// has its own order, gc's ascompatee (bind, stmt.go: keepPanics).
//
// Reads of the function's own variables that are not boxed are not effects: such a variable
// can change only by an assignment, and an expression has none. Every variable whose address
// is taken, or that a closure captures and that changes, is boxed (box.go), and a read of it is
// one (above). A statement that assigns several variables (a, b = b, a) keeps its own rule
// (bind, stmt.go). Constants are never effects. Nor is &x: the box of x, and the address of a
// package variable, never change while a statement runs.
//
// A call in the target of x op= y (*f() += 1) always goes into a temporary (fg.force), because
// x op= y is x = x op y, which evaluates x twice.
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
//   - the right side of && or ||: inside an if on the left side's value;
//   - the operations that can panic and go before the expression (planPanics): after the units
//     of their scope, in the same place.
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
// have an effect: one of them is a call, or both can panic (then the order decides which
// panic message is printed).
func conflict(a, b effect) bool {
	return a.call || b.call || a.panics && b.panics
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
	force bool            // a call that must go into a temporary (fg.force)
	recv  bool            // the read of a method call's receiver through a pointer (analyzeMethod)
	first bool            // gc evaluates it before the other items of its expression (walkFirst)
}

// An orderStep is one operator above an item: a linked list up to the top of its expression.
// For an element of a composite literal, node is the literal and elem the element's index
// (+1): C++ evaluates the elements of its braced list in order.
type orderStep struct {
	node ast.Expr
	up   *orderStep
	elem int
}

// An orderScope is one scope of the order: a list of expressions that C++ evaluates in
// order (seq) or in any order, with the items found in them, in post order.
type orderScope struct {
	seq   bool
	items []*orderItem
	hoist []*orderItem // the units that go into temporaries, in Go's order
	pre   bool         // writing the scope needs statements before it
	calls []ast.Expr   // the calls in the scope that are not units (len of a local): ordered too

	// rank says when Go evaluates the item it, which is not a unit, against the other such
	// items (nil: all 0): the items of a lower rank come first, and those of one rank are in
	// post order. The operands of the target of x = v come after v (except their parts that gc
	// copies first: storeRank), the func value of a call after the arguments (rootRank).
	rank func(it *orderItem) int
	// hoistRoots: the units of the expressions before this index of the list always go into
	// temporaries (bind: gc's order pass evaluates every call before the stores).
	hoistRoots int
	// copy reports whether the expression e at the index i of the list is a unit of its own,
	// evaluated in its place among the calls (analyzeWith; nil: none is): a key that gc's
	// mapKeyTemp copies (orderKey).
	copy func(i int, e ast.Expr) bool
	// objFirst: the expression root 0 is the object of a C++ member call, x.M(args), which
	// C++ evaluates before the arguments (the other roots).
	objFirst bool
	// keepPanics: the panicking items are left where they are (bind orders them itself).
	keepPanics bool
	// panics are the items that are not units but can panic and go before the expression,
	// after the units, in Go's order (planPanics).
	panics []*orderItem
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

// goOrder returns the items that are not units in the order in which Go evaluates them:
// in post order, except that in the items of one expression of the list the parts that gc's walk
// evaluates first (walkFirst) go before the others, and then by rank (orderScope.rank).
func goOrder(items []*orderItem, rank func(*orderItem) int) []*orderItem {
	out := make([]*orderItem, 0, len(items))
	for i := 0; i < len(items); {
		j := i
		for j < len(items) && items[j].root == items[i].root {
			j++
		}
		for _, it := range items[i:j] {
			if it.first {
				out = append(out, it)
			}
		}
		for _, it := range items[i:j] {
			if !it.first {
				out = append(out, it)
			}
		}
		i = j
	}
	if rank != nil {
		slices.SortStableFunc(out, func(a, b *orderItem) int { return rank(a) - rank(b) })
	}
	return out
}

// walkFirst reports whether gc evaluates the item e, below the operators up, before the other
// items of its expression that are not units: gc's walk turns a map read m[k] and an ordering
// comparison of strings (<, <=, >, >=) into calls of the runtime, and walkCall1 copies their
// arguments that can panic into temporaries in front of the statement, so the panics in a map
// read's key and in the operands of such a comparison come first: s[i] + m[a[j]] checks j
// first. (The arguments of fmt's and println's calls are such calls too, but they are copied
// in order, which is post order already.)
func (fg *funcGen) walkFirst(e ast.Expr, up *orderStep) bool {
	for st := up; st != nil; st = st.up {
		switch x := st.node.(type) {
		case *ast.IndexExpr:
			if isMap(fg.g.info.TypeOf(x.X)) && !fg.stores[x] && within(e, x.Index) {
				return true
			}
		case *ast.BinaryExpr:
			switch x.Op {
			case token.LSS, token.LEQ, token.GTR, token.GEQ:
				if isString(fg.g.info.TypeOf(x.X)) {
					return true
				}
			}
		}
	}
	return false
}

// within reports whether the expression e is a part of the expression x.
func within(e, x ast.Expr) bool {
	return x.Pos() <= e.Pos() && e.End() <= x.End()
}

// rootRank returns a rank function (orderScope) that puts the items of the expressions before
// the index n of the scope's list after the others.
func rootRank(n int) func(*orderItem) int {
	return func(it *orderItem) int {
		if it.root < n {
			return 1
		}
		return 0
	}
}

// gcCopied reports whether gc's safeExpr (walk/expr.go) copies e, a part of the target of a
// store, into a temporary before the store's value is evaluated: e is not a name, a constant,
// len or cap, a field, *p or an index of an array, a slice or a string (whose own operands it
// looks at in turn). A map read is copied: walk makes it a call of the runtime.
func (fg *funcGen) gcCopied(e ast.Expr) bool {
	e = ast.Unparen(e)
	if fg.temps[e] != "" || fg.constValue(e) != nil {
		return false
	}
	switch x := e.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.StarExpr:
		return false
	case *ast.IndexExpr:
		return isMap(fg.g.info.TypeOf(x.X))
	case *ast.CallExpr:
		if fg.sameConversion(x) {
			return false // no conversion in gc: partCopies looks at its operand
		}
		return !fg.isBuiltin(x, "len") && !fg.isBuiltin(x, "cap")
	}
	return true
}

// sameConversion reports whether c converts a value to its own type (uint(u) of a uint u),
// which gc's typecheck drops.
func (fg *funcGen) sameConversion(c *ast.CallExpr) bool {
	return fg.g.info.Types[c.Fun].IsType() && len(c.Args) == 1 &&
		types.Identical(fg.g.info.TypeOf(c), fg.g.info.TypeOf(c.Args[0]))
}

// targetCopies returns the parts of the target e of a store that gc copies into temporaries
// before the store's value (gcCopied, walkAssign's safeExpr), in its order. The target itself
// is the place (a map entry m[k] too, whose own order is maps.go's).
func (fg *funcGen) targetCopies(e ast.Expr) []ast.Expr {
	switch x := ast.Unparen(e).(type) {
	case *ast.IndexExpr:
		return append(fg.partCopies(x.X), fg.partCopies(x.Index)...)
	case *ast.StarExpr:
		return fg.partCopies(x.X)
	case *ast.SelectorExpr:
		return fg.partCopies(x.X)
	}
	return nil
}

// partCopies is targetCopies for a part e of a target.
func (fg *funcGen) partCopies(e ast.Expr) []ast.Expr {
	if fg.gcCopied(e) {
		return []ast.Expr{e}
	}
	if c, ok := ast.Unparen(e).(*ast.CallExpr); ok && fg.sameConversion(c) {
		return fg.partCopies(c.Args[0])
	}
	return fg.targetCopies(e)
}

// inCopies reports whether the item it is a part of one of the expressions copies.
func inCopies(it *orderItem, copies []ast.Expr) bool {
	for _, c := range copies {
		if within(it.e, c) {
			return true
		}
	}
	return false
}

// assertNoCall reports an internal error if e, which an order statement evaluates twice (a check
// in front, then the expression), still has a call: it would run twice.
func (fg *funcGen) assertNoCall(e ast.Expr) {
	for _, it := range fg.analyze(false, []ast.Expr{e}).items {
		if it.eff.call {
			fg.g.errorf(e.Pos(), "internal error: an expression with a call is evaluated twice (order.go)")
			return
		}
	}
}

// orderWith is order for a scope with the options of sc (rank, copy, objFirst, keepPanics), which
// it fills.
func (fg *funcGen) orderWith(sc *orderScope, list ...ast.Expr) []cppir.Stmt {
	var pre []cppir.Stmt
	fg.emitScope(fg.analyzeWith(sc, list), &pre)
	return pre
}

// analyzeWith is analyze for a scope with the options of sc, which it fills and returns.
func (fg *funcGen) analyzeWith(sc *orderScope, list []ast.Expr) *orderScope {
	copy := sc.copy
	if copy == nil {
		copy = func(int, ast.Expr) bool { return false }
	}
	fg.collectScope(sc, list, copy)
	sc.plan()
	return sc
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
	return fg.analyzeCopied(seq, list, func(i int, e ast.Expr) bool { return i >= iface && fg.copied(e) })
}

// analyzeCopied is analyze, where an expression of the list for which copy says so is a unit
// of its own: gc copies it into a temporary in its place among the calls (an argument of fmt's
// ...any, a bound of a slice expression).
func (fg *funcGen) analyzeCopied(seq bool, list []ast.Expr, copy func(i int, e ast.Expr) bool) *orderScope {
	sc := fg.collectScope(&orderScope{seq: seq}, list, copy)
	sc.plan()
	return sc
}

// collectScope adds the items of the expressions list to sc, as analyzeCopied does, and returns
// it, without planning it.
func (fg *funcGen) collectScope(sc *orderScope, list []ast.Expr, copy func(i int, e ast.Expr) bool) *orderScope {
	for i, e := range list {
		if copy(i, e) {
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
	return sc
}

// copied reports whether gc copies the argument e of fmt's ...any into a temporary in its
// place among the calls: a value that is not addressable (and not a unit already), unless gc
// converts it by value (convCopied): a bool, a comparison, a byte s[i] of a string, a map read
// m[k] of an array, a struct, a pointer or a map, a composite literal.
func (fg *funcGen) copied(e ast.Expr) bool {
	e = ast.Unparen(e)
	if _, ok := fg.temps[e]; ok || fg.constValue(e) != nil {
		return false
	}
	switch x := e.(type) {
	case *ast.Ident:
		// A variable: addressable, read after the calls; but gc copies a local in place, which
		// a closure can see (inplace.go). (A local that is not boxed is copied too, unseen.)
		v, ok := fg.g.info.Uses[x].(*types.Var)
		return ok && fg.inPlaceLocal(v)
	case *ast.StarExpr:
		return false // a variable: addressable
	case *ast.IndexExpr, *ast.SelectorExpr:
		if fg.addressable(x) {
			// An element of a slice or of an addressable array, a field: like *p; but a part
			// of a local that gc copies in place is copied too (inplace.go: inPlacePart).
			return fg.inPlacePart(x)
		}
	case *ast.CallExpr:
		if !fg.g.info.Types[x.Fun].IsType() {
			return false // a call
		}
	case *ast.BinaryExpr:
		if x.Op == token.LAND || x.Op == token.LOR {
			return false
		}
	}
	return convCopied(fg.g.info.TypeOf(e)) // structs.go
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
		it.first = !it.unit && fg.walkFirst(e, up)
		sc.items = append(sc.items, it)
	}
	below := &orderStep{node: e, up: up} // for the operands of e
	switch x := e.(type) {
	case *ast.Ident:
		if v, ok := fg.g.info.Uses[x].(*types.Var); ok && (isPkgVar(v) || fg.isBoxed(v)) {
			add(&orderItem{eff: effect{read: true}})
		}
	case *ast.StarExpr:
		// *p reads what p points to, which a call can change, and panics for a nil p.
		fg.collect(sc, x.X, root, below)
		add(&orderItem{eff: effect{read: true, panics: true}})
	case *ast.CallExpr:
		if fg.g.info.Types[x.Fun].IsType() {
			to := fg.g.info.TypeOf(x)
			if len(x.Args) == 1 && isSlice(to) && isString(fg.g.info.TypeOf(x.Args[0])) {
				// []byte(s) and []rune(s): gc evaluates them in their places among the calls
				// (walk/order.go: OSTR2BYTES, OSTR2RUNES), like len.
				fg.collectInPlace(sc, x, x.Args, effect{}, root, up)
				return
			}
			for _, a := range x.Args {
				fg.collect(sc, a, root, up) // a conversion: its operand
			}
			if len(x.Args) == 1 && isString(to) && isSlice(fg.g.info.TypeOf(x.Args[0])) {
				// string(b) reads the bytes of b where it is, after the calls of its scope
				// (gc doesn't copy OBYTES2STR): memory that a call can change.
				add(&orderItem{eff: effect{read: true}})
			}
			if eff := fg.conversionEffect(x); eff.any() {
				add(&orderItem{eff: eff}) // [N]T(s), (*[N]T)(s): arrays.go
			}
			return
		}
		if fg.isBuiltin(x, "new") {
			sc.calls = append(sc.calls, x) // new(T) has no effect: its argument is a type
			return
		}
		if fg.isBuiltin(x, "min") || fg.isBuiltin(x, "max") {
			// min and max have no effect of their own, but gc evaluates them in their places
			// among the calls, like len (walk/order.go: OMIN, OMAX).
			fg.collectInPlace(sc, x, x.Args, effect{}, root, up)
			return
		}
		if fg.isBuiltin(x, "complex") || fg.isBuiltin(x, "real") || fg.isBuiltin(x, "imag") {
			// complex, real and imag have no effect of their own, but gc evaluates them in
			// their places among the calls, like len (walk/order.go: OCOMPLEX, OREAL, OIMAG):
			// real(g) + f() reads the package variable g before f runs (complex.go).
			fg.collectInPlace(sc, x, x.Args, effect{}, root, up)
			return
		}
		if fg.isBuiltin(x, "make") {
			// make([]T, n): in its place among the calls (gc copies it), and it panics for a
			// bad length (a constant one is checked by go/types, also against a constant
			// capacity). Its first argument is a type. make(map[K]V, n) never panics.
			var eff effect
			for _, a := range x.Args[1:] {
				if fg.constValue(a) == nil && !isMap(fg.g.info.TypeOf(x.Args[0])) {
					eff.panics = true
				}
			}
			fg.collectInPlace(sc, x, x.Args[1:], eff, root, up)
			return
		}
		iface := len(x.Args)
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			if fn, ok := fg.g.info.Uses[sel.Sel].(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" {
				iface = 0
				if fmtFormatted(fn.Name()) {
					iface = 1 // the format is a string (fmtcall.go)
				}
			}
		}
		if fg.isBuiltin(x, "len") || fg.isBuiltin(x, "cap") {
			// len and cap have no effect of their own, but gc evaluates them in their places
			// among the calls. len(m) reads the map's table, which a call can change.
			fg.collectInPlace(sc, x, x.Args, effect{read: isMap(fg.g.info.TypeOf(x.Args[0]))}, root, up)
			return
		}
		// A call; also append, copy and clear, which write the elements of an array that
		// other slices can see. A method's receiver is evaluated with the arguments
		// (methods.go). The func value of a call of one (closures.go) is in the call's scope,
		// before the arguments (calleeCopied).
		var args *orderScope
		if sel, s := fg.methodSel(x); sel != nil {
			args = fg.analyzeMethod(x, sel, s)
		} else if fg.funcValueCall(x) {
			// gc evaluates the arguments that can panic before a func value without a call
			// (walk's walkCall1 copies them into temporaries first): fs[i](a[j]) checks j first;
			// but a deferred call evaluates its func value first (deferStmt).
			list := append([]ast.Expr{x.Fun}, x.Args...)
			sc := &orderScope{}
			if x != fg.deferCall {
				sc.rank = rootRank(1) // the func value, root 0
			}
			args = fg.collectScope(sc, list, func(i int, e ast.Expr) bool { return i == 0 && fg.calleeCopied(e) })
			args.plan()
		} else {
			args = fg.analyzeArgs(false, x.Args, iface)
		}
		add(&orderItem{unit: true, inner: []*orderScope{args}, eff: effect{call: true}, force: fg.force[x]})
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
		if x.Op == token.AND {
			fg.collectAddr(sc, x.X, root, up)
			return
		}
		fg.collect(sc, x.X, root, up)
	case *ast.IndexExpr:
		if isMap(fg.g.info.TypeOf(x.X)) {
			fg.collectMapIndex(sc, x, root, up) // maps.go
			return
		}
		fg.collect(sc, x.X, root, below)
		fg.collect(sc, x.Index, root, below)
		eff := effect{panics: true}
		if isSlice(fg.g.info.TypeOf(x.X)) {
			// An element of a slice is memory that a call can change through another slice
			// of the array: read after the calls, like *p. (A string's bytes never change.)
			eff.read = true
		}
		if fg.inArray(x) {
			eff = fg.indexEffect(x) // arrays.go
		}
		if eff.any() {
			add(&orderItem{eff: eff})
		}
	case *ast.CompositeLit:
		if isMap(fg.g.info.TypeOf(x)) {
			fg.collectMapLit(sc, x, root, up) // maps.go
			return
		}
		// The elements, like the operands of an operator: gc orders them (and reads the
		// variables in them) as the operands of an expression. The keys of a slice literal
		// are constants; the values of a keyed struct literal are in the fields' order in
		// C++ (litElem, structs.go).
		for i, elt := range x.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				elt = kv.Value
			}
			fg.collect(sc, elt, root, &orderStep{node: e, up: up, elem: fg.litElem(x, i)})
		}
	case *ast.SelectorExpr:
		// x.f reads x (its own item), and through a pointer also memory that a call can
		// change, and panics for nil (structs.go).
		if fg.fieldSel(x) != nil {
			fg.collect(sc, x.X, root, below)
			if eff := fg.fieldEffect(x); eff.any() {
				add(&orderItem{eff: eff})
			}
		} else if s := fg.g.info.Selections[x]; s != nil && s.Kind() == types.MethodVal {
			fg.collectRecv(sc, x, s, root, up) // a method value's receiver (methods.go)
		}
	case *ast.SliceExpr:
		// gc evaluates a slice expression as a whole in its place among the calls (walk/
		// order.go copies it into a temporary), after its bounds, which it copies into
		// temporaries in their own places too unless they are a name, a constant or len or
		// cap of a name (cheapExpr): so fmt.Println(s[i:], f()) slices s before f runs (and panics
		// before it), and in s[n+0 : f()] n is read before f runs.
		list := []ast.Expr{x.X}
		for _, b := range []ast.Expr{x.Low, x.High, x.Max} {
			if b != nil {
				list = append(list, b)
			}
		}
		in := fg.analyzeCopied(false, list, func(i int, e ast.Expr) bool { return i > 0 && !fg.cheap(e) })
		eff := in.effect()
		// s[:] and s[0:] can't fail; the other forms check their bounds. p[:] of a pointer to
		// an array panics for a nil p.
		if x.High != nil || x.Low != nil && !isZero(fg.constValue(x.Low)) || arrayPtr(fg.g.info.TypeOf(x.X)) != nil {
			eff.panics = true
		}
		if !eff.any() {
			sc.calls = append(sc.calls, in.calls...)
			return
		}
		add(&orderItem{unit: true, eff: eff, inner: []*orderScope{in}})
	}
}

// calleeCopied reports whether gc evaluates the func value e of a call into a temporary before
// the call's arguments (typecheck's RewriteNonNameCall): e has a call in it (also of a builtin,
// but not new(T); not one inside a func literal, nor a conversion) or a receive.
func (fg *funcGen) calleeCopied(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if !fg.g.info.Types[x.Fun].IsType() && fg.g.info.Types[x].Value == nil && !fg.isBuiltin(x, "new") {
				found = true
			}
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				found = true
			}
		}
		return !found
	})
	return found
}

// collectInPlace adds the call or conversion x, whose operands are args, to the scope as an
// item that gc evaluates in its place among the calls (len, cap, make, []byte(s)), which has
// no effect of its own but eff: a unit, if it or its operands have an effect, or else an
// expression that can be evaluated anywhere (its calls are only marked ordered).
func (fg *funcGen) collectInPlace(sc *orderScope, x *ast.CallExpr, args []ast.Expr, eff effect, root int, up *orderStep) {
	in := fg.analyze(false, args)
	if eff = eff.or(in.effect()); !eff.any() {
		sc.calls = append(append(sc.calls, x), in.calls...)
		return
	}
	sc.items = append(sc.items, &orderItem{e: x, unit: true, eff: eff, root: root, up: up, inner: []*orderScope{in}, force: fg.force[x]})
}

// collectAddr adds the items of &e to the scope: those of what e is addressed through. &x of
// a variable (its box, or a package variable) has none; &*p reads p and panics if p is nil.
// (The other forms come with their types, addr.go.)
func (fg *funcGen) collectAddr(sc *orderScope, e ast.Expr, root int, up *orderStep) {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
	case *ast.StarExpr:
		fg.collect(sc, x.X, root, up)
		sc.items = append(sc.items, &orderItem{e: ast.Unparen(e), root: root, up: up, eff: effect{panics: true}})
	default:
		fg.collect(sc, e, root, up)
	}
}

// cheap reports whether gc leaves the bound e of a slice expression in place instead of
// copying it into a temporary (cmd/compile's cheapExpr): a name, a constant, or len or cap
// of a cheap expression.
func (fg *funcGen) cheap(e ast.Expr) bool {
	e = ast.Unparen(e)
	if _, ok := fg.temps[e]; ok || fg.constValue(e) != nil {
		return true
	}
	switch x := e.(type) {
	case *ast.Ident:
		return true
	case *ast.CallExpr:
		return (fg.isBuiltin(x, "len") || fg.isBuiltin(x, "cap")) && len(x.Args) == 1 && fg.cheap(x.Args[0])
	}
	return false
}

// isZero reports whether the constant v is 0 (false for nil, not a constant).
func isZero(v constant.Value) bool {
	return v != nil && constant.Sign(v) == 0
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
		b := underBasic(fg.g.info.TypeOf(e))
		return b != nil && b.Info()&types.IsInteger != 0
	case token.SHL, token.SHR:
		// An untyped count is a uint (typeOf).
		b := underBasic(fg.g.info.TypeOf(e.Y))
		return b != nil && b.Info()&(types.IsUnsigned|types.IsUntyped) == 0
	}
	return false
}

// plan chooses the temporaries of the scope (see the top of the file): the shortest leading
// part of its units, in Go's order, such that the C++ of the rest evaluates every remaining
// unit before everything that Go evaluates after it and whose order with it can be seen.
// Only units go into temporaries: the other items are evaluated after all units, in Go as
// in the C++ of the rest, except the panicking ones that C++ could evaluate in another order
// than Go (planPanics): those go before the expression too, after all the units.
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
	others = goOrder(others, sc.rank) // in Go's order
	// An && or || whose right side needs temporaries is an if: it can't stay inside the
	// C++ expression.
	n := 0
	for i, u := range units {
		if u.block || u.force || u.root < sc.hoistRoots {
			n = i + 1
		}
	}
	for i := len(units) - 1; i >= n; i-- {
		if !sc.first(units[i], units[i+1:], others) {
			n = i + 1
			break
		}
	}
	if !sc.keepPanics {
		if sc.panics = sc.planPanics(others); len(sc.panics) > 0 {
			n = len(units) // Go evaluates them after all the units
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
	sc.pre = n > 0 || len(sc.panics) > 0
	for _, u := range units[n:] {
		sc.pre = sc.pre || u.inner[0].pre
	}
}

// first reports whether C++ evaluates the unit u before the later units and the other items,
// wherever Go's order can be seen.
func (sc *orderScope) first(u *orderItem, later, others []*orderItem) bool {
	for _, b := range later {
		if conflict(u.eff, b.eff) && !sc.cxxBefore(u, b) {
			return false
		}
	}
	for _, b := range others {
		if conflict(u.eff, b.eff) && !sc.cxxBefore(u, b) {
			return false
		}
	}
	return true
}

// cxxBefore reports whether the C++ of the scope evaluates the item a before the item b, both
// in the expressions of the scope: an operator above a runs after its operands; a list that
// C++ evaluates in order keeps the order of its expressions, also the braced list of a
// composite literal ([]int{f(), g()}), and the object of a member call is evaluated before
// its arguments (objFirst). Everything else is unordered in C++ (or ordered only since C++17,
// as s[i] and a = b, which gocpp doesn't rely on here).
func (sc *orderScope) cxxBefore(a, b *orderItem) bool {
	if sc.seq && a.root < b.root || sc.objFirst && a.root == 0 && b.root > 0 {
		return true
	}
	elems := make(map[ast.Expr]int) // the composite literals above a: the element a is in
	for st := a.up; st != nil; st = st.up {
		if st.node == b.e {
			return true
		}
		if st.elem > 0 {
			elems[st.node] = st.elem
		}
	}
	for st := b.up; st != nil && len(elems) > 0; st = st.up {
		if e, ok := elems[st.node]; ok && st.elem > e {
			return true
		}
	}
	return false
}

// planPanics returns the items of others (the items that are not units, in Go's order) that
// can panic and must be evaluated before the scope's expression, so that C++ checks them in
// Go's order: fmt.Println(s[i], a[i]) checks s[i] first in Go, but g++ evaluates a call's
// arguments from right to left. The rest of the expression then reads their temporaries.
//
// An item p can stay in the expression if C++ evaluates it before each later one that can
// panic: one above it, a later one of a list that C++ evaluates in order (cxxBefore), or one
// in an item that goes before the expression after the one that p is in. So the items are
// chosen from the last to the first: when p's turn comes, the items above it and the later
// ones are chosen already. If any is chosen, Go evaluates it after all the units, so they go
// into temporaries first (plan).
func (sc *orderScope) planPanics(others []*orderItem) []*orderItem {
	var ps []*orderItem
	for _, it := range others {
		if it.eff.panics {
			ps = append(ps, it)
		}
	}
	if len(ps) < 2 {
		return nil
	}
	index := make(map[*orderItem]int, len(ps))
	byExpr := make(map[ast.Expr]*orderItem, len(ps))
	for i, p := range ps {
		index[p] = i
		byExpr[p.e] = p
	}
	chosen := make(map[*orderItem]bool)
	// in returns the chosen item that p is evaluated in: p itself (with self), or the nearest
	// chosen item above it; nil for the expression.
	in := func(p *orderItem, self bool) *orderItem {
		if self && chosen[p] {
			return p
		}
		for st := p.up; st != nil; st = st.up {
			if q := byExpr[st.node]; q != nil && chosen[q] {
				return q
			}
		}
		return nil
	}
	// before reports whether C++ evaluates p (not chosen yet) before q.
	before := func(p, q *orderItem) bool {
		cp, cq := in(p, false), in(q, true)
		switch {
		case cp == cq:
			return sc.cxxBefore(p, q) // in the same expression
		case cp == nil:
			return false // q's temporary comes first
		case cq == nil:
			return true
		}
		return index[cp] < index[cq] // the temporaries are in Go's order
	}
	for i := len(ps) - 2; i >= 0; i-- {
		for _, q := range ps[i+1:] {
			if !before(ps[i], q) {
				chosen[ps[i]] = true
				break
			}
		}
	}
	var list []*orderItem
	for _, p := range ps {
		if chosen[p] {
			list = append(list, p)
		}
	}
	return list
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
	for _, it := range sc.panics {
		fg.emitPanic(it, pre)
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
	if lit, ok := it.e.(*ast.CompositeLit); ok && isMap(fg.g.info.TypeOf(lit)) {
		fg.emitMapLit(it, lit, pre) // maps.go
		return
	}
	if it.block {
		// r := x; if r { r = y } (for ||: if !r)
		fg.emitScope(it.inner[0], pre)
		t := fg.names.temp("t")
		*pre = append(*pre, varDecl(cppir.Pos{}, fg.boolType(it.e), t, fg.expr(it.bin.X)))
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
	var typ cppir.Type = typeName("auto")
	if it.bin != nil {
		typ = fg.boolType(it.e)
	}
	x := fg.exactExpr(it.e)
	t := fg.names.temp("t")
	*pre = append(*pre, varDecl(cppir.Pos{}, typ, t, x))
	fg.temps[it.e] = t
}

// emitPanic writes the statement that evaluates the item it, which can panic, before its
// expression (planPanics): auto gp_t0 = s[i]; whose temporary the expression reads. The target
// of x op= y is bound by reference instead (heldPlace), and the receiver of a deferred value
// method through a pointer is checked (*p). Another place that the expression uses as one (the
// operand of &: places) or a value that is not cheap to copy is only checked,
// static_cast<void>(s[i]);, and evaluated again in the expression: that has the same result,
// since nothing between the two changes it (the calls of the scope came before).
func (fg *funcGen) emitPanic(it *orderItem, pre *[]cppir.Stmt) {
	if it.e == fg.opTarget && fg.heldPlace(it.e) {
		// The target of x op= y: bound once, auto& gp_r0 = s[i]; gp_r0 = gp_r0 + y;, since
		// nothing between its check and its store can change it or free it.
		r := fg.names.temp("r")
		*pre = append(*pre, varDecl(cppir.Pos{}, typeName("auto&"), r, fg.expr(it.e)))
		fg.temps[it.e] = r
		return
	}
	if it.recv {
		// The receiver of a deferred value method through a pointer: *p's nil check.
		x := it.e.(*ast.SelectorExpr)
		fg.assertNoCall(x.X) // evaluated again by the deferred call
		recv, ptr := fg.recvExpr(x, pathOf(fg.g.info.Selections[x]), false)
		if ptr {
			recv = deref(recv)
		}
		*pre = append(*pre, &cppir.ExprStmt{X: voidCast(recv)})
		return
	}
	if fg.place(it.e) || !cheapCopy(fg.g.info.TypeOf(it.e)) {
		fg.assertNoCall(it.e)
		*pre = append(*pre, &cppir.ExprStmt{X: voidCast(fg.expr(it.e))})
		return
	}
	t := fg.names.temp("t")
	*pre = append(*pre, varDecl(cppir.Pos{}, typeName("auto"), t, fg.exactExpr(it.e)))
	fg.temps[it.e] = t
}

// heldPlace reports whether the place e is held by something that outlives its statement (a
// reference to it stays valid): it is reached from a variable, a temporary of the order or a
// map's entry, through fields, elements and pointers, and not from a composite literal
// ([]int{1, 2}[i]), whose array only the statement's temporary holds.
func (fg *funcGen) heldPlace(e ast.Expr) bool {
	for {
		e = ast.Unparen(e)
		if _, ok := fg.temps[e]; ok {
			return true
		}
		switch x := e.(type) {
		case *ast.Ident:
			return true
		case *ast.IndexExpr:
			e = x.X
		case *ast.SelectorExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return false
		}
	}
}

// cheapCopy reports whether a value of type t is cheap to copy into a temporary: not an array
// or a struct (a basic type, a pointer, a string, a slice, a map, a func value).
func cheapCopy(t types.Type) bool {
	switch under(t).(type) {
	case *types.Array, *types.Struct, *types.Tuple:
		return false
	}
	return t != nil
}

// boolType returns the C++ type of the temporary of e, an && or ||: bool, or the named bool type
// (type ok bool, step 3.9) that it has.
func (fg *funcGen) boolType(e ast.Expr) cppir.Type {
	if t := fg.g.info.TypeOf(e); isNewtype(t) {
		return fg.g.cxxType(t, e.Pos())
	}
	return typeName("bool")
}

// withPre returns the C++ statement s with the statements pre in front of it: in an inline
// block at pos, so that a goto doesn't jump over the temporaries' initializations (stmt.go).
func withPre(pos cppir.Pos, pre []cppir.Stmt, s cppir.Stmt) cppir.Stmt {
	if len(pre) == 0 {
		return s
	}
	return &cppir.Block{Pos: pos, Stmts: append(pre, s), Inline: true}
}

// place reports whether the C++ of e is used as a place, not only for its value (emitPanic):
// the target of an assignment, x op= y, x++ and a range, the operand of &, an array that is
// sliced, the receiver of a pointer method that Go takes the address of, and the struct or
// array value that such a place is a field or an element of.
func (fg *funcGen) place(e ast.Expr) bool {
	if fg.top == nil {
		return true
	}
	if fg.places == nil || fg.placesTop != fg.top {
		fg.places, fg.placesTop = findPlaces(fg.g.info, fg.top), fg.top
	}
	return fg.places[e]
}

// findPlaces returns the expressions of the code top that are used as places (place).
func findPlaces(info *types.Info, top ast.Node) map[ast.Expr]bool {
	places := make(map[ast.Expr]bool)
	mark := func(e ast.Expr) {
		for {
			e = ast.Unparen(e)
			places[e] = true
			switch x := e.(type) {
			case *ast.SelectorExpr:
				if s := info.Selections[x]; s == nil || s.Kind() != types.FieldVal || s.Indirect() {
					return
				}
				e = x.X
			case *ast.IndexExpr:
				if !isArray(info.TypeOf(x.X)) {
					return
				}
				e = x.X
			default:
				return
			}
		}
	}
	ast.Inspect(top, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				mark(x.X)
			}
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				mark(l)
			}
		case *ast.IncDecStmt:
			mark(x.X)
		case *ast.RangeStmt:
			for _, t := range []ast.Expr{x.Key, x.Value} {
				if t != nil {
					mark(t)
				}
			}
		case *ast.SliceExpr:
			if isArray(info.TypeOf(x.X)) {
				mark(x.X)
			}
		case *ast.SelectorExpr:
			if s := info.Selections[x]; s != nil && s.Kind() == types.MethodVal {
				if sig, ok := s.Obj().Type().(*types.Signature); ok && sig.Recv() != nil && isPointer(sig.Recv().Type()) && !isPointer(info.TypeOf(x.X)) {
					mark(x.X) // &x of x.PM
				}
			}
		}
		return true
	})
	return places
}
