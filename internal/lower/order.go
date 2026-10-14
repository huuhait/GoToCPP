package lower

import (
	"go/ast"
	"go/constant"
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
//   - a slice expression s[a:b] (Phase 03), with its bounds: gc copies it into a temporary
//     in its place, so it reads s and panics there, and its bounds that are not a name, a
//     constant or len or cap of a name are copied in their places first (collect, cheap);
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
//   - a read of a boxed variable (box.go) and a read through a pointer, *p (Phase 03 step
//     3.3): a call can change them through a pointer, and gc reads them after the calls of
//     their scope too, like a package variable: in x + f(), *p + f(), and y, z := x, f(), f
//     runs first and the new value is read (and *p's nil check panics after f ran);
//   - an operation that can panic: an integer division or remainder by a non-constant
//     (gp::div), a shift by a non-constant signed count (gp::shl), and s[i] (out of range). gc
//     evaluates them after the calls of their scope: a/b + f() calls f first, then panics;
//   - an element of a slice, s[i] (step 3.5), which a call can change through another slice of
//     the same array: a read, like *p, also of a local slice; string(b) reads b's bytes;
//   - an element of an array (step 3.6, arrays.go): a read of the variable it is in (so after
//     the calls for a package or a boxed array, anywhere for a plain local), or through a
//     pointer, p[i], like *p; it panics for an index that is not a constant (and a nil p);
//     [N]T(s) reads s's elements, and both [N]T(s) and (*[N]T)(s) panic for a short s, after
//     the calls.
//
// Slices add units (step 3.5), each evaluated in its place among the calls, as gc copies them
// into temporaries: append, copy and clear (calls: they write elements that other slices can
// see), make (it can panic), cap (like len), []byte(s) and []rune(s) (gc's OSTR2BYTES and
// OSTR2RUNES). The elements of a composite literal are ordered like the operands of an
// operator (gc reads the variables in them after the calls).
//
// Maps (step 3.7, maps.go) add a read and units: m[k] reads the map's table, which any call can
// change, after the calls of its scope (gc doesn't copy OINDEXMAP), and len(m) and make are
// evaluated in their places among the calls; a map literal is evaluated as a whole in its place
// among the calls, entry by entry (collectMapLit).
//
// A divergence (the manager's decision, 2.5 review): gc also evaluates two such operations
// from left to right (with both out of range, s[i] == s[j] reports i), but gocpp doesn't keep
// that order: it would need temporaries in common code (fmt.Println(s[i], s[j])), and the
// only difference is which of the two panic messages is printed. Calls, output and the values
// that are read keep Go's order.
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
	force bool            // a call that must go into a temporary (fg.force)
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
	return fg.analyzeCopied(seq, list, func(i int, e ast.Expr) bool { return i >= iface && fg.copied(e) })
}

// analyzeCopied is analyze, where an expression of the list for which copy says so is a unit
// of its own: gc copies it into a temporary in its place among the calls (an argument of fmt's
// ...any, a bound of a slice expression).
func (fg *funcGen) analyzeCopied(seq bool, list []ast.Expr, copy func(i int, e ast.Expr) bool) *orderScope {
	sc := &orderScope{seq: seq}
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
	case *ast.Ident, *ast.StarExpr:
		return false // a variable: addressable (a local that is not boxed is copied, unseen)
	case *ast.IndexExpr:
		if fg.addressable(x) {
			return false // an element of a slice or of an addressable array: like *p
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
				if fn.Name() == "Printf" {
					iface = 1 // the format is a string
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
		// other slices can see.
		args := fg.analyzeArgs(false, x.Args, iface)
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
		// are constants.
		for i, elt := range x.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				elt = kv.Value
			}
			fg.collect(sc, elt, root, &orderStep{node: e, up: up, elem: i + 1})
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
		if u.block || u.force {
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
	var elems map[ast.Expr]int  // the composite literals above u: the element u is in
	for st := u.up; st != nil; st = st.up {
		if above == nil {
			above = make(map[ast.Expr]bool)
		}
		above[st.node] = true
		if st.elem > 0 {
			if elems == nil {
				elems = make(map[ast.Expr]int)
			}
			elems[st.node] = st.elem
		}
	}
	before := func(b *orderItem) bool {
		// An operator above u runs after its operands; a list that C++ evaluates in order
		// keeps the order of its expressions, also the braced list of a composite literal
		// ([]int{f(), g()}). Everything else is unordered in C++ (or ordered only since
		// C++17, as s[i], which gocpp doesn't rely on).
		if above[b.e] || sc.seq && u.root < b.root {
			return true
		}
		for st := b.up; st != nil && elems != nil; st = st.up {
			if e, ok := elems[st.node]; ok && st.elem > e {
				return true
			}
		}
		return false
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
	if lit, ok := it.e.(*ast.CompositeLit); ok && isMap(fg.g.info.TypeOf(lit)) {
		fg.emitMapLit(it, lit, pre) // maps.go
		return
	}
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
