package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"sync/atomic"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// Boxes of C/C++ objects (Phase 04 README §2.5, step 4.8; runtime: gocpp/interop.hpp).
//
// A C/C++ object can be changed by anything that has its address: a member call (v.push_back),
// an operator, a function that takes a reference (std.swap(v, w)). So the Phase 03 rule "a
// variable that is never assigned is captured by copy" is wrong for it, and gocpp can't see
// what a C++ call does. An opaque local (a variable whose type holds a C/C++ object by value:
// cObject) that a func literal uses, or that a deferred call is made on, is therefore always
// boxed, whatever the closure does with it. The closure copies the box (a gp::ptr<T>), and the
// closure and the function share the one object, as they share a Go variable. The same box makes
// the object outlive the block that declares it, which `defer v.clear()` needs: C++ destroys a
// local at the end of its block, before the function's deferred calls run.
//
// Capture by copy would be right only for a type that can be copied, and whether a C++ type can
// be copied is the C++ compiler's to say, not gocpp's. So a closure never captures an opaque local
// by copy: the policy table in docs/spec/interop.md ("Types that can't be copied") lists what
// each construct does.
//
// The box of a C/C++ object is not gp::new_<T>(value), which would move the value into the box (a
// type that can't be moved, std::mutex, couldn't be boxed): the object is built in the box from
// the prvalue of its initializer (gp::cgo::box, guaranteed copy elision).
//
//	v := std.vector[int]{}                auto v = gp::cgo::box([&] { return ::std::vector<int64_t>{}; });
//	var m std.mutex                       gp::ptr<::std::mutex> m = gp::cgo::new_box<::std::mutex>();
//	var n C.int = f()                     gp::ptr<int> n = gp::cgo::box_as<int>([&]() -> int { return ::f(); });

// cObject reports whether a variable of type t holds a C/C++ object by value: a C/C++ type, a
// raw C/C++ pointer (*lib.FILE: the pointer is the object), a struct that has one in a field, or
// an array of them. A slice, a map, a channel and a func hold their elements by reference, and a
// pointer to a gocpp type is a gocpp pointer: they follow Phase 03's rules.
func cObject(t types.Type) bool {
	return cObjectSeen(t, make(map[types.Type]bool))
}

func cObjectSeen(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Kind() == types.Invalid
	case *types.Pointer:
		return !complete(u.Elem())
	case *types.Struct:
		for i := range u.NumFields() {
			if cObjectSeen(u.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Array:
		return cObjectSeen(u.Elem(), seen)
	}
	return false
}

// isCBox reports whether the variable v lives in a box of a C/C++ object (cBox).
func (fg *funcGen) isCBox(v *types.Var) bool {
	return fg.isBoxed(v) && cObject(v.Type())
}

// isAutoType reports whether typ is auto: the type of a variable declared from a C/C++ value
// without a type (x := lib.f()).
func isAutoType(typ cppir.Type) bool {
	n, ok := typ.(*cppir.TypeName)
	return ok && n.Name == "auto" && len(n.Args) == 0
}

// cBox returns the box of a variable of the C/C++ type typ (auto if gocpp doesn't know it) with
// the value: gp::cgo::box_as<T>([&]() -> T { return value; }), or gp::cgo::box([&] { return
// value; }) for auto, which builds the object in the box from the prvalue. The zero value (a nil
// value) is gp::cgo::new_box<T>().
func cBox(typ cppir.Type, value cppir.Expr) cppir.Expr {
	if value == nil {
		return templateCall("gp::cgo::new_box", []cppir.Node{typ})
	}
	auto := isAutoType(typ)
	lam := cBoxLambda(typ, value)
	if auto {
		return call("gp::cgo::box", lam)
	}
	return templateCall("gp::cgo::box_as", []cppir.Node{typ}, lam)
}

// cBoxLambda returns the lambda [&]() -> T { return value; } that builds the object of cBox in place
// (no result type for auto).
func cBoxLambda(typ cppir.Type, value cppir.Expr) *cppir.Lambda {
	auto := isAutoType(typ)
	if l, ok := value.(*cppir.InitList); ok && l.Type == nil && !auto {
		value = &cppir.InitList{Type: typ, Elems: l.Elems} // T{a, b}: explicit constructors too
	}
	lam := &cppir.Lambda{
		Default: "&",
		Body:    &cppir.Block{Inline: true, Stmts: []cppir.Stmt{&cppir.Return{X: value}}},
	}
	if !auto {
		lam.Result = typ
	}
	return lam
}

// cBoxDecl returns the declaration of the variable name that is the box of a C/C++ object of the
// type typ, with the value (nil: the zero value): auto for a type gocpp doesn't know.
func cBoxDecl(pos cppir.Pos, typ cppir.Type, name string, value cppir.Expr) cppir.Stmt {
	var t cppir.Type = ptrType(typ)
	if isAutoType(typ) {
		t = typ
	}
	return varDecl(pos, t, name, cBox(typ, value))
}

// boxType returns the C++ type of the variable of the box of a C/C++ object whose type is typ:
// gp::ptr<T>, or auto.
func boxType(typ cppir.Type) cppir.Type {
	if isAutoType(typ) {
		return typ
	}
	return ptrType(typ)
}

// boxDeferred marks the variables that a deferred call needs in a box (boxer in box.go): every
// C/C++ local that the call's object or arguments mention (the object of a member call, defer
// v.clear(), the path of defer h.v.clear(), a local in an argument, defer lib.f(v.data()): a
// pointer or reference into it is stored in the frame), except what an argument copies: a bare
// variable, and a value of a type that has no C/C++ part (defer fmt.Println(int(v.size()))) or a
// conversion to a C number (C.int(n)), which can't refer to a local. The frame holds the boxes
// (deferThunk), because C++ destroys a local when its block ends, which is before the function's
// deferred calls run.
func (b *boxer) boxDeferred(c *ast.CallExpr) {
	for _, v := range deferLocals(b.info, c) {
		b.mark(v, boxDefer)
	}
}

// deferLocals returns the C/C++ locals that the deferred call c keeps alive (boxDeferred), in the
// order of their first mention. In the object (the function of the call) every mention counts. In
// an argument only a path counts, whose pointer or reference stays reachable: &v, v.f, v.data(),
// v[i], *v. A plain use of a variable (n in Vec{x: n}, f(n), n + 1) copies its value.
func deferLocals(info *tinfo, c *ast.CallExpr) []*types.Var {
	b := &boxer{info: info}
	var out []*types.Var
	add := func(id *ast.Ident) {
		if v := b.localVar(id); v != nil && cObject(v.Type()) && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	ast.Inspect(c.Fun, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false // (its captures are boxed already)
		case *ast.Ident:
			add(x)
		}
		return true
	})
	for _, a := range c.Args {
		if copiedArg(info, a) {
			continue
		}
		ast.Inspect(a, func(n ast.Node) bool {
			var path ast.Expr
			switch x := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.SelectorExpr, *ast.IndexExpr, *ast.SliceExpr, *ast.StarExpr:
				path = x.(ast.Expr)
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					path = x
				}
			}
			for path != nil {
				switch y := ast.Unparen(path).(type) {
				case *ast.Ident:
					add(y)
					path = nil
				case *ast.SelectorExpr:
					path = y.X
				case *ast.IndexExpr:
					path = y.X
				case *ast.SliceExpr:
					path = y.X
				case *ast.StarExpr:
					path = y.X
				case *ast.UnaryExpr:
					path = y.X
				default:
					path = nil
				}
			}
			return true
		})
	}
	return out
}

// copiedArg reports whether the argument a of a deferred call is evaluated and copied into the
// frame without anything of a C/C++ local staying reachable: a bare variable, a value whose Go type
// has no C/C++ part (int(v.size()), a string), and a conversion to a C number (C.int(n)).
func copiedArg(info *tinfo, a ast.Expr) bool {
	a = ast.Unparen(a)
	if _, ok := a.(*ast.Ident); ok {
		return true
	}
	if tv, ok := info.Types[a]; ok && tv.IsValue() && tv.Type != nil && complete(tv.Type) {
		return true
	}
	if c, ok := a.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "C" && cBuiltins[sel.Sel.Name] != "" {
				return true
			}
		}
	}
	return false
}

// A funcSpan is a function (a declaration or a literal) inside the function being lowered.
type funcSpan struct {
	pos, end token.Pos
	defers   bool         // has a defer statement of its own
	lit      bool         // a function literal (its frame ends when the literal returns)
	scope    *types.Scope // the scope of the parameters and the function's own block
}

// funcSpans lists the functions in top.
func funcSpans(info *tinfo, top ast.Node) []funcSpan {
	var fns []funcSpan
	ast.Inspect(top, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Body != nil {
				fns = append(fns, funcSpan{x.Pos(), x.End(), hasDefer(x.Body), false, info.Scopes[x.Type]})
			}
		case *ast.FuncLit:
			fns = append(fns, funcSpan{x.Pos(), x.End(), hasDefer(x.Body), true, info.Scopes[x.Type]})
		}
		return true
	})
	return fns
}

// rangeFuncBodies lists the bodies of the range-over-func statements in top: a body is a function
// of its own in C++ (a lambda the iterator calls), and what it declares ends when it returns.
func rangeFuncBodies(info *tinfo, top ast.Node) []*ast.BlockStmt {
	var out []*ast.BlockStmt
	ast.Inspect(top, func(n ast.Node) bool {
		if r, ok := n.(*ast.RangeStmt); ok {
			if _, isFunc := under(info.TypeOf(r.X)).(*types.Signature); isFunc {
				out = append(out, r.Body)
			}
		}
		return true
	})
	return out
}

// inBody reports whether v is declared inside one of the blocks.
func inBody(bodies []*ast.BlockStmt, v *types.Var) bool {
	for _, b := range bodies {
		if b.Pos() <= v.Pos() && v.Pos() < b.End() {
			return true
		}
	}
	return false
}

// declaringFunc returns the innermost function that declares v, or nil.
func declaringFunc(fns []funcSpan, v *types.Var) *funcSpan {
	var in *funcSpan
	for i := range fns {
		if f := &fns[i]; f.pos <= v.Pos() && v.Pos() < f.end && (in == nil || f.pos >= in.pos) {
			in = f
		}
	}
	return in
}

// addrHeld returns the C/C++ locals of the function top that the frame of their own function must
// hold: a local whose address is taken (&x, a raw C++ address that a pointer, a slice, a map or a
// struct can keep) in a function that has a defer statement of its own and declares it, and
// whose pointer can outlive its C++ block:
//   - the local is declared in the function's own block (not in an inner block or a loop): C++
//     destroys it before the deferred calls run, where Go keeps it until the function ends;
//   - or the address, or a variable that holds it (a slice it was appended to, a struct that
//     has it in a field, a map), is stored in a variable declared outside the local's block, in a
//     package variable or in a field of one: the pointer is visibly kept past the block.
//
// The frame holds the box (gp_hold) until after every deferred call ran. A pointer that is only
// passed to a call, used as the receiver of a call or stored in a variable of the same block
// dies with the block, as in C++ (so a lock guard whose address is passed to a function is
// destroyed at the end of its iteration or block, as without defers), and one that a C++ call
// keeps or gives back (lib.find(x)) follows C++'s lifetime rules.
func addrHeld(info *tinfo, top ast.Node) map[*types.Var]bool {
	return newAddrAnalysis(info, top).heldLocals()
}

// An addrAnalysis is the result of addrStores, addrHeld and addrEscapes for one function top, each
// made once on demand. The functions that a function declaration has inside it (its literals and the
// bodies of its range-over-func statements) are lowered one at a time with the same top, so they
// share one analysis: made again for each of them it is quadratic in their number. It belongs to
// the lowering of one function (funcInfo), which is also what keeps it from outliving the type
// information that an instance of a generic function replaces.
type addrAnalysis struct {
	info *tinfo
	top  ast.Node

	done      bool // taken, holds, leaving and outliving are made (addrStores)
	taken     []*types.Var
	holds     []*addrHold
	leaving   []*ast.UnaryExpr
	outliving []*ast.UnaryExpr

	heldDone bool
	held     map[*types.Var]bool

	escDone     bool
	escapes     map[*types.Var]bool
	escapeExprs map[*ast.UnaryExpr]bool
}

func newAddrAnalysis(info *tinfo, top ast.Node) *addrAnalysis {
	return &addrAnalysis{info: info, top: top}
}

// stores makes the result of addrStores, once.
func (a *addrAnalysis) stores() {
	if !a.done && a.top == nil {
		a.done = true // (a funcGen with no function: nothing to analyse)
	}
	if !a.done {
		a.taken, a.holds, a.leaving, a.outliving = addrStores(a.info, a.top)
		a.done = true
		addrStoresRuns.Add(1)
	}
}

// heldLocals is addrHeld, made once. The map is shared: no caller changes it.
func (a *addrAnalysis) heldLocals() map[*types.Var]bool {
	if a.heldDone {
		return a.held
	}
	a.heldDone = true
	a.held = a.makeHeld()
	return a.held
}

func (a *addrAnalysis) makeHeld() map[*types.Var]bool {
	info, top := a.info, a.top
	a.stores()
	taken, holds := a.taken, a.holds
	fns := funcSpans(info, top)
	if len(taken) == 0 {
		return nil
	}
	holdsOf := make(map[*types.Var][]*addrHold) // the holds that keep the address of a local
	for _, h := range holds {
		for _, v := range h.addrs {
			holdsOf[v] = append(holdsOf[v], h)
		}
	}
	held := make(map[*types.Var]bool)
	for _, v := range taken {
		in := declaringFunc(fns, v) // the innermost function that declares v
		if in == nil || !in.defers {
			continue
		}
		if v.Parent() == in.scope {
			held[v] = true // declared in the function's own block
			continue
		}
		sc := v.Parent()
	holders:
		for _, h := range holdsOf[v] {
			if h.leaves || sc == nil {
				held[v] = true
				break
			}
			for _, w := range h.members {
				if !(sc.Pos() <= w.Pos() && w.Pos() < sc.End()) {
					held[v] = true // kept in a variable that outlives v's block
					break holders
				}
			}
		}
	}
	return held
}

// An addrHold is a place that keeps the address of C/C++ locals (addrStores): the class of the
// variables that may refer to one object, with the address-taken locals it holds, or the place where
// an address leaves the function's own objects (leaves).
type addrHold struct {
	members []*types.Var // the variables of the class (none for leaves)
	addrs   []*types.Var // the address-taken locals whose address it keeps
	leaves  bool
}

// addrStores follows the addresses of C/C++ locals inside the function top (addrHeld and
// addrEscapes): taken lists the locals whose address is taken (&x, &x.f, &x[i]), holds the places
// that keep an address (see addrHold), and leaving the & expressions (of C/C++ locals and of C/C++
// members of gocpp objects) that leave the function's own objects. An address leaves when it is
// stored in a package variable, a member of a C/C++ package, a result, a return statement, a
// channel send, a go statement (its arguments and the variables its closure captures), a copy or an
// append into such a place, or through a pointer, slice or map that is not provably an object of
// this function (a parameter, a range variable, a value of a call, a map lookup, an assertion, a
// receive, a package variable's copy of a pointer, a pointer whose own address is taken ...): that
// pointer may point into package data or into memory that the caller keeps, so the address must
// live as long as it may be kept. outliving lists the & expressions that a variable of the class is
// holding while it is declared in a block that encloses the block of the local (list =
// append(list, &x) in a loop): the pointer outlives the local's block, and the caller decides
// whether the frame already holds the local (addrEscapes). Over-boxing is allowed, under-boxing is
// not.
//
// It works in two steps. (1) Variables that may refer to the same object are one class (a union
// of the variables joined by a := b, a = b, p := &h, p := s[:], append(s, ...)): a store through
// one is a store through all. A class is local when every member is a pointer, slice or map
// declared in the function and set only from &local, new, make, a composite literal, nil,
// append of such a value, or another member (a local struct value or array is its own object), and
// no member has its own address taken. (2) The addresses flow to a fixpoint: a store adds the
// addresses in the stored value, and the addresses held by the variables the value reads, to the
// class of the target (or to nil, where it leaves). A store whose value cannot hold a pointer (an
// int, a string) carries no address. The work is one pass over the function for the classes and a
// worklist over the classes for the addresses, whose sets are the address-taken locals; the result
// is made once per class, not once per variable.
func addrStores(info *tinfo, top ast.Node) (taken []*types.Var, holds []*addrHold, leaving, outliving []*ast.UnaryExpr) {
	b := &boxer{info: info}
	// the locals whose address is taken, and the & expressions that matter: those of C/C++ locals
	// and of C/C++ members of gocpp objects (ownedMemberAddr)
	tracked := make(map[*ast.UnaryExpr]bool)
	seen := make(map[*types.Var]bool)
	ast.Inspect(top, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND {
			if cAddrOf(info, u) {
				tracked[u] = true
				if v := b.localVar(rootIdentOf(u.X)); v != nil && !seen[v] {
					seen[v] = true
					taken = append(taken, v)
				}
			} else if goOwner(info, u.X) != nil {
				tracked[u] = true
			}
		}
		return true
	})
	if len(tracked) == 0 {
		return nil, nil, nil, nil
	}
	a := &addrFlow{info: info, b: b, tracked: tracked, parent: make(map[*types.Var]*types.Var)}
	a.collect(top)
	a.unite()
	a.flow()
	varsOf := func(items map[*ast.UnaryExpr]bool) []*types.Var {
		var out []*types.Var
		have := make(map[*types.Var]bool)
		for u := range items {
			if cAddrOf(info, u) {
				if v := b.localVar(rootIdentOf(u.X)); v != nil && !have[v] {
					have[v] = true
					out = append(out, v)
				}
			}
		}
		slices.SortFunc(out, func(x, y *types.Var) int { return int(x.Pos() - y.Pos()) })
		return out
	}
	// one hold per class that keeps an address, made once for the class
	members := make(map[*types.Var][]*types.Var)
	for w := range a.parent {
		r := a.find(w)
		members[r] = append(members[r], w)
	}
	for r, ms := range members {
		if len(a.items[r]) == 0 {
			continue
		}
		if vs := varsOf(a.items[r]); len(vs) > 0 {
			holds = append(holds, &addrHold{members: ms, addrs: vs})
		}
	}
	if vs := varsOf(a.items[nil]); len(vs) > 0 {
		holds = append(holds, &addrHold{addrs: vs, leaves: true})
	}
	for u := range a.items[nil] {
		leaving = append(leaving, u)
	}
	outliving = a.outliving()
	return taken, holds, leaving, outliving
}

// An addrFlow is the state of addrStores: the records of the stores in the function, the classes
// of the variables that may refer to the same object, and the & expressions that each class holds.
type addrFlow struct {
	info    *tinfo
	b       *boxer
	tracked map[*ast.UnaryExpr]bool
	recs    []flowRec
	// the classes (union-find over the variables that matter)
	parent   map[*types.Var]*types.Var
	foreign  map[*types.Var]bool // a class that may point into memory of the caller or of a package
	declared map[*types.Var]bool // declared in the function by := or var (not a parameter, a range variable)
	results  map[*types.Var]bool // the named results
	taken    map[*types.Var]bool // a pointer, slice or map variable whose own address is taken
	items    map[*types.Var]map[*ast.UnaryExpr]bool
}

// A flowRec is one store: the value rhs goes to the place lhs (nil: it leaves), or, for a range
// variable, a call parameter and the like, to the variable that lhs is. rangeVar: lhs is a range
// variable. The own locals of a function literal in rhs are not read (flow).
type flowRec struct {
	lhs      ast.Expr
	rhs      ast.Expr
	rangeVar bool
	isGo     bool // a go statement: it reads its arguments whatever the call returns
}

func (a *addrFlow) find(v *types.Var) *types.Var {
	p, ok := a.parent[v]
	if !ok {
		a.parent[v] = v
		return v
	}
	if p == v {
		return v
	}
	r := a.find(p)
	a.parent[v] = r
	return r
}

func (a *addrFlow) union(x, y *types.Var) {
	rx, ry := a.find(x), a.find(y)
	if rx != ry {
		a.parent[rx] = ry
	}
}

// mayHoldPointers reports whether a value of type t can refer to memory (a pointer, a slice, a map,
// a channel, a function, an interface, or a struct or array of those). A C/C++ type that gocpp does
// not know counts.
func mayHoldPointers(t types.Type, depth int) bool {
	if t == nil || depth > 8 {
		return true
	}
	switch u := under(t).(type) {
	case *types.Basic:
		return u.Kind() == types.UnsafePointer || u.Kind() == types.Invalid
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if mayHoldPointers(u.Field(i).Type(), depth+1) {
				return true
			}
		}
		return false
	case *types.Array:
		return mayHoldPointers(u.Elem(), depth+1)
	}
	return true
}

// refType reports whether t is a pointer, a slice or a map (or has no type known to gocpp): a variable of
// it can refer to an object that the function did not make.
func refType(t types.Type) bool {
	switch u := under(t).(type) {
	case *types.Pointer, *types.Slice, *types.Map:
		return true
	case *types.Basic:
		return u.Kind() == types.Invalid // a C/C++ type that gocpp does not know: it may be a pointer
	case nil:
		return true
	}
	return false
}

// collect reads the records of the stores in top.
func (a *addrFlow) collect(top ast.Node) {
	info := a.info
	a.declared = make(map[*types.Var]bool)
	a.results = make(map[*types.Var]bool)
	a.taken = make(map[*types.Var]bool)
	a.foreign = make(map[*types.Var]bool)
	add := func(r flowRec) { a.recs = append(a.recs, r) }
	index := func(x ast.Expr) ast.Expr { return &ast.IndexExpr{X: x} } // x[i]: a store into an element
	ast.Inspect(top, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncType:
			if x.Results != nil {
				for _, f := range x.Results.List {
					for _, id := range f.Names {
						if v, ok := info.Defs[id].(*types.Var); ok {
							a.results[v] = true
						}
					}
				}
			}
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				if id, ok := ast.Unparen(l).(*ast.Ident); ok && x.Tok == token.DEFINE {
					if v, ok := info.Defs[id].(*types.Var); ok && v.Name() != "_" {
						a.declared[v] = true
					}
				}
				switch {
				case len(x.Lhs) == len(x.Rhs):
					add(flowRec{lhs: l, rhs: x.Rhs[i]})
				case len(x.Rhs) == 1:
					add(flowRec{lhs: l, rhs: x.Rhs[0]})
				}
			}
		case *ast.ValueSpec:
			for i, id := range x.Names {
				if v, ok := info.Defs[id].(*types.Var); ok && v.Name() != "_" {
					a.declared[v] = true
				}
				switch {
				case len(x.Names) == len(x.Values):
					add(flowRec{lhs: id, rhs: x.Values[i]})
				case len(x.Values) == 1:
					add(flowRec{lhs: id, rhs: x.Values[0]})
				}
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if e != nil {
					add(flowRec{lhs: e, rhs: x.X, rangeVar: true})
				}
			}
		case *ast.ReturnStmt:
			for _, r := range x.Results {
				add(flowRec{rhs: r}) // what a function returns outlives it
			}
		case *ast.SendStmt:
			add(flowRec{rhs: x.Value}) // the receiver may be anywhere, for as long as it likes
		case *ast.GoStmt:
			// everything the new goroutine reads leaves: the arguments and what the closure captures
			// (not the locals that the closure declares itself)
			add(flowRec{rhs: x.Call, isGo: true})
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				if id, ok := ast.Unparen(x.X).(*ast.Ident); ok {
					if v := a.b.localVar(id); v != nil && refType(v.Type()) {
						a.taken[v] = true // the callee can store any pointer in it
					}
				}
			}
		case *ast.CallExpr:
			fun := ast.Unparen(x.Fun)
			if sel, ok := fun.(*ast.SelectorExpr); ok && a.b.pointerMethod(sel) {
				if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok {
					if v := a.b.localVar(id); v != nil && refType(v.Type()) {
						a.taken[v] = true // (&p).m()
					}
				}
			}
			if lit, ok := fun.(*ast.FuncLit); ok {
				// an immediately called function literal: the arguments are its parameters
				i := 0
				for _, f := range lit.Type.Params.List {
					for _, id := range f.Names {
						if i < len(x.Args) {
							add(flowRec{lhs: id, rhs: x.Args[i]})
						}
						i++
					}
					if len(f.Names) == 0 {
						i++
					}
				}
			}
			if id, ok := fun.(*ast.Ident); ok {
				if _, ok := info.Uses[id].(*types.Builtin); ok {
					switch id.Name {
					case "copy":
						if len(x.Args) == 2 {
							add(flowRec{lhs: index(x.Args[0]), rhs: x.Args[1]})
						}
					case "append":
						// the elements are stored into the backing array of the first argument, which
						// may be package data: append(gptrs, &x)
						for _, arg := range x.Args[min(1, len(x.Args)):] {
							add(flowRec{lhs: index(x.Args[0]), rhs: arg})
						}
					}
				}
			}
		}
		return true
	})
}

// aliasOf reports whether the value e is a pointer, slice or map that the function itself makes or
// that refers to a variable's own storage, and the variables whose objects it may be: nil, a
// composite literal, new, make, &literal, &v, an append to such a value, a slice of an array
// variable or of such a value, another local variable. Anything else (a package variable, a call, a
// lookup, an assertion, a receive, a selector or element of a pointer: p.next, s[0]) is not.
func (a *addrFlow) aliasOf(e ast.Expr) (bool, []*types.Var) {
	info := a.info
	switch x := ast.Unparen(e).(type) {
	case *ast.CompositeLit:
		return true, nil
	case *ast.Ident:
		if _, ok := info.Uses[x].(*types.Nil); ok {
			return true, nil
		}
		if v := a.b.localVar(x); v != nil {
			return true, []*types.Var{v}
		}
	case *ast.UnaryExpr:
		if x.Op != token.AND {
			return false, nil
		}
		if _, lit := ast.Unparen(x.X).(*ast.CompositeLit); lit {
			return true, nil
		}
		if v := valuePathRoot(info, x.X); v != nil {
			return true, []*types.Var{v}
		}
	case *ast.CallExpr:
		id, ok := ast.Unparen(x.Fun).(*ast.Ident)
		if !ok {
			return false, nil
		}
		if _, ok := info.Uses[id].(*types.Builtin); !ok {
			return false, nil
		}
		switch id.Name {
		case "new", "make":
			return true, nil
		case "append":
			if len(x.Args) > 0 {
				return a.aliasOf(x.Args[0])
			}
		}
	case *ast.SliceExpr:
		if _, isArray := under(info.TypeOf(x.X)).(*types.Array); isArray {
			if v := valuePathRoot(info, x.X); v != nil {
				return true, []*types.Var{v}
			}
			return false, nil
		}
		return a.aliasOf(x.X)
	}
	return false, nil
}

// unite makes the classes of the variables that may refer to the same object, and marks the
// classes that may point into memory that the function did not make.
func (a *addrFlow) unite() {
	for _, r := range a.recs {
		if r.lhs == nil {
			continue
		}
		id, ok := ast.Unparen(r.lhs).(*ast.Ident)
		if !ok {
			continue
		}
		w := a.b.localVar(id)
		if w == nil || !mayHoldPointers(w.Type(), 0) {
			continue
		}
		a.find(w)
		if r.rangeVar {
			a.foreign[w] = true
			continue
		}
		ok, roots := a.aliasOf(r.rhs)
		if !ok {
			a.foreign[w] = true
			continue
		}
		for _, v := range roots {
			if mayHoldPointers(v.Type(), 0) {
				a.union(w, v)
			}
		}
	}
	// a reference variable that the function did not declare (a parameter, a closure's parameter,
	// a named result), or whose own address is taken, may refer to anything
	for v := range a.parent {
		if refType(v.Type()) && (!a.declared[v] || a.taken[v]) {
			a.foreign[v] = true
		}
	}
	for v := range a.taken {
		if _, ok := a.parent[v]; !ok {
			a.find(v)
			a.foreign[v] = true
		}
	}
	// the class is foreign when a member is
	roots := make(map[*types.Var]bool)
	for v := range a.parent {
		if a.foreign[v] {
			roots[a.find(v)] = true
		}
	}
	a.foreign = make(map[*types.Var]bool, len(roots))
	for r := range roots {
		a.foreign[r] = true
	}
}

// target resolves where the record stores its value: the class root of the variable that the
// target lhs is a part of, or nil when the value leaves the function's own objects.
func (a *addrFlow) target(lhs ast.Expr) (dst *types.Var, skip bool) {
	info := a.info
	e := ast.Unparen(lhs)
	if id, ok := e.(*ast.Ident); ok && id.Name == "_" {
		return nil, true // nothing keeps it
	}
	indirect := 0      // the steps of the target that go through a pointer, a slice or a map
	innermost := false // the step that is applied to the variable itself is one of them
	for {
		var through bool
		switch y := e.(type) {
		case *ast.SelectorExpr:
			sel := info.Selections[y]
			through = sel != nil && sel.Indirect() || reachesThroughPointer(info.TypeOf(y.X))
			e = ast.Unparen(y.X)
		case *ast.IndexExpr:
			through = reachesThroughPointer(info.TypeOf(y.X))
			e = ast.Unparen(y.X)
		case *ast.StarExpr:
			through = true
			e = ast.Unparen(y.X)
		case *ast.Ident:
			if info.Uses[y] == nil && info.Defs[y] == nil {
				return nil, true // defines no variable of its own: the binding of a type switch
			}
			base := a.b.localVar(y)
			if base == nil || a.results[base] {
				return nil, false // a package variable, or a result: it leaves
			}
			root := a.find(base)
			if a.foreign[root] || refType(base.Type()) && (!a.declared[base] || a.taken[base]) {
				// (a variable that is only used here is no member of a class: judge it by itself)
				a.foreign[root] = true
			}
			if indirect > 0 && !(indirect == 1 && innermost && !a.foreign[root]) {
				return nil, false // the address leaves the function's own objects
			}
			return root, false
		default:
			return nil, false
		}
		if through {
			indirect++
		}
		innermost = through
	}
}

// outliving lists the & expressions that a class holds while one of its variables is declared in a
// block that encloses the block of the local (list = append(list, &x) in a loop, a pointer variable
// declared before the loop): the pointer outlives the local.
func (a *addrFlow) outliving() []*ast.UnaryExpr {
	scopes := make(map[*types.Var]map[*types.Scope]bool) // class root -> scopes of its members
	for w := range a.parent {
		if sc := w.Parent(); sc != nil {
			r := a.find(w)
			if scopes[r] == nil {
				scopes[r] = make(map[*types.Scope]bool)
			}
			scopes[r][sc] = true
		}
	}
	var out []*ast.UnaryExpr
	seen := make(map[*ast.UnaryExpr]bool) // (many classes can hold the same expression)
	for r, items := range a.items {
		if r == nil || len(scopes[r]) == 0 {
			continue
		}
		for u := range items {
			if seen[u] || !a.tracked[u] || !cAddrOf(a.info, u) || a.items[nil][u] {
				continue
			}
			v := a.b.localVar(rootIdentOf(u.X))
			if v == nil || v.Parent() == nil {
				continue
			}
			for sc := v.Parent().Parent(); sc != nil; sc = sc.Parent() {
				if scopes[r][sc] {
					seen[u] = true
					out = append(out, u)
					outlivingListed.Add(1)
					break
				}
			}
		}
	}
	return out
}

// flow resolves the records and runs the addresses to a fixpoint with a worklist.
func (a *addrFlow) flow() {
	a.items = make(map[*types.Var]map[*ast.UnaryExpr]bool)
	type edgeSet = map[*types.Var]bool
	edges := make(map[*types.Var]edgeSet) // class (nil: leaves) -> classes that receive what it holds
	var nilEdges edgeSet
	hold := func(c *types.Var) map[*ast.UnaryExpr]bool {
		m := a.items[c]
		if m == nil {
			m = make(map[*ast.UnaryExpr]bool)
			a.items[c] = m
		}
		return m
	}
	var work []*types.Var
	queued := make(map[*types.Var]bool)
	push := func(c *types.Var) {
		if !queued[c] {
			queued[c] = true
			work = append(work, c)
		}
	}
	for i := range a.recs {
		r := &a.recs[i]
		if t := a.info.TypeOf(r.rhs); !r.isGo && t != nil && !mayHoldPointers(t, 0) {
			continue // an int, a string: the value carries no address
		}
		var dst *types.Var
		if r.lhs != nil {
			var skip bool
			dst, skip = a.target(r.lhs)
			if skip {
				continue
			}
		}
		// what the value holds: the tracked & expressions in it, and the variables it reads
		var reads []*types.Var
		written := make(map[*ast.Ident]bool) // the variables that a function literal in the value sets
		// The locals of a function literal in the value are objects of the literal: what they hold
		// leaves only by a store or a return in the literal, which have records of their own. Reading
		// them here would make every literal hold what all the others stored through one class.
		var lits [][2]token.Pos
		inLit := func(pos token.Pos) bool {
			for _, l := range lits {
				if l[0] <= pos && pos < l[1] {
					return true
				}
			}
			return false
		}
		ast.Inspect(r.rhs, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				lits = append(lits, [2]token.Pos{x.Pos(), x.End()})
			case *ast.AssignStmt:
				// (cur = &x in a literal sets cur, it does not read it: reading would make every
				// literal that sets cur hold what all the others stored: quadratic)
				if x.Tok == token.ASSIGN || x.Tok == token.DEFINE {
					for _, l := range x.Lhs {
						if id, ok := ast.Unparen(l).(*ast.Ident); ok {
							written[id] = true
						}
					}
				}
			case *ast.ValueSpec:
				for _, id := range x.Names {
					written[id] = true
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND && a.tracked[x] {
					if v := a.b.localVar(rootIdentOf(x.X)); v != nil && inLit(v.Pos()) {
						return true
					}
					hold(dst)[x] = true
				}
			case *ast.Ident:
				if written[x] {
					return true
				}
				if w := a.b.localVar(x); w != nil {
					if inLit(w.Pos()) {
						return true
					}
					reads = append(reads, a.find(w))
				}
			}
			return true
		})
		for _, s := range reads {
			if dst == nil {
				if nilEdges == nil {
					nilEdges = make(edgeSet)
				}
				nilEdges[s] = true
				continue
			}
			if s == dst {
				continue
			}
			if edges[s] == nil {
				edges[s] = make(edgeSet)
			}
			edges[s][dst] = true
		}
	}
	// the classes that are read into nil: everything they hold leaves
	for s := range nilEdges {
		if edges[s] == nil {
			edges[s] = make(edgeSet)
		}
		edges[s][nil] = true
	}
	for s := range a.items {
		push(s)
	}
	for len(work) > 0 {
		n := work[len(work)-1]
		work = work[:len(work)-1]
		queued[n] = false
		for m := range edges[n] {
			grew := false
			dst := hold(m)
			for u := range a.items[n] {
				if !dst[u] {
					dst[u] = true
					grew = true
				}
			}
			if grew {
				push(m)
			}
		}
	}
}

// addrEscapes returns the C/C++ locals of the function top whose address leaves the function's
// own objects (addrStores, key nil): stored in a package variable, in a member of a C/C++
// package, in a result, returned, or stored through a pointer, slice or map that is not provably
// an object of this function. Go keeps such a variable alive as long as a pointer to it is, so the
// local is boxed and its &x is a gp::cptr that owns the box (ownedLocalAddr), not a raw C++ address
// that dangles when the block ends. It also returns the & expressions that were stored so (for
// ownedMemberAddr).
func addrEscapes(info *tinfo, top ast.Node) (map[*types.Var]bool, map[*ast.UnaryExpr]bool) {
	return newAddrAnalysis(info, top).escaping()
}

// escaping is addrEscapes, made once. The maps are shared: no caller changes them.
func (a *addrAnalysis) escaping() (map[*types.Var]bool, map[*ast.UnaryExpr]bool) {
	if !a.escDone {
		a.escDone = true
		a.escapes, a.escapeExprs = a.makeEscaping()
	}
	return a.escapes, a.escapeExprs
}

func (a *addrAnalysis) makeEscaping() (map[*types.Var]bool, map[*ast.UnaryExpr]bool) {
	info, top := a.info, a.top
	a.stores()
	holds, outliving := a.holds, a.outliving
	leaving := slices.Clone(a.leaving)
	var leaves *addrHold
	for _, h := range holds {
		if h.leaves {
			leaves = h
		}
	}
	// An address kept by a variable of an enclosing block leaves, unless the frame holds the local
	// already: in a function declaration with a defer statement of its own, addrHeld keeps it to the
	// end of that function, which is where the holder dies too. Not in a function literal (the holder
	// may be declared outside it and outlive the call) and not in the body of a range-over-func
	// statement (the body is a lambda that ends before the function does, and the frame does not
	// hold what it declares).
	var addrs []*types.Var // the locals of the leaving addresses
	haveAddr := make(map[*types.Var]bool)
	if leaves != nil {
		addrs = leaves.addrs
		for _, v := range addrs {
			haveAddr[v] = true
		}
	}
	if len(outliving) > 0 {
		fns := funcSpans(info, top)
		bodies := rangeFuncBodies(info, top)
		for _, u := range outliving {
			v := (&boxer{info: info}).localVar(rootIdentOf(u.X))
			if v == nil {
				continue
			}
			if in := declaringFunc(fns, v); in != nil && in.defers && !in.lit && !inBody(bodies, v) {
				continue
			}
			leaving = append(leaving, u)
			if !haveAddr[v] {
				haveAddr[v] = true
				addrs = append(addrs, v)
			}
		}
	}
	var outExprs map[*ast.UnaryExpr]bool
	for _, u := range leaving {
		if outExprs == nil {
			outExprs = make(map[*ast.UnaryExpr]bool)
		}
		outExprs[u] = true
	}
	if len(addrs) == 0 {
		return nil, outExprs
	}
	// The address must be of the variable itself or of a part stored inside it (&x, &x.f,
	// &x.arr[i], &x.in.a): the address of what a raw pointer variable points to is the pointee's.
	inside := make(map[*types.Var]bool)
	ast.Inspect(top, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND && cAddrOf(info, u) {
			if v := valuePathRoot(info, u.X); v != nil {
				inside[v] = true
			}
		}
		return true
	})
	var out map[*types.Var]bool
	for _, v := range addrs {
		if !inside[v] {
			continue
		}
		if out == nil {
			out = make(map[*types.Var]bool)
		}
		out[v] = true
	}
	return out, outExprs
}

// escapeInfo computes the results of addrEscapes for the function being lowered, once.
func (fg *funcGen) escapeInfo() {
	if (fg.escapesDone && fg.escTop == fg.top) || fg.top == nil {
		return
	}
	fg.escTop = fg.top // (gp_var_init has one funcGen for all the package initializers: decl.go)
	fg.escapes, fg.escapeExprs = fg.addrAnalysis().escaping()
	fg.escapesDone = true
}

// outlivingListed counts the addresses that the outliving pass lists (TestCAddrManyHolders: each address
// once for the function, not once for each class that holds it).
var outlivingListed atomic.Int64

// addrStoresRuns counts the analyses of a function (TestCAddrManyBodies: one for each function declaration,
// shared with the literals and the range-over-func bodies in it).
var addrStoresRuns atomic.Int64

// addrShareOff makes every funcGen analyse its function on its own, as before the analysis was shared;
// TestCAddrManyBodies compares the output of the two.
var addrShareOff bool

// addrAnalysis returns the analysis of the function top that this funcGen shares with the literals and
// range-over-func bodies inside the same function declaration (addrAnalysis).
func (fg *funcGen) addrAnalysis() *addrAnalysis {
	if addrShareOff {
		return newAddrAnalysis(fg.g.info, fg.top) // (a test compares the two)
	}
	// gp_var_init has one funcGen (no funcInfo) for all the package initializers and changes its top
	// for each of them: an analysis belongs to one top, and a funcGen asked about another makes a new
	// one. The literals inside an initializer share the analysis of that initializer.
	if fg.fn == nil {
		if fg.addrOwn == nil || fg.addrOwn.top != fg.top {
			fg.addrOwn = newAddrAnalysis(fg.g.info, fg.top)
		}
		return fg.addrOwn
	}
	if fg.fn.addr == nil {
		fg.fn.addr = newAddrAnalysis(fg.g.info, fg.top)
	}
	return fg.fn.addr
}

// escapedAddr reports whether e is &x of a C/C++ local, or of a part inside it, whose address
// leaves the function's own objects (addrEscapes): the variable is boxed (boxCEscape).
func (fg *funcGen) escapedAddr(e *ast.UnaryExpr) bool {
	fg.escapeInfo()
	if len(fg.escapes) == 0 {
		return false
	}
	v := valuePathRoot(fg.g.info, e.X)
	return v != nil && fg.escapes[v] && fg.isCBox(v)
}

// escapedMember returns the gocpp pointer that owns the object holding the C/C++ member whose
// address e takes, when the address is stored where it may outlive the call (addrEscapes): &w.in.b
// with w a pointer to a gocpp struct that has a C struct field `in` (a method's receiver, a
// parameter, a local). Without it the address is a raw pointer into the object, which dangles when
// the last gocpp pointer to the object is dropped. nil: not that shape.
func (fg *funcGen) escapedMember(e *ast.UnaryExpr) ast.Expr {
	fg.escapeInfo()
	if !fg.escapeExprs[e] {
		return nil
	}
	return goOwner(fg.g.info, e.X)
}

// goOwner returns the expression of the gocpp pointer that owns the object a C/C++ member path x
// is in: w for w.in.b, w.p for w.p.in.b (w, w.p pointers to complete gocpp types), or nil when the
// path has no C/C++ member step or starts at a raw C pointer or a local value.
func goOwner(info *tinfo, x ast.Expr) ast.Expr {
	sawC := false
	for {
		sel, ok := ast.Unparen(x).(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		s := info.Selections[sel]
		if s == nil {
			sawC = true
		}
		t := info.TypeOf(sel.X)
		if s != nil && s.Indirect() {
			if _, isPtr := under(t).(*types.Pointer); !isPtr {
				return nil // (an embedded pointer on the way: not followed)
			}
		}
		if t == nil {
			x = sel.X
			continue
		}
		if p, ok := under(t).(*types.Pointer); ok {
			if sawC && complete(p.Elem()) {
				return sel.X
			}
			return nil
		}
		x = sel.X
	}
}

// cAddrOf reports whether e is the address of a C/C++ local or of a part inside it that
// addrStores follows: cAddr's, and the address of a C/C++ member of a gocpp struct value
// (&o.in.a with o a Go struct that holds a C struct: the member has no types.Selection, so the
// boxer's root does not see it, and cAddr does not take it for C's because the type of o is a
// complete Go type).
func cAddrOf(info *tinfo, e *ast.UnaryExpr) bool {
	if cAddr(info, e) {
		return true
	}
	x := ast.Unparen(e.X)
	if _, bare := x.(*ast.Ident); bare {
		return false
	}
	b := &boxer{info: info}
	return b.root(x) == nil && valuePathRoot(info, x) != nil
}

// valuePathRoot returns the local variable that the operand x of & is, or a part stored inside
// (x.f, x.in.a, x[i] of an array or a C++ container held by value), or nil: a step through a
// pointer, a slice or a map (p.f with p a pointer, s[i]) leaves the variable's own storage. (A C/C++
// member has no types.Selection in the opaque and the typed mode, so the steps are read from the
// types of the operands.)
func valuePathRoot(info *tinfo, x ast.Expr) *types.Var {
	b := &boxer{info: info}
	for {
		switch y := ast.Unparen(x).(type) {
		case *ast.Ident:
			return b.localVar(y)
		case *ast.SelectorExpr:
			if sel := info.Selections[y]; sel != nil && sel.Indirect() {
				return nil
			}
			if reachesThroughPointer(info.TypeOf(y.X)) {
				return nil
			}
			x = y.X
		case *ast.IndexExpr:
			if reachesThroughPointer(info.TypeOf(y.X)) {
				return nil
			}
			x = y.X
		default:
			return nil
		}
	}
}

// reachesThroughPointer reports whether a member or element of a value of type t is outside the
// value: t is a pointer, a slice or a map.
func reachesThroughPointer(t types.Type) bool {
	if t == nil {
		return false
	}
	switch under(t).(type) {
	case *types.Pointer, *types.Slice, *types.Map:
		return true
	}
	return false
}

// ownedLocalAddr returns the C++ of &x for the C/C++ local x (or a part of it) whose address is
// stored in a package variable (escapedAddr): a gp::cptr that owns the box of x, as Go's pointer
// keeps the variable alive, where the raw address &x would dangle when the block ends. The address
// of a part is an aliasing pointer that shares the box: gp::cptr<T>(gp::ptr<T>(x, &(*x).f)).
func (fg *funcGen) ownedLocalAddr(e *ast.UnaryExpr) cppir.Expr {
	operand := ast.Unparen(e.X)
	_, bare := operand.(*ast.Ident)
	var elem cppir.Type
	if bare {
		elem = fg.typeOfExpr(operand)
	}
	if elem == nil || isAutoType(elem) {
		// A member of a C++ object, or a variable declared from a C++ value (t := lib.T{}): the
		// C++ compiler names the type.
		elem = typeName("std::remove_cvref_t<decltype(" + cppir.ExprString(fg.expr(e.X)) + ")>")
	}
	if bare {
		return construct(cptrType(elem), fg.addr(e))
	}
	root := valuePathRoot(fg.g.info, operand)
	box := cxxName(fg.cname(root, rootIdentOf(operand).Pos()))
	part := construct(&cppir.TypeName{Name: "gp::ptr", Args: []cppir.Node{elem}}, box, &cppir.Unary{Op: "&", X: fg.expr(operand)})
	return construct(cptrType(elem), part)
}

// ownedMemberAddr returns the C++ of &w.in.b for escapedMember's owner: the aliasing gp::cptr<T>(
// gp::ptr<T>(w, &member)), which keeps the object that holds the member alive.
func (fg *funcGen) ownedMemberAddr(e *ast.UnaryExpr, owner ast.Expr) cppir.Expr {
	elem := typeName("std::remove_cvref_t<decltype(" + cppir.ExprString(fg.expr(e.X)) + ")>")
	part := construct(&cppir.TypeName{Name: "gp::ptr", Args: []cppir.Node{elem}}, fg.expr(owner), &cppir.Unary{Op: "&", X: fg.expr(e.X)})
	return construct(cptrType(elem), part)
}

// A deferObject says how a deferred call gets the C/C++ object it is made on (the object of a
// member call, defer v.clear(), or the callee that is a C++ callable, defer cb()). Go evaluates
// it when the defer statement runs; the call runs later, after the C++ blocks of the locals ended.
type deferObject int

const (
	// objStatic is an object that lives as long as the program (a C++ global, a package
	// variable, a header's name): the deferred call names it.
	objStatic deferObject = iota
	// objBoxed is a path that starts at a local variable (v, h.v, p.next): the local is
	// boxed, the frame holds its box, and the path is read when the call runs. (Go reads a
	// pointer in the path now: the difference shows only when the local is assigned before
	// the function ends; docs/spec/interop.md says so.)
	objBoxed
	// objPtr is a path that starts at a local gocpp pointer (s.c, s.mu): the frame holds a copy of
	// the pointer, as Go evaluates it now, and the path is read through it when the call runs.
	objPtr
	// objValue is a temporary (a call, a literal), or a local callable: evaluated now and
	// moved (copied for a variable) into the frame, as Go stores the func value.
	objValue
	// objBad is any other place that holds a local (v[i], *p): the call would read it late.
	objBad
)

// deferObjectOf classifies x, the object of a deferred call, and returns the local variable that
// a boxed path starts at.
func (g *gen) deferObjectOf(x ast.Expr) (deferObject, *types.Var) {
	switch y := ast.Unparen(x).(type) {
	case *ast.CallExpr, *ast.CompositeLit, *ast.FuncLit:
		return objValue, nil
	case *ast.Ident:
		if v, ok := g.info.Uses[y].(*types.Var); ok && !isPkgVar(v) && !v.IsField() {
			if !cObject(v.Type()) {
				if p, ok := v.Type().Underlying().(*types.Pointer); ok && complete(p.Elem()) {
					return objPtr, v
				}
				return objBad, nil // a gocpp value: Go reads the place now
			}
			return objBoxed, v
		}
		return objStatic, nil
	case *ast.SelectorExpr:
		if _, ok := g.cRefOf(y); ok {
			return objStatic, nil // a C/C++ name: lib.global
		}
		if g.isC(y.X) {
			return objStatic, nil
		}
		return g.deferObjectOf(y.X)
	}
	// Anything else: fine if it holds no local (lib.table[0], (*lib.p)), else the call would
	// read the local when it runs.
	bad := false
	ast.Inspect(x, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := g.info.Uses[id].(*types.Var); ok && !isPkgVar(v) && !v.IsField() {
				bad = true
			}
		}
		return !bad
	})
	if bad {
		return objBad, nil
	}
	return objStatic, nil
}

// deferRecvOK reports whether the deferred call c can keep the object it is made on (cbox.go), and
// reports it if it can't: a receiver that is an element or a pointer's target of a local
// (defer v[i].m(), defer (*p).m()) is a place that Go evaluates at the defer statement and C++ would
// read when the call runs.
func (g *gen) deferRecvOK(c *ast.CallExpr, stmt string) bool {
	if !g.opaqueCallee(c) {
		return true
	}
	obj := c.Fun
	what := "callee"
	if sel := g.opaqueMember(c.Fun); sel != nil {
		obj, what = sel.X, "object"
	}
	if k, _ := g.deferObjectOf(obj); k == objBad {
		if stmt == "go" {
			g.userError(obj.Pos(), "cannot start a goroutine with a call whose %s is %s: Go evaluates it when the go statement runs, and a C++ call in a new goroutine can only keep a variable's object or a temporary: use a func literal, go func() { ... }()", what, g.srcText(obj))
		} else {
			g.userError(obj.Pos(), "cannot defer a call whose %s is %s: Go evaluates it when the defer statement runs, and a deferred C++ call can only keep a variable's object or a temporary: use a func literal, defer func() { ... }()", what, g.srcText(obj))
		}
		return false
	}
	return true
}

// storeObject makes the thunk of the deferred call c (deferThunk) keep its object: for a member of
// a C/C++ value, the box of the local that the object path starts at (the frame holds a copy of the
// box, and the path is read through it when the call runs), or the object itself if it is a
// temporary (gp::cgo::keep: moved into the frame, or a pointer if C++ gave a reference); for a
// C++ callable, its value (a copy, as Go stores the func value). It adds the thunk's parameters and
// the stored values, and returns the function that forgets what it set up.
func (fg *funcGen) storeObject(c *ast.CallExpr, params *[]cppir.Param, values *[]cppir.Expr) func() {
	g := fg.g
	if !g.opaqueCallee(c) {
		return func() {}
	}
	param := func() string {
		name := "gp_a" + strconv.Itoa(len(*params))
		*params = append(*params, cppir.Param{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Name: name})
		return name
	}
	sel := g.opaqueMember(c.Fun)
	if sel == nil {
		// A C++ callable: Go evaluates the func value now.
		kind, _ := g.deferObjectOf(c.Fun)
		if kind == objStatic {
			return func() {}
		}
		*values = append(*values, fg.deferArg(c.Fun))
		name := param()
		old, had := fg.temps[c.Fun]
		fg.temps[c.Fun] = name
		return func() {
			if had {
				fg.temps[c.Fun] = old
			} else {
				delete(fg.temps, c.Fun)
			}
		}
	}
	kind, root := g.deferObjectOf(sel.X)
	switch kind {
	case objBoxed:
		if !fg.isBoxed(root) {
			g.errorf(sel.Pos(), "internal error: the object of a deferred call, %s, is not boxed", root.Name())
			return func() {}
		}
		*values = append(*values, cxxName(fg.cname(root, sel.Pos())))
		name := param()
		if fg.heldBoxes == nil {
			fg.heldBoxes = make(map[*types.Var]string)
		}
		fg.heldBoxes[root] = name
		return func() { delete(fg.heldBoxes, root) }
	case objPtr:
		var ptr cppir.Expr = cxxName(fg.cname(root, sel.Pos()))
		if fg.isBoxed(root) {
			ptr = &cppir.Paren{X: deref(ptr)}
		}
		*values = append(*values, call("gp::cgo::nonnil", ptr)) // Go panics here for a nil pointer
		name := param()
		if fg.heldPtrs == nil {
			fg.heldPtrs = make(map[*types.Var]string)
		}
		fg.heldPtrs[root] = name
		return func() { delete(fg.heldPtrs, root) }
	case objValue:
		// A temporary named by the order pass (auto&& gp_t0 = f()) is an rvalue if f gave a value
		// and an lvalue if it gave a reference, which keep() tells apart.
		val := fg.exactExpr(sel.X)
		if t := fg.temps[sel.X]; t != "" {
			val = movedName(t)
		}
		*values = append(*values, call("gp::cgo::keep", val))
		name := param()
		old, had := fg.temps[sel.X]
		fg.temps[sel.X] = name
		return func() {
			if had {
				fg.temps[sel.X] = old
			} else {
				delete(fg.temps, sel.X)
			}
		}
	}
	return func() {}
}

// deferArg returns the C++ of the argument a of a deferred call, as the frame stores it: evaluated
// now; a C/C++ value goes through gp::cgo::defer_arg, which copies a variable and moves a temporary
// (std.move(x), f()), and is a clear error for a type that can't be copied or moved. A conversion to
// a number (C.int(n)) is stored as it is.
func (fg *funcGen) deferArg(a ast.Expr) cppir.Expr {
	x := fg.exactExpr(a)
	if !fg.g.pkg.Opaque.IsOpaque(a) {
		return x
	}
	switch u := ast.Unparen(a).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr:
		if fg.temps[ast.Unparen(a)] == "" {
			return call("gp::cgo::defer_arg", x)
		}
	case *ast.CompositeLit:
		return x // a prvalue, built where the frame takes it
	case *ast.CallExpr:
		if fg.g.pkg.Conversion(u) != nil {
			return x
		}
		if r, ok := fg.g.cRefOf(u.Fun); ok && r.kind == interop.Type {
			if n, ok := r.x.(*cppir.Name); ok && isBuiltinC(n.Name) {
				return x
			}
		}
	}
	return call("gp::cgo::defer_arg", fg.cMoved(a, x))
}

// keptBoxes returns the boxed C/C++ locals that the frame of the deferred call c must hold besides
// the object's own box (deferLocals): the thunk takes each as a parameter that it doesn't use.
func (fg *funcGen) keptBoxes(c *ast.CallExpr) []*types.Var {
	var boxes []*types.Var
	for _, v := range deferLocals(fg.g.info, c) {
		if _, held := fg.heldBoxes[v]; !held && fg.isCBox(v) {
			boxes = append(boxes, v)
		}
	}
	return boxes
}

// holdBox returns the statement that makes the frame of the function hold the box name of a C/C++
// local whose address is taken (addrHeld), or nil if v needs none: the frame keeps the box until
// every deferred call has run, however early or late each was registered.
func (fg *funcGen) holdBox(v *types.Var, name string) cppir.Stmt {
	if fg.fn == nil || fg.fn.defers == nil || fg.top == nil || !fg.isCBox(v) {
		return nil
	}
	if fg.addrHeld == nil {
		fg.addrHeld = fg.addrAnalysis().heldLocals()
	}
	if !fg.addrHeld[v] {
		return nil
	}
	return holdStmt(name)
}

// holdStmt returns gp_df.gp_hold(name): see holdBox.
func holdStmt(name string) cppir.Stmt {
	return &cppir.ExprStmt{X: frameCall("gp_hold", cxxName(name))}
}

// isSlot reports whether the result r of the function is a gp::result_slot (result.hpp): a result
// of a C/C++ type that has no name in the program, in a function with defers, which the return
// statements set and the exit takes. A named result is a variable of its own.
func (fg *funcGen) isSlot(r *types.Var) bool {
	return fg.fn.defers != nil && cObject(r.Type()) && (r.Name() == "" || r.Name() == "_") && !fg.isBoxed(r)
}

// resultValue returns the C++ of the result variable r (named name) as the function returns it, at
// its exit or in a bare return: the variable (a box is read through, (*r)), where a C/C++ object
// is moved out of an unboxed variable, which nothing sees any more, and out of a slot; a boxed one
// is copied if C++ can copy it (the closures that share the box keep their object), else moved
// (gp::cgo::result_of).
func (fg *funcGen) resultValue(r *types.Var, name string) cppir.Expr {
	switch {
	case fg.isSlot(r):
		return &cppir.Call{Fun: &cppir.Member{X: cxxName(name), Name: "gp_take"}}
	case fg.isCBox(r):
		return call("gp::cgo::result_of", deref(cxxName(name)))
	case fg.isBoxed(r):
		return &cppir.Paren{X: deref(cxxName(name))}
	case cObject(r.Type()):
		return movedName(name)
	}
	return cxxName(name)
}

// returnMove returns the C++ x of the value e of the return statement being lowered, as the
// result is made from it: a C/C++ object in a local that nothing sees after the return is moved
// (static_cast<T&&>(v)), where Go copies: its type may not be copyable, and the copy is
// never seen. That holds for an unboxed local used once in the results (no closure or deferred
// call can see it; a result variable is not moved into itself). A boxed one (the closures that
// share the box keep their object) is copied if C++ can, else moved (gp::cgo::result_of). store is
// whether the value is stored into a result variable or a std::tuple, which C++ copies from a
// variable; the return of one value is not moved by hand, so C++ can elide the copy (NRVO).
func (fg *funcGen) returnMove(e ast.Expr, x cppir.Expr, store bool) cppir.Expr {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok || fg.retStmt == nil || fg.temps[ast.Unparen(e)] != "" {
		return x
	}
	v, ok := fg.g.info.Uses[id].(*types.Var)
	if !ok || isPkgVar(v) || !cObject(v.Type()) || v.IsField() {
		return x
	}
	if fg.isBoxed(v) {
		return call("gp::cgo::result_of", deref(cxxName(fg.cname(v, id.Pos()))))
	}
	if fg.isResult(v) || !store {
		return x // (the return of one value moves a local by itself, and keeps C++'s NRVO)
	}
	uses := 0
	for _, r := range fg.retStmt.Results {
		ast.Inspect(r, func(n ast.Node) bool {
			if u, ok := n.(*ast.Ident); ok && fg.g.info.Uses[u] == v {
				uses++
			}
			return true
		})
	}
	if uses > 1 {
		return x
	}
	return movedName(fg.cname(v, id.Pos()))
}

// withHold returns the declaration decl, followed by the statement hold if it is not nil (holdBox),
// as one line.
func withHold(pos cppir.Pos, decl cppir.Stmt, hold cppir.Stmt) cppir.Stmt {
	if hold == nil {
		return decl
	}
	return &cppir.Line{Pos: pos, Stmts: []cppir.Stmt{decl, hold}}
}

// maybeNoDefault reports whether the C/C++ type of the field f can be a class with no default
// constructor, which gocpp can't see: not a raw pointer or one of C's number types, which always
// have a zero value (so the struct's code carries no check for those).
func (g *gen) maybeNoDefault(f *types.Var) bool {
	switch t := ast.Unparen(g.typeExprOf(f)).(type) {
	case *ast.StarExpr:
		return false
	case *ast.SelectorExpr:
		return !(g.isC(t.X) && cBuiltins[t.Sel.Name] != "")
	}
	return true
}

// rootIdentOf returns the variable that the operand e of & starts at: x in &x, &x[i], &x.f,
// &x[i].f and &(*x).f (a conservative reading: the address of a part of x counts as the address of
// x for the hold and escape rules), or nil. (cAddr, box.go, takes the same forms for a C/C++ root.)
func rootIdentOf(e ast.Expr) *ast.Ident {
	for {
		switch y := ast.Unparen(e).(type) {
		case *ast.Ident:
			return y
		case *ast.SelectorExpr:
			e = y.X
		case *ast.IndexExpr:
			e = y.X
		case *ast.StarExpr:
			e = y.X
		default:
			return nil
		}
	}
}
