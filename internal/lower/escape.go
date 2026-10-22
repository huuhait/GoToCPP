package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/check"
)

// Escape analysis (Phase 15 step 15.2, ADR-114).
//
// The boxing analysis (box.go) puts every local variable whose address is taken on the heap,
// and the lowering of &T{...} and new(T) always allocates. This pass finds the objects whose
// address never outlives them, and keeps those on the C++ stack:
//
//   - a local variable x of a function (declared by `x := ...` or `var x ...`) whose only
//     reason to be boxed is that its address is taken (&x, &x.f, &x.arr[i]) or that a pointer
//     method is called on it, when every pointer to it goes straight into a borrowed
//     parameter (borrow.go: ADR-086) or is held by a local pointer variable that does;
//   - an allocation &T{...} or new(T) that is held by such a local pointer variable
//     (p := &T{...}): the object is a member of gp::local_ptr<T> p, which is a gp::ptr<T>
//     that owns nothing;
//   - an allocation &T{...} that is an argument or receiver of a borrowed parameter: a
//     temporary of the call instead of a heap block.
//
// What the pointers may do is exactly what borrow.go lets a borrowed parameter do (the same
// walk, borrowWalk.useOK): read and write through it, compare it, call methods on it, and pass
// it on to borrowed parameters; anything else (stored, returned, sent, captured by a closure,
// used in go or defer, converted to an interface, passed to a parameter that is not borrowed,
// to an interface method, a func value or C/C++ code, reassigned, address-taken) makes the
// object escape. The summaries of the callees are the parameter summaries of ADR-086, so
// unknown code (C/C++, unsafe.Pointer, interface calls) is escaping by construction, and the
// analysis is interprocedural without a second fixpoint.
//
// A pointer variable p is held by exactly one object: p := &x, p := &T{...} or p := new(T)
// in a statement of a block, with p never assigned again, never address-taken and never
// captured. A pointer that is copied to another variable is not followed: the copy makes the
// object escape.
//
// The variables that stay on the stack keep their entry in the map of boxed variables, with
// the reason boxStack: a callee can change them through the pointer it borrows, so the rules
// that treat a boxed variable as memory (order of evaluation, stable arguments) still apply.

// An escapeInfo is the result of the escape analysis of one function declaration.
type escapeInfo struct {
	stack map[*types.Var]bool // the locals that stay on the stack
	// held are the allocations (&T{...}, new(T)) that a local pointer variable holds: the object is
	// in the variable's gp::local_ptr. They map to that variable.
	held map[ast.Expr]*types.Var
	// holders are those pointer variables.
	holders map[*types.Var]bool
	// decided are the variables the analysis decided about (stay or move).
	decided map[*types.Var]bool
	// temps are the allocations &T{...} that go straight to a borrowed parameter or receiver: a
	// temporary of the call.
	temps map[ast.Expr]bool
	// refs are the element variables that are const references to the element (elemref.go).
	refs  map[*types.Var]bool
	notes []escapeNote
}

// An escapeNote is one decision of the analysis for the diagnostics (-gp:m).
type escapeNote struct {
	pos    token.Pos
	msg    string // like go build -gcflags=-m: "moved to heap: x", "&T{...} does not escape"
	detail string // why (shown by -gp:m=2), or ""
	only2  bool   // shown by -gp:m=2 only
}

// escapeOf returns the escape analysis of the function declaration d (once). A function without a
// body, a generic function, a method of a generic type and a function that calls recover keep
// every variable as the boxing analysis chose.
func (g *gen) escapeOf(d *ast.FuncDecl) *escapeInfo {
	if e, ok := g.escCache[d]; ok {
		return e
	}
	if g.escCache == nil {
		g.escCache = make(map[*ast.FuncDecl]*escapeInfo)
	}
	e := &escapeInfo{
		stack:   make(map[*types.Var]bool),
		held:    make(map[ast.Expr]*types.Var),
		holders: make(map[*types.Var]bool),
		decided: make(map[*types.Var]bool),
		temps:   make(map[ast.Expr]bool),
		refs:    make(map[*types.Var]bool),
	}
	g.escCache[d] = e
	if d.Body == nil || d.Type.TypeParams != nil || g.opts.NoEscape {
		return e
	}
	fn, _ := g.info.Defs[d.Name].(*types.Func)
	if fn == nil {
		return e
	}
	if recv := fn.Signature().Recv(); recv != nil {
		t := recv.Type()
		if p, ok := types.Unalias(t).(*types.Pointer); ok {
			t = p.Elem()
		}
		named, _ := types.Unalias(t).(*types.Named)
		if named == nil || named.TypeParams().Len() > 0 {
			return e
		}
	}
	w := &escWalker{
		g:      g,
		d:      d,
		out:    e,
		bi:     g.boxInfoOf(d),
		assign: assignedVars(g.info, d.Body),
		levels: make(map[*types.Var]bool),
		objs:   make(map[*types.Var]*escObject),
		ptrs:   make(map[*types.Var]*escPtrVar),
		allocs: make(map[ast.Expr]*escObject),
	}
	w.bw = &borrowWalk{
		g: g, cands: make(map[*types.Var]bool), esc: make(map[*types.Var]string),
		pure: true, pureV: true, deps: make(map[*types.Func]bool),
	}
	w.run()
	w.elemRefs()
	w.closureNotes()
	return e
}

// An escObject is something whose address might escape: a local variable, or an allocation.
type escObject struct {
	v       *types.Var // the variable, or nil for an allocation
	site    ast.Expr   // the allocation expression, or nil for a variable
	pos     token.Pos  // where to report it
	declPos token.Pos  // where it is declared (closureUse)
	holder  *types.Var // the pointer variable that holds it, if one does
	reason  string     // why it escapes, "" while it does not
	sites   []escSite  // every place its address is taken, for the notes
	direct  bool       // an allocation that goes straight to a borrowed parameter or receiver
	cost    int64      // the object and holder's maximum storage (the function's stack budget)
}

// An escSite is a place where an address of an object is made.
type escSite struct {
	pos  token.Pos
	text string // "&x", "&x.f", "x.Inc()" (receiver), "&T{...}", "new(T)"
	ok   bool
	why  string
}

// An escPtrVar is a local pointer variable that holds the address of one object.
type escPtrVar struct {
	v      *types.Var
	obj    *escObject
	reason string // why one of its uses lets the object escape, "" while none does
}

type escWalker struct {
	g      *gen
	d      *ast.FuncDecl
	out    *escapeInfo
	bi     *boxResult
	assign map[*types.Var]bool
	bw     *borrowWalk

	levels map[*types.Var]bool // the variables declared by a statement of a block
	objs   map[*types.Var]*escObject
	ptrs   map[*types.Var]*escPtrVar
	allocs map[ast.Expr]*escObject
	order  []*escObject // in source order
}

const exposureBits = 1<<boxAddr | 1<<boxMethod

// run does the analysis.
func (w *escWalker) run() {
	g := w.g
	// Pass 1: the variables that a statement of a block declares.
	ast.Inspect(w.d.Body, func(n ast.Node) bool {
		var list []ast.Stmt
		switch n := n.(type) {
		case *ast.BlockStmt:
			list = n.List
		case *ast.CaseClause:
			list = n.Body
		case *ast.CommClause:
			list = n.Body
		default:
			return true
		}
		for _, s := range list {
			switch s := s.(type) {
			case *ast.AssignStmt:
				if s.Tok == token.DEFINE {
					for _, l := range s.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							if v, ok := g.info.Defs[id].(*types.Var); ok {
								w.levels[v] = true
							}
						}
					}
				}
			case *ast.DeclStmt:
				if gd, ok := s.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
					for _, spec := range gd.Specs {
						for _, id := range spec.(*ast.ValueSpec).Names {
							if v, ok := g.info.Defs[id].(*types.Var); ok {
								w.levels[v] = true
							}
						}
					}
				}
			}
		}
		return true
	})

	// Pass 2: the objects and the places where their addresses are made and used.
	var stack []ast.Node
	ast.Inspect(w.d.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		w.bw.stack = stack
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				w.addrSite(n)
			}
		case *ast.CallExpr:
			if w.g.isBuiltinCall(n, "new") {
				w.allocSite(n)
			}
		case *ast.SelectorExpr:
			w.receiverSite(n)
		case *ast.Ident:
			w.pointerUse(n)
		}
		return true
	})

	w.finish()
}

// stackableVar reports whether the variable v is one the analysis may keep on the stack: a local
// that a statement of a block declares, boxed for no reason but its address, not captured, of a
// type that has a plain C++ value.
func (w *escWalker) stackableVar(v *types.Var) bool {
	if v == nil || !w.levels[v] || v.Name() == "_" || w.bi.captured[v] {
		return false
	}
	bits := w.bi.all[v]
	if bits == 0 || bits&^exposureBits != 0 {
		return false
	}
	return w.plainValue(v.Type())
}

// A C++ stack does not grow, unlike a goroutine's stack in Go, and a function can call itself:
// every object that stays on the stack because it does not escape is a frame that the program did
// not have before (a box on the heap). So an object is kept only when it is small, and a function
// keeps a few of them only. (A variable that is not captured or address-taken has no such limit
// beyond maxStackVarSize: it was on the stack before.)
const (
	maxStackObjectSize = 2 * 1024  // the largest object that the analysis keeps on the stack
	maxStackObjectsSum = 16 * 1024 // all of them in one function
)

// plainValue reports whether a value of type t is an ordinary C++ value of a Go type that the
// stack can hold: complete, no C/C++ part, no type parameter, and small (maxStackObjectSize).
func (w *escWalker) plainValue(t types.Type) bool {
	if !complete(t) || hasC(t) || hasTypeParam(t) || cObject(t) || largeVar(t) {
		return false
	}
	return check.Sizes().Sizeof(t) <= maxStackObjectSize
}

// stackObjectCost charges the object, the pointer base of a local_ptr and alignment padding.
// Plain variables and call temporaries have no pointer base, but charging it too is conservative.
// Sizes describes the C++ layout, not Go's: gp::ptr and gp::string are larger than Go's values.
func stackObjectCost(t types.Type) int64 {
	sizes := check.Sizes()
	pointer := types.NewPointer(t)
	alignment := max(sizes.Alignof(t), sizes.Alignof(pointer))
	return sizes.Sizeof(t) + sizes.Sizeof(pointer) + alignment - 1
}

// object returns the escObject of the variable v, making it on first use.
func (w *escWalker) object(v *types.Var) *escObject {
	o := w.objs[v]
	if o == nil {
		o = &escObject{v: v, pos: v.Pos(), declPos: v.Pos(), cost: stackObjectCost(v.Type())}
		w.objs[v] = o
		w.order = append(w.order, o)
	}
	return o
}

// addrSite handles the expression &x: the address of a local variable, of a part of one, or a new
// object &T{...}.
func (w *escWalker) addrSite(n *ast.UnaryExpr) {
	g := w.g
	operand := ast.Unparen(n.X)
	if lit, ok := operand.(*ast.CompositeLit); ok {
		t := g.info.TypeOf(lit)
		switch under(t).(type) {
		case *types.Struct, *types.Array:
		default:
			return // &[]T{...}, &map[K]V{...}: stay boxed
		}
		if g.cxxLit(lit) || !w.plainValue(t) {
			return
		}
		w.alloc(n, "&"+litText(lit))
		return
	}
	root := (&boxer{info: g.info}).root(n.X)
	if root == nil || !w.stackableVar(root) {
		return
	}
	o := w.object(root)
	text := "&" + types.ExprString(n.X)
	w.classify(o, n, text)
}

// allocSite handles new(T).
func (w *escWalker) allocSite(n *ast.CallExpr) {
	t := w.g.info.TypeOf(n.Args[0])
	switch under(t).(type) {
	case *types.Struct, *types.Array, *types.Basic:
	default:
		return
	}
	if !w.plainValue(t) {
		return
	}
	w.alloc(n, "new("+types.ExprString(n.Args[0])+")")
}

// alloc handles an allocation expression n.
func (w *escWalker) alloc(n ast.Expr, text string) {
	o := &escObject{site: n, pos: n.Pos(), declPos: n.Pos()}
	if p, ok := types.Unalias(w.g.info.TypeOf(n)).(*types.Pointer); ok {
		o.cost = stackObjectCost(p.Elem())
	}
	w.allocs[n] = o
	w.order = append(w.order, o)
	w.classify(o, n, text)
}

// receiverSite handles x.M where M is a pointer method of a local variable x (or of a part of
// it): the address of x is the receiver.
func (w *escWalker) receiverSite(n *ast.SelectorExpr) {
	b := &boxer{info: w.g.info}
	if !b.pointerMethod(n) {
		return
	}
	root := b.root(n.X)
	if root == nil || !w.stackableVar(root) {
		return
	}
	o := w.object(root)
	sel := w.g.info.Selections[n]
	w.bw.declPos = o.declPos
	w.bw.why = ""
	why := w.bw.closureUse()
	ok := why == ""
	if ok {
		ok = w.bw.methodOK(0, n, sel, false)
		why = w.bw.why
	}
	text := "&" + types.ExprString(n.X)
	o.sites = append(o.sites, escSite{pos: n.X.Pos(), text: text, ok: ok, why: why})
	if !ok {
		o.escape(why)
	}
}

// escape records that the object escapes, with the first reason.
func (o *escObject) escape(why string) {
	if o.reason == "" {
		if why == "" {
			why = "its address is used where the analysis cannot follow it"
		}
		o.reason = why
	}
}

// classify decides the use of the address expression n (the top of the stack) of the object o: a
// borrowed argument, the initialization of a local pointer variable, or an escape.
func (w *escWalker) classify(o *escObject, n ast.Expr, text string) {
	// The initialization of a pointer variable: the variable holds the object.
	if p := w.heldBy(n); p != nil && (o.holder == nil || o.v != nil) {
		if o.site != nil && w.bindingReadsName(n, p.Name()) {
			// bind may stage this initializer before declaring the shadowing variable. A
			// local_ptr cannot be moved from that temporary without invalidating its address.
			why := "its initializer may need staging before the shadowing declaration"
			o.sites = append(o.sites, escSite{pos: n.Pos(), text: text, why: why})
			o.escape(why)
			return
		}
		w.ptrs[p] = &escPtrVar{v: p, obj: o}
		o.holder = p
		o.sites = append(o.sites, escSite{pos: n.Pos(), text: text, ok: true})
		if o.site != nil {
			w.out.held[o.site] = p
		}
		return
	}
	w.bw.declPos = o.declPos
	w.bw.why = ""
	ok := w.bw.useOK(nil)
	why := w.bw.why
	if ok && o.site != nil {
		// A new object that goes straight to a borrowed parameter or receiver: a temporary of the
		// call, for &T{...} (new(T) allocates).
		if u, isAddr := n.(*ast.UnaryExpr); isAddr && u.Op == token.AND {
			o.direct = true
		} else {
			ok = false
			why = "it is not held by a local variable"
		}
	}
	o.sites = append(o.sites, escSite{pos: n.Pos(), text: text, ok: ok, why: why})
	if !ok {
		o.escape(why)
	}
}

// bindingReadsName reports whether the initializer uses a variable or function with the new
// binding's name. Go resolves that use before the declaration, whereas C++ puts the new binding
// in scope in its own initializer. This is conservative: ordering may hoist the use first, but
// otherwise bind must stage the allocation, and local_ptr deliberately cannot be moved.
func (w *escWalker) bindingReadsName(n ast.Expr, name string) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if found {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			switch obj := w.g.info.Uses[id].(type) {
			case *types.Var:
				found = !obj.IsField()
			case *types.Func:
				found = true
			}
		}
		return !found
	})
	return found
}

// heldBy returns the local pointer variable that the statement around n (an address expression)
// declares as its only variable with n as its only value, if it may hold it: it is declared by
// a statement of a block, has a pointer type, is never assigned, captured or address-taken.
func (w *escWalker) heldBy(n ast.Expr) *types.Var {
	k := len(w.bw.stack) - 1
	e := ast.Node(n)
	for k > 0 {
		if p, ok := w.bw.stack[k-1].(*ast.ParenExpr); ok && p.X == e {
			e = p
			k--
			continue
		}
		break
	}
	if k == 0 {
		return nil
	}
	var id *ast.Ident
	switch s := w.bw.stack[k-1].(type) {
	case *ast.AssignStmt:
		if s.Tok != token.DEFINE || len(s.Lhs) != 1 || len(s.Rhs) != 1 || ast.Node(s.Rhs[0]) != e {
			return nil
		}
		id, _ = s.Lhs[0].(*ast.Ident)
	case *ast.ValueSpec:
		if len(s.Names) != 1 || len(s.Values) != 1 || ast.Node(s.Values[0]) != e {
			return nil
		}
		id = s.Names[0]
	default:
		return nil
	}
	if id == nil {
		return nil
	}
	v, ok := w.g.info.Defs[id].(*types.Var)
	if !ok || v.Name() == "_" || !w.levels[v] || w.bi.captured[v] || w.assign[v] {
		return nil
	}
	if w.bi.first[v] != 0 {
		return nil
	}
	pt, ok := types.Unalias(v.Type()).(*types.Pointer)
	if !ok || !w.plainValue(pt.Elem()) {
		return nil
	}
	return v
}

// pointerUse classifies a use of a local pointer variable that holds an object.
func (w *escWalker) pointerUse(id *ast.Ident) {
	v, ok := w.g.info.Uses[id].(*types.Var)
	if !ok {
		return
	}
	// The variable may be declared later in the walk than its first use only for a name that is
	// not this variable; a use always follows the declaration.
	pv := w.ptrs[v]
	if pv == nil {
		return
	}
	w.bw.declPos = pv.obj.declPos
	w.bw.why = ""
	if !w.bw.useOK(nil) {
		if pv.reason == "" {
			pv.reason = w.bw.why
			if pv.reason == "" {
				pv.reason = "it is used as a value: stored, returned, converted to an interface " +
					"or reassigned"
			}
		}
		pv.obj.escape(pv.reason)
	}
}

// finish records the verdicts and the notes.
func (w *escWalker) finish() {
	// Local closures already reserved the storage of captures that used to live in boxes.
	used := w.bi.stackBytes
	for _, o := range w.order {
		if o.reason == "" {
			if used+o.cost > maxStackObjectsSum {
				o.reason = "the objects that the function keeps on the stack would be larger " +
					"than the budget"
			} else {
				used += o.cost
			}
		}
		stays := o.reason == ""
		switch {
		case o.v != nil:
			w.out.decided[o.v] = true
			if stays {
				w.out.stack[o.v] = true
			}
			w.noteVar(o, stays)
		default:
			if stays && o.holder != nil {
				w.out.holders[o.holder] = true // held by its pointer variable
			} else if stays && o.direct {
				w.out.temps[o.site] = true
			} else {
				stays = false
				if o.reason == "" {
					o.reason = "it is not held by a local variable or passed straight to a " +
						"borrowed parameter"
				}
				delete(w.out.held, o.site)
			}
			w.noteAlloc(o, stays)
		}
	}
}

// note records a decision for the diagnostics.
func (w *escWalker) note(pos token.Pos, msg, detail string) {
	w.out.notes = append(w.out.notes, escapeNote{pos: pos, msg: msg, detail: detail})
}

func (w *escWalker) noteVar(o *escObject, stays bool) {
	name := o.v.Name()
	if stays {
		for _, s := range o.sites {
			w.note(s.pos, s.text+" does not escape", "")
		}
		return
	}
	for _, s := range o.sites {
		w.note(s.pos, s.text+" escapes to heap", s.why)
	}
	w.note(o.pos, "moved to heap: "+name, o.reason)
}

func (w *escWalker) noteAlloc(o *escObject, stays bool) {
	for _, s := range o.sites {
		if stays {
			w.note(s.pos, s.text+" does not escape", "")
		} else {
			w.note(s.pos, s.text+" escapes to heap", o.reason)
		}
	}
	if len(o.sites) == 0 {
		text := "new(T)"
		if o.site != nil {
			text = types.ExprString(o.site)
		}
		w.note(o.pos, text+" escapes to heap", o.reason)
	}
}

// litText returns how the diagnostics write the composite literal lit: T{...}, or T{} when it is
// empty (the form of go build -gcflags=-m).
func litText(lit *ast.CompositeLit) string {
	t := ""
	if lit.Type != nil {
		t = types.ExprString(lit.Type)
	}
	if len(lit.Elts) == 0 {
		return t + "{}"
	}
	return t + "{...}"
}

// sorted returns the diagnostics of the function, in the order of their places.
func (e *escapeInfo) sorted() []escapeNote {
	out := append([]escapeNote(nil), e.notes...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].pos < out[j].pos })
	return out
}

// isBuiltinCall reports whether c is a call of the builtin name.
func (g *gen) isBuiltinCall(c *ast.CallExpr, name string) bool {
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := g.info.Uses[id].(*types.Builtin)
	return ok && b.Name() == name && len(c.Args) == 1
}

// An EscapeLog collects the decisions of the escape analysis for `gocpp -gp:m`, in the form of
// `go build -gcflags=-m`: one line for each decision, at the .gp place, like
//
//	main.gp:20:6: moved to heap: d
//	main.gp:23:7: &Counter{} does not escape
//	main.gp:7:11: c does not escape
//	main.gp:13:11: leaking param: c
//
// With Level 2 (`-gp:m=2`) a line says why. It never changes the C++. A nil *EscapeLog is the
// disabled sink.
type EscapeLog struct {
	Level int // 1: the decisions, 2: with the reasons

	mu    sync.Mutex
	lines []EscapeLine
	seen  map[EscapeLine]bool
}

// An EscapeLine is one diagnostic.
type EscapeLine struct {
	Pkg  string // the import path of the package
	File string
	Line int
	Col  int
	Msg  string
}

// NewEscapeLog returns an empty sink for the given level (1 or 2).
func NewEscapeLog(level int) *EscapeLog {
	if level < 1 {
		level = 1
	}
	return &EscapeLog{Level: level, seen: make(map[EscapeLine]bool)}
}

func (e *EscapeLog) add(l EscapeLine) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.seen[l] {
		return
	}
	e.seen[l] = true
	e.lines = append(e.lines, l)
}

// Lines returns the diagnostics sorted by package, file, line and column.
func (e *EscapeLog) Lines() []EscapeLine {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	out := append([]EscapeLine(nil), e.lines...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.Pkg != b.Pkg:
			return a.Pkg < b.Pkg
		case a.File != b.File:
			return a.File < b.File
		case a.Line != b.Line:
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	return out
}

// noteEscape records the decisions about the function f in the sink: the address-taking
// expressions and allocations (the escape analysis), the variables that live in a box, and the
// pointer parameters. The standard library is left out.
func (g *gen) noteEscape(f *funcInfo) {
	sink := g.opts.Escape
	if sink == nil || g.mute != 0 || g.pkg.Load == nil || g.pkg.Load.Std || f.decl == nil {
		return
	}
	add := func(pos token.Pos, msg, detail string) {
		if !pos.IsValid() {
			return
		}
		p := g.fset.Position(pos)
		if sink.Level >= 2 && detail != "" {
			msg += ": " + strings.TrimSuffix(detail, ".")
		}
		line := EscapeLine{Pkg: g.pkg.Types.Path(), File: p.Filename, Line: p.Line, Col: p.Column, Msg: msg}
		sink.add(line)
	}
	if f.esc != nil {
		for _, n := range f.esc.sorted() {
			if n.only2 && sink.Level < 2 {
				continue
			}
			add(n.pos, n.msg, n.detail)
		}
	}
	// Variables in a box for another reason than their address, and those the analysis did not
	// consider (a parameter, a result, a loop variable).
	vars := make([]*types.Var, 0, len(f.boxed))
	for v, r := range f.boxed {
		onHeap := r != boxStack && r != boxRefCaptured
		if onHeap && v.Name() != "" && v.Name() != "_" && !strings.HasPrefix(v.Name(), "gp_") {
			vars = append(vars, v)
		}
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Pos() < vars[j].Pos() })
	for _, v := range vars {
		if f.esc != nil && f.esc.decided[v] {
			continue // (the escape analysis said why)
		}
		reason := boxReasonText(f.boxed[v])
		exposed := f.boxed[v] == boxAddr || f.boxed[v] == boxMethod
		if exposed && check.Sizes().Sizeof(v.Type()) > maxStackObjectSize {
			reason = "it is larger than 2 KiB, the most that the analysis keeps on the stack"
		}
		add(v.Pos(), "moved to heap: "+v.Name(), reason)
	}
	// The pointer parameters and the receiver.
	s := g.borrowOf(f.obj)
	if s == nil {
		return
	}
	sig := f.obj.Signature()
	one := func(v *types.Var, i int) {
		if v == nil || v.Name() == "" || v.Name() == "_" || !g.borrowType(v.Type()) {
			return
		}
		if s.borrowed(i) {
			add(v.Pos(), v.Name()+" does not escape", "")
			return
		}
		why := s.why[i]
		if why == "" {
			why = s.notBorrowed
		}
		add(v.Pos(), "leaking param: "+v.Name(), why)
	}
	one(sig.Recv(), -1)
	for i := range sig.Params().Len() {
		one(sig.Params().At(i), i)
	}
}

// boxReasonText is the reason a variable is in a box, for the diagnostics.
func boxReasonText(r boxReason) string {
	switch r {
	case boxAddr:
		return "its address is taken and escapes"
	case boxMethod:
		return "a pointer method is called on it and the receiver escapes"
	case boxSliced:
		return "an array in it is sliced"
	case boxCaptured:
		return "a closure captures it and it changes"
	case boxLarge:
		return "too large for the stack"
	case boxShared:
		return "a closure captures it and it is larger than 128 bytes"
	case boxCObject:
		return "a closure captures a C/C++ object"
	case boxDefer:
		return "a deferred call holds a C/C++ object or its address"
	case boxCEscape:
		return "its address leaves the function (a package variable, memory the caller keeps, a result or a send)"
	}
	return "boxed"
}
