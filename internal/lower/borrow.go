package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// Borrowed pointer parameters (Phase 15 step 15.3, ADR-086).
//
// A function whose pointer parameter (or pointer receiver) never outlives the call takes it as a
// gp::bptr<T>, a raw pointer, instead of an owning gp::ptr<T> that every call copies (one atomic
// increment and one decrement). The analysis is on the callee's body, one function at a time,
// with the summaries of the functions it calls (the fixpoint over the package, the packages one
// after the other in the order of their imports):
//
// A parameter p *T is *borrowable* when every use of p in the body is one of
//
//	p.f, (*p).f, p.f.g[i]...   a field of *p, read or written (the value, not its address)
//	*p, *p = v                 the whole value
//	p == q, p != nil           a comparison
//	p.M(...)                   a value method (it reads through p); a pointer method whose
//	                           receiver is p, or the address of a field of *p reached through
//	                           embedded structs (p.Embedded.M()), when the callee borrows it too
//	f(p), f(&p.f)              an argument to a parameter that the callee borrows
//	len(p), p[i]               of a pointer to an array
//
// and anything else makes p escape: assigned or stored anywhere (also x := p), returned,
// captured by a func literal or a range-over-func body, converted to an interface, sent on a
// channel, an argument of go or defer, passed to a parameter that the callee does not borrow, to
// an interface method, a func value or C/C++ code, p = ..., &p, a method value p.M, a slice of
// an array in *p (p.arr[:]), or the address of a part of *p (&p.f) that is not passed straight to
// a borrowed parameter. Unknown code counts as escaping. A function that is generic, a method
// of a generic type, exported in a library, //export-ed or without a body is not analysed: its
// parameters are not borrowed (and the callers pass owning pointers, as before).
//
// A second property decides what a caller may pass: a function is *pure* (it cannot release a
// pointer) when it, and everything it calls, makes no store of a value that can own memory (a
// pointer, slice, map, interface, func or channel, or a struct holding one) except into its own
// local variables, runs no unknown code (no call through an interface or func value, no C/C++
// operation, including implicit construction/destruction/copy/assignment, no go, defer, channel
// operation, select, func literal, range over a func) and calls only pure functions. While a pure
// function runs, no owner stored anywhere can be dropped, so a pointer that a caller loaded from
// a field, an element or a global stays valid for the call. A caller
// whose argument is not a temporary or an unchangeable local passes it raw only to a pure
// callee, otherwise it holds a copy for the call (borrowArg).
//
// Only the call's arguments are lent. The callee's body is lowered as before: a borrowed
// parameter has the operations of gp::ptr that the uses above need (->, *, ==).
//
// A parameter that is not borrowed is a const reference when it is a string, slice, map, channel,
// func, interface or pointer that the body never assigns, never returns as it is (it would be
// copied where a by-value parameter is moved) and that no closure boxes. A function is *pure for
// values* (pureV) when it also stores no string; the caller passes a field to such a function
// without a copy (borrowemit.go: refArg).

// A borrowSum is the result of the analysis for one function or method.
type borrowSum struct {
	recv   bool   // the pointer receiver is borrowed
	params []bool // the parameters, by position (only pointer parameters can be true)
	pure   bool   // the function cannot release a pointer
	// pureV: the function stores no pointer, slice, map, interface, func, channel or string except
	// into its own locals, and runs no unknown code: no value that a caller passes by const
	// reference from memory can change while it runs.
	pureV bool
	// ref are the parameters passed as const references (refRecv: the receiver): a pointer that
	// is not borrowed, a string, slice, map, channel, func or interface that the body never
	// assigns. The caller passes a copy unless nothing can change the argument during the call.
	ref     []bool
	refRecv bool
	// eligible says that the signature may change (the function is analysed); an ineligible
	// function can still be pure.
	eligible bool
	// why says, for the reason log (gocpp emit -annotate), why a pointer parameter is not borrowed
	// (index -1: the receiver), and notBorrowed why the function keeps all owning pointers.
	why         map[int]string
	notBorrowed string
}

// setWhy records why the parameter i (-1: the receiver) is not borrowed.
func (s *borrowSum) setWhy(i int, msg string) {
	if s.why == nil {
		s.why = make(map[int]string)
	}
	if _, ok := s.why[i]; !ok {
		s.why[i] = msg
	}
}

// borrowable reports whether the pointer parameter i of the function is borrowed (-1: the
// receiver).
func (s *borrowSum) borrowed(i int) bool {
	if s == nil {
		return false
	}
	if i < 0 {
		return s.recv
	}
	return i < len(s.params) && s.params[i]
}

// isRef reports whether the parameter i (-1: the receiver) is a const reference.
func (s *borrowSum) isRef(i int) bool {
	if s == nil {
		return false
	}
	if i < 0 {
		return s.refRecv
	}
	return i < len(s.ref) && s.ref[i]
}

// hasRef reports whether the function has a const reference parameter or receiver.
func (s *borrowSum) hasRef() bool {
	if s == nil {
		return false
	}
	if s.refRecv {
		return true
	}
	for _, b := range s.ref {
		if b {
			return true
		}
	}
	return false
}

// hasBorrowed reports whether the function borrows any parameter or its receiver.
func (s *borrowSum) hasBorrowed() bool {
	if s == nil {
		return false
	}
	if s.recv {
		return true
	}
	for _, b := range s.params {
		if b {
			return true
		}
	}
	return false
}

// borrowOf returns the summary of the function or method fn (of any package that the Program
// lowers), or nil if it is not analysed.
func (g *gen) borrowOf(fn *types.Func) *borrowSum {
	if fn == nil {
		return nil
	}
	fn = fn.Origin()
	o := g.other(fn.Pkg())
	if o == nil {
		return nil
	}
	o.ensureBorrow()
	return o.borrow[fn]
}

// declOf returns the declaration of the function or method fn of this package, or nil.
func (g *gen) declOf(fn *types.Func) *ast.FuncDecl {
	if g.funcDecls == nil {
		g.funcDecls = make(map[*types.Func]*ast.FuncDecl)
		for _, f := range g.pkg.Load.Files {
			for _, d := range f.AST.Decls {
				if d, ok := d.(*ast.FuncDecl); ok && d.Body != nil {
					if obj, ok := g.info.Defs[d.Name].(*types.Func); ok {
						g.funcDecls[obj] = d
					}
				}
			}
		}
	}
	return g.funcDecls[fn]
}

// borrowIneligible says why the function declaration d may not get borrowed parameters, or ""
// when it may: it has a body, is not generic, is not a method of a generic type, is not part of
// a library's interface and is not an //export function.
func (g *gen) borrowIneligible(d *ast.FuncDecl, fn *types.Func) string {
	if d.Body == nil || d.Name.Name == "_" || d.Name.Name == "init" {
		return "the function has no body that can be analysed"
	}
	if d.Type.TypeParams != nil {
		return "the function is generic (its instances keep owning pointers)"
	}
	if g.opts.Library && fn.Exported() {
		return "the function is exported by a library, whose C++ API keeps its signatures"
	}
	if d.Doc != nil {
		for _, c := range d.Doc.List {
			if strings.HasPrefix(c.Text, "//export ") || strings.HasPrefix(c.Text, "//export\t") {
				return "the function is an //export function (its C signature is fixed)"
			}
		}
	}
	if recv := fn.Signature().Recv(); recv != nil {
		t := recv.Type()
		if p, ok := types.Unalias(t).(*types.Pointer); ok {
			t = p.Elem()
		}
		named, _ := types.Unalias(t).(*types.Named)
		if named == nil || named.TypeParams().Len() > 0 {
			return "the method belongs to a generic type"
		}
	}
	for i := range fn.Signature().Params().Len() {
		if largeVar(fn.Signature().Params().At(i).Type()) {
			return "the function has a parameter larger than 128 KiB, which is already a const reference"
		}
	}
	return ""
}

// borrowType reports whether a parameter of type t can be borrowed: *T with the C++ type
// gp::ptr<T> (not a pointer to a C/C++ type, not unsafe.Pointer).
func (g *gen) borrowType(t types.Type) bool {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	elem := p.Elem()
	return complete(elem) && !hasC(elem) && !isInterface(elem) && !hasTypeParam(elem)
}

// ensureBorrow runs the analysis over the package's functions (once).
func (g *gen) ensureBorrow() {
	if g.borrow != nil {
		return
	}
	g.borrow = make(map[*types.Func]*borrowSum)
	var decls []*ast.FuncDecl
	for _, f := range g.pkg.Load.Files {
		for _, d := range f.AST.Decls {
			d, ok := d.(*ast.FuncDecl)
			if !ok || d.Body == nil {
				continue
			}
			fn, _ := g.info.Defs[d.Name].(*types.Func)
			if fn == nil {
				continue
			}
			sig := fn.Signature()
			s := &borrowSum{params: make([]bool, sig.Params().Len()), ref: make([]bool, sig.Params().Len()), pure: true, pureV: true}
			if why := g.borrowIneligible(d, fn); why != "" {
				s.notBorrowed = why
			} else {
				s.eligible = true
				if r := sig.Recv(); r != nil && g.borrowType(r.Type()) {
					s.recv = true
				}
				for i := range sig.Params().Len() {
					if g.borrowType(sig.Params().At(i).Type()) {
						s.params[i] = true
					}
				}
			}
			if s.eligible && g.awareBody(d.Body) {
				s.eligible, s.recv, s.params = false, false, make([]bool, len(s.params))
				s.notBorrowed = "the function calls recover, so a deferred call gives it a ticket (it keeps its signature)"
			}
			if s.eligible {
				// A parameter that lives in a box is a gp::ptr<T> of its own (box.go).
				boxed := g.boxedOf(d)
				if r := sig.Recv(); r != nil && boxed[r] != 0 {
					s.recv = false
					s.setWhy(-1, "it lives in a box (its address is taken or a closure changes it)")
				}
				for i := range s.params {
					if boxed[sig.Params().At(i)] != 0 {
						s.params[i] = false
						s.setWhy(i, "it lives in a box (its address is taken or a closure changes it)")
					}
				}
			}
			g.borrow[fn] = s
			decls = append(decls, d)
		}
	}
	// A function that is not generic, but whose method set is used through generics, is fine:
	// instances call it through the same signature.
	// A worklist: a function is analysed again only when a function of this package that it
	// consults changed (the summaries only lose properties, so the order does not matter).
	index := make(map[*types.Func]int, len(decls))
	for i, d := range decls {
		fn, _ := g.info.Defs[d.Name].(*types.Func)
		index[fn] = i
	}
	callers := make(map[*types.Func]map[int]bool)
	queued := make([]bool, len(decls))
	queue := make([]int, 0, len(decls))
	for i := range decls {
		queued[i] = true
		queue = append(queue, i)
	}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		queued[i] = false
		d := decls[i]
		fn, _ := g.info.Defs[d.Name].(*types.Func)
		s := g.borrow[fn]
		esc, pure, pureV, deps := g.analyseBorrow(d, fn)
		for dep := range deps {
			if callers[dep] == nil {
				callers[dep] = make(map[int]bool)
			}
			callers[dep][i] = true
		}
		changed := false
		if s.pure && !pure {
			s.pure, changed = false, true
		}
		if s.pureV && !pureV {
			s.pureV, changed = false, true
		}
		if r := fn.Signature().Recv(); r != nil && s.recv && esc[r] != "" {
			s.recv, changed = false, true
			s.setWhy(-1, esc[r])
		}
		for k := range s.params {
			if s.params[k] && esc[fn.Signature().Params().At(k)] != "" {
				s.params[k], changed = false, true
				s.setWhy(k, esc[fn.Signature().Params().At(k)])
			}
		}
		if changed {
			for c := range callers[fn] {
				if !queued[c] {
					queued[c] = true
					queue = append(queue, c)
				}
			}
		}
	}
	// The parameters that are not borrowed and that nothing assigns are const references.
	for _, d := range decls {
		fn, _ := g.info.Defs[d.Name].(*types.Func)
		s := g.borrow[fn]
		if !s.eligible {
			continue
		}
		sig := fn.Signature()
		assigned := assignedVars(g.info, d.Body)
		returned := returnedVars(g.info, d.Body)
		boxed := g.boxedOf(d)
		refOK := func(v *types.Var) bool {
			return v != nil && v.Name() != "" && v.Name() != "_" && !assigned[v] && !returned[v] && boxed[v] == 0 && g.refType(v.Type())
		}
		if r := sig.Recv(); r != nil && !s.recv && refOK(r) {
			if _, isPtr := types.Unalias(r.Type()).(*types.Pointer); isPtr {
				s.refRecv = true
			}
		}
		for i := range s.ref {
			if p := sig.Params().At(i); !s.params[i] && refOK(p) {
				s.ref[i] = true
			}
		}
	}
}

// refType reports whether a parameter of type t is passed as a const reference when the body does
// not assign it: a pointer (that is not borrowed), a string, slice, map, channel, func or
// interface of Go types. (A struct or array parameter is a copy that the body may change, and a
// large one is already a const reference.)
func (g *gen) refType(t types.Type) bool {
	if !complete(t) || hasC(t) || hasTypeParam(t) || largeVar(t) {
		return false
	}
	if g.borrowType(t) {
		return true
	}
	switch under(t).(type) {
	case *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Interface:
		return true
	case *types.Basic:
		return isString(t) && basicOf(t) != nil
	}
	return false
}

// returnedVars returns the variables that a return statement of the function itself (not of a func
// literal in it) returns as they are: `return p`. A by-value parameter is moved out there, a const
// reference would be copied, which costs a count that the caller's copy of the argument (the
// usual case for a pointer loaded from a field) already paid: such a parameter stays by value.
func returnedVars(info *tinfo, body *ast.BlockStmt) map[*types.Var]bool {
	out := make(map[*types.Var]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			for _, e := range x.Results {
				if id, ok := ast.Unparen(e).(*ast.Ident); ok {
					if v, ok := info.Uses[id].(*types.Var); ok {
						out[v] = true
					}
				}
			}
		}
		return true
	})
	return out
}

// assignedVars returns the variables that the statements of body assign as a whole (v = ..., v op=
// ..., v++, for v = range), also inside func literals.
func assignedVars(info *tinfo, body *ast.BlockStmt) map[*types.Var]bool {
	out := make(map[*types.Var]bool)
	mark := func(e ast.Expr) {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok {
			if v, ok := info.Uses[id].(*types.Var); ok {
				out[v] = true
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				mark(l)
			}
		case *ast.IncDecStmt:
			mark(x.X)
		case *ast.RangeStmt:
			if x.Tok == token.ASSIGN {
				if x.Key != nil {
					mark(x.Key)
				}
				if x.Value != nil {
					mark(x.Value)
				}
			}
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				mark(x.X) // the address of the parameter itself
			}
		}
		return true
	})
	return out
}

// analyseBorrow returns the pointer parameters (and the receiver) of fn that escape in the body
// of d with the summaries as they are now, and whether the body is pure.
func (g *gen) analyseBorrow(d *ast.FuncDecl, fn *types.Func) (map[*types.Var]string, bool, bool, map[*types.Func]bool) {
	a := &borrowWalk{g: g, cands: make(map[*types.Var]bool), esc: make(map[*types.Var]string), pure: true, pureV: true, deps: make(map[*types.Func]bool)}
	sig := fn.Signature()
	s := g.borrow[fn]
	if s.eligible {
		if r := sig.Recv(); r != nil && s.recv {
			a.cands[r] = true
		}
		for i := range sig.Params().Len() {
			if s.params[i] {
				a.cands[sig.Params().At(i)] = true
			}
		}
	}

	// A C++ value in the signature can run user code without an AST call: parameter
	// copies/destructors and the initialization of named results are opaque operations.
	if g.borrowHasC(sig) || sig.Recv() != nil && g.borrowHasC(sig.Recv().Type()) {
		a.pure, a.pureV = false, false
	}
	a.walk(d.Body)
	return a.esc, a.pure, a.pureV, a.deps
}

// borrowHasC memoizes the conservative foreign-operation boundary. Go aggregates and
// containers holding C++ values count too: their implicit operations can invoke callbacks.
func (g *gen) borrowHasC(t types.Type) bool {
	if t == nil {
		return false
	}
	if g.borrowC == nil {
		g.borrowC = make(map[types.Type]bool)
	}
	if foreign, ok := g.borrowC[t]; ok {
		return foreign
	}
	foreign := hasC(t)
	g.borrowC[t] = foreign
	return foreign
}

// sumOf returns the summary of fn and records that the walk depends on it when fn is of this package.
func (a *borrowWalk) sumOf(fn *types.Func) *borrowSum {
	if fn != nil && fn.Origin().Pkg() == a.g.pkg.Types {
		a.deps[fn.Origin()] = true
	}
	return a.g.borrowOf(fn)
}

// A borrowWalk is the analysis of one function body.
type borrowWalk struct {
	g     *gen
	cands map[*types.Var]bool   // the parameters that may still be borrowed
	esc   map[*types.Var]string // the candidates found to escape, with the reason
	why   string                // the reason that useOK found
	pure  bool
	pureV bool // pure also for strings (borrowSum.pureV)
	// deps are the functions of this package whose summaries the walk read.
	deps  map[*types.Func]bool
	stack []ast.Node
}

func (a *borrowWalk) walk(body *ast.BlockStmt) {
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			a.stack = a.stack[:len(a.stack)-1]
			return true
		}
		a.stack = append(a.stack, n)
		a.purity(n)
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := a.g.info.Uses[id].(*types.Var); ok && a.cands[v] && a.esc[v] == "" {
				a.why = ""
				if !a.useOK(v) {
					a.esc[v] = a.why
					if a.why == "" {
						a.esc[v] = "it is used as a value: stored, returned, converted to an interface or reassigned"
					}
				}
			}
		}
		return true
	})
}

// parent returns the k-th ancestor of the node at the top of the stack (1: its parent).
func (a *borrowWalk) parent(k int) ast.Node {
	if i := len(a.stack) - 1 - k; i >= 0 {
		return a.stack[i]
	}
	return nil
}

// useOK reports whether the use of the candidate v at the top of the stack (an identifier) keeps
// v borrowable.
func (a *borrowWalk) useOK(v *types.Var) bool {
	// The uses inside a func literal, a range over a func (its body is a closure), a go or a
	// defer statement are captured or stored for later.
	for _, n := range a.stack {
		switch n := n.(type) {
		case *ast.FuncLit:
			return a.fail("a func literal captures it")
		case *ast.GoStmt, *ast.DeferStmt:
			return a.fail("a go or defer statement keeps it for later")
		case *ast.RangeStmt:
			if isRangeFunc(a.g.info, n) {
				return a.fail("the body of a range over a func is a closure that captures it")
			}
		}
	}
	g := a.g
	k := 0 // the stack index (from the top) of the expression e being classified
	e := ast.Expr(a.stack[len(a.stack)-1].(*ast.Ident))
	up := func() { // e = its parent, if that is a ParenExpr
		for {
			p, ok := a.parent(k + 1).(*ast.ParenExpr)
			if !ok || p.X != e {
				return
			}
			k++
			e = p
		}
	}
	up()
	switch p := a.parent(k + 1).(type) {
	case *ast.BinaryExpr:
		if p.Op != token.EQL && p.Op != token.NEQ {
			return a.fail("it is an operand of " + p.Op.String())
		}
		// p == q, p == nil: the other side is a pointer too (not an interface value, which
		// converts p).
		other := p.X
		if other == e {
			other = p.Y
		}
		t := g.info.TypeOf(other)
		if t != nil && (isPointer(t) || t == types.Typ[types.UntypedNil]) {
			return true
		}
		return a.fail("it is compared with an interface value, which converts it")
	case *ast.CallExpr:
		if p.Fun == e {
			return false
		}
		return a.argOK(p, e)
	case *ast.IndexExpr:
		// v[i] of a pointer to an array: the element is storage inside *v
		if p.X == e && arrayPtr(g.info.TypeOf(e)) != nil {
			return a.valueChain(k+1, p)
		}
		return false
	case *ast.StarExpr:
		if p.X != e {
			return false
		}
		return a.valueChain(k+1, p)
	case *ast.SelectorExpr:
		if p.X != e {
			return false
		}
		sel := g.info.Selections[p]
		if sel == nil {
			return false
		}
		switch sel.Kind() {
		case types.FieldVal:
			return a.valueChain(k+1, p)
		case types.MethodVal:
			return a.methodOK(k+1, p, sel, true)
		}
		return false
	case *ast.AssignStmt:
		for _, l := range p.Lhs {
			if l == e {
				return a.fail("it is reassigned")
			}
		}
		return a.fail("it is copied into a variable or field")
	case *ast.ReturnStmt:
		return a.fail("it is returned")
	case *ast.SendStmt:
		return a.fail("it is sent on a channel")
	case *ast.CompositeLit, *ast.KeyValueExpr, *ast.ValueSpec:
		return a.fail("it is stored in a composite literal or a variable")
	case *ast.UnaryExpr:
		if p.Op == token.AND {
			return a.fail("its address is taken")
		}
	}
	return false
}

// fail records the reason why the use of the candidate keeps it from being borrowed, and
// returns false.
func (a *borrowWalk) fail(msg string) bool {
	if a.why == "" {
		a.why = msg
	}
	return false
}

// valueChain classifies the use of the value storage x inside *v: x is the expression at stack
// index k (from the top), rooted at *v through value fields and array elements. It reports
// whether that use is fine: reading or writing the value is; taking its address is only fine as
// an argument to a borrowed parameter, and slicing an array in it is not.
func (a *borrowWalk) valueChain(k int, x ast.Expr) bool {
	g := a.g
	for {
		// Extend the chain while x is a struct or an array value: x.f, x[i], (x).
		p := a.parent(k + 1)
		switch p := p.(type) {
		case *ast.ParenExpr:
			k++
			x = p
			continue
		case *ast.SelectorExpr:
			if p.X != x {
				return true
			}
			sel := g.info.Selections[p]
			if sel == nil {
				return true
			}
			switch sel.Kind() {
			case types.FieldVal:
				if !a.isValueType(g.info.TypeOf(x)) {
					return true // x is a pointer, an interface or a map: p.f loads through it
				}
				k++
				x = p
				continue
			case types.MethodVal:
				t := g.info.TypeOf(x)
				if isPointer(t) || isInterface(t) {
					return true // a method of a loaded pointer or interface
				}
				// Named scalars and containers have pointer methods too: taking their method
				// value can retain the address of the storage inside *v, just like a struct.
				return a.methodOK(k+1, p, sel, false)
			}
			return false
		case *ast.IndexExpr:
			if p.X != x {
				return true // x is the index
			}
			if isArray(g.info.TypeOf(x)) {
				k++
				x = p
				continue
			}
			return true // an element of a slice, a map, a string
		case *ast.SliceExpr:
			if p.X == x && isArray(g.info.TypeOf(x)) {
				return a.fail("a slice of an array in it shares its block (it needs an owner)")
			}
			return true
		case *ast.UnaryExpr:
			if p.Op != token.AND || p.X != x {
				return true
			}
			// &x: fine only straight into a borrowed parameter.
			if call, ok := a.parent(k + 2).(*ast.CallExpr); ok && call.Fun != p {
				return a.argOK(call, p)
			}
			return a.fail("the address of a part of it is taken and kept (it needs an owner)")
		}
		return true
	}
}

// isValueType reports whether t is a struct or an array (storage that a chain from *v continues
// through).
func (a *borrowWalk) isValueType(t types.Type) bool {
	if t == nil {
		return false
	}
	switch under(t).(type) {
	case *types.Struct, *types.Array:
		return true
	}
	return false
}

// methodOK classifies the method selection p (x.M, the expression at stack index k from the top)
// whose operand x is v itself (a pointer, direct) or a value inside *v. A value method only
// reads. A pointer method gets a pointer into *v unless the path reaches it through an embedded
// pointer: then it must be called, and the callee must borrow its receiver.
func (a *borrowWalk) methodOK(k int, p *ast.SelectorExpr, sel *selection, direct bool) bool {
	fn, _ := sel.Obj().(*types.Func)
	if fn == nil || fn.Signature().Recv() == nil {
		return false
	}
	if isIfaceMethod(fn) {
		return true // a method of an embedded interface value inside *v: v is not passed
	}
	r := pathOf(sel)
	related := true
	t := sel.Recv()
	if p, ok := under(t).(*types.Pointer); ok {
		if !direct {
			return true // x is a loaded pointer: unrelated to v
		}
		t = p.Elem()
	}
	for _, i := range r.path {
		st, ok := under(t).(*types.Struct)
		if !ok {
			return false
		}
		t = st.Field(i).Type()
		if isPointer(t) {
			related = false
			break
		}
	}
	if related {
		if _, ptrRecv := types.Unalias(fn.Signature().Recv().Type()).(*types.Pointer); !ptrRecv {
			return true // a value method reads *v
		}
	} else {
		return true // the receiver is a pointer that was loaded, not v
	}
	// A pointer method on (a part of) *v: v is lent to its receiver.
	call, ok := a.parent(k + 1).(*ast.CallExpr)
	if !ok || call.Fun != ast.Expr(p) {
		return a.fail("it is turned into a method value (a closure that keeps it)")
	}
	if a.sumOf(fn).borrowed(-1) && a.callPlain(call) {
		return true
	}
	return a.fail("it is the receiver of " + fn.Name() + ", which keeps its receiver")
}

// argOK reports whether the expression e (v, or &part of *v) as an argument of the call is lent
// to a borrowed parameter.
func (a *borrowWalk) argOK(call *ast.CallExpr, e ast.Expr) bool {
	g := a.g
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if b, ok := g.info.Uses[id].(*types.Builtin); ok {
			if (b.Name() == "len" || b.Name() == "cap") && len(call.Args) == 1 && arrayPtr(g.info.TypeOf(call.Args[0])) != nil {
				return true
			}
			return a.fail("it is an argument of the builtin " + b.Name())
		}
	}
	fn := g.calleeFunc(call)
	if fn == nil {
		if g.info.Types[call.Fun].IsType() {
			return a.fail("it is converted to another type")
		}
		return a.fail("it is passed to code that is not analysed (an interface method, a func value or C/C++)")
	}
	sig := fn.Signature()
	i := -1
	for j, x := range call.Args {
		if x == e {
			i = j
		}
	}
	if i < 0 || sig.Variadic() && i >= sig.Params().Len()-1 || i >= sig.Params().Len() {
		return a.fail("it is passed to a variadic parameter of " + fn.Name())
	}
	if a.sumOf(fn).borrowed(i) && a.callPlain(call) {
		return true
	}
	return a.fail("it is passed to " + fn.Name() + ", which keeps that parameter")
}

// callPlain reports whether the call is lowered as a plain call of the callee's function with
// one C++ argument for each Go argument: not the spread of a call with several results.
func (a *borrowWalk) callPlain(call *ast.CallExpr) bool {
	if len(call.Args) == 1 && isTuple(a.g.info.TypeOf(call.Args[0])) {
		return false
	}
	return true
}

// calleeFunc returns the function or method that the call c calls by name (a function of a
// package that the Program lowers, a method of a concrete type), or nil: a func value, an
// interface method, a builtin, a conversion, an instance of a generic function.
func (g *gen) calleeFunc(c *ast.CallExpr) *types.Func {
	if g.info.Types[c.Fun].IsType() {
		return nil
	}
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		if _, inst := g.info.Instances[fun]; inst {
			return nil
		}
		fn, _ := g.info.Uses[fun].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		if sel := g.info.Selections[fun]; sel != nil {
			if sel.Kind() != types.MethodVal {
				return nil
			}
			fn, _ := sel.Obj().(*types.Func)
			if fn == nil || isIfaceMethod(fn) {
				return nil
			}
			return fn
		}
		if _, inst := g.info.Instances[fun.Sel]; inst {
			return nil
		}
		return g.qualifiedFunc(fun)
	}
	return nil
}

// purity updates the purity of the body for the node n (see the top of the file).
func (a *borrowWalk) purity(n ast.Node) {
	if !a.pure && !a.pureV {
		return
	}
	g := a.g
	both := func() { a.pure, a.pureV = false, false }
	// An opaque value can run constructors, destructors, copy/move/assignment operators
	// or conversions even when the source contains no CallExpr. This includes declaration
	// identifiers and Go values with foreign fields/elements, not just explicit C++ calls.
	if e, ok := n.(ast.Expr); ok {
		if id, ok := e.(*ast.Ident); ok {
			switch g.info.ObjectOf(id).(type) {
			case *types.PkgName, *types.Builtin, *types.Label:
				return // these are not values; their placeholder type can be types.Invalid
			}
		}
		if g.borrowHasC(g.info.TypeOf(e)) {
			both()
			return
		}
	}
	switch x := n.(type) {
	case *ast.FuncLit, *ast.GoStmt, *ast.DeferStmt, *ast.SendStmt, *ast.SelectStmt:
		both()
	case *ast.UnaryExpr:
		if x.Op == token.ARROW {
			both()
		}
	case *ast.RangeStmt:
		switch under(g.info.TypeOf(x.X)).(type) {
		case *types.Signature, *types.Chan:
			both()
		}
		if x.Tok == token.ASSIGN {
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if e != nil {
					a.store(e)
				}
			}
		}
	case *ast.AssignStmt:
		for _, l := range x.Lhs {
			a.store(l)
		}
	case *ast.IncDecStmt:
		a.store(x.X)
	case *ast.CallExpr:
		if g.info.Types[x.Fun].IsType() {
			return
		}
		if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok {
			if b, ok := g.info.Uses[id].(*types.Builtin); ok {
				if !a.pureBuiltin(b.Name(), x, false) {
					a.pure = false
				}
				if !a.pureBuiltin(b.Name(), x, true) {
					a.pureV = false
				}
				return
			}
		}
		fn := g.calleeFunc(x)
		if fn == nil {
			both()
			return
		}
		s := a.sumOf(fn)
		if s == nil || !s.pure {
			a.pure = false
		}
		if s == nil || !s.pureV {
			a.pureV = false
		}
	}
}

// store updates the purity for the assignment to l.
func (a *borrowWalk) store(l ast.Expr) {
	if !a.privateStore(l, false) {
		a.pure = false
	}
	if !a.privateStore(l, true) {
		a.pureV = false
	}
}

// pureBuiltin reports whether the builtin call keeps the body pure (strict: also for strings).
func (a *borrowWalk) pureBuiltin(name string, c *ast.CallExpr, strict bool) bool {
	g := a.g
	owns := func(t types.Type) bool { return mayOwn(t) || strict && holdsString(t) }
	switch name {
	case "len", "cap", "min", "max", "new", "make", "print", "println", "panic", "complex", "real", "imag":
		return true
	case "append", "copy", "clear":
		// They write elements of arrays that other slices share.
		if len(c.Args) == 0 {
			return false
		}
		t := g.info.TypeOf(c.Args[0])
		if t == nil {
			return false
		}
		if s, ok := under(t).(*types.Slice); ok {
			return !owns(s.Elem())
		}
		if m, ok := under(t).(*types.Map); ok {
			return !owns(m.Key()) && !owns(m.Elem())
		}
		return false
	case "delete":
		if len(c.Args) == 0 {
			return false
		}
		if m, ok := under(g.info.TypeOf(c.Args[0])).(*types.Map); ok {
			return !owns(m.Key()) && !owns(m.Elem())
		}
	}
	return false
}

// holdsString reports whether a value of type t is or holds a string.
func holdsString(t types.Type) bool {
	switch u := under(t).(type) {
	case *types.Basic:
		return u.Info()&types.IsString != 0
	case *types.Array:
		return holdsString(u.Elem())
	case *types.Struct:
		for i := range u.NumFields() {
			if holdsString(u.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}

// privateStore reports whether assigning to the lvalue l can't drop an owner that something
// else holds: the value stored over is not an owner (a number, a bool, a string, or arrays and
// structs of them), or l is a local variable of the function (or a part of one that a value
// path reaches: x.f, x.arr[i]), which nothing else sees while the function runs.
func (a *borrowWalk) privateStore(l ast.Expr, strict bool) bool {
	g := a.g
	l = ast.Unparen(l)
	if id, ok := l.(*ast.Ident); ok && id.Name == "_" {
		return true
	}
	if t := g.info.TypeOf(l); t != nil && !mayOwn(t) && !(strict && holdsString(t)) {
		return true
	}
	for {
		switch x := l.(type) {
		case *ast.Ident:
			v, ok := g.info.Uses[x].(*types.Var)
			if !ok {
				v, ok = g.info.Defs[x].(*types.Var)
			}
			return ok && !isPkgVar(v) && !v.IsField()
		case *ast.ParenExpr:
			l = x.X
		case *ast.SelectorExpr:
			sel := g.info.Selections[x]
			if sel == nil || sel.Kind() != types.FieldVal || sel.Indirect() || isPointer(g.info.TypeOf(x.X)) {
				return false
			}
			l = x.X
		case *ast.IndexExpr:
			if !isArray(g.info.TypeOf(x.X)) {
				return false
			}
			l = x.X
		default:
			return false
		}
	}
}

// boxedOf returns boxedVars of the function declaration d, computed once (the lowering of d and the
// borrow analysis both need it). The map is shared: a caller that changes it takes a copy.
func (g *gen) boxedOf(d *ast.FuncDecl) map[*types.Var]boxReason {
	if isGenericDecl(d) {
		// Each instance of a generic function has variable objects of its own (generics.go: apply),
		// and the same declaration: a map computed for one instance names none of the variables of
		// another, which would then have no box for a variable that a closure changes.
		return boxedVars(g.info, d)
	}
	if b, ok := g.boxCache[d]; ok {
		return b
	}
	if g.boxCache == nil {
		g.boxCache = make(map[*ast.FuncDecl]map[*types.Var]boxReason)
	}
	b := boxedVars(g.info, d)
	g.boxCache[d] = b
	return b
}

// isGenericDecl reports whether d is a generic function, or a method of a generic type: the
// declarations that are lowered once for each instance.
func isGenericDecl(d *ast.FuncDecl) bool {
	if d.Type.TypeParams != nil {
		return true
	}
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return false
	}
	recv := ast.Unparen(d.Recv.List[0].Type)
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = ast.Unparen(star.X)
	}
	switch recv.(type) {
	case *ast.IndexExpr, *ast.IndexListExpr:
		return true
	}
	return false
}
