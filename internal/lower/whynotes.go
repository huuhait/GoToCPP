package lower

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// The notes of the reason log (why.go): one function for each decision that the annotated view
// explains. Each one returns at once when the log is off, so the lowering's code only calls it
// (`g.noteX(...)`); the text of the reasons is here, not scattered through the lowering. A
// message is one line, in the present tense, and says why the C++ looks as it does.
//
// The decisions themselves stay where they were made: a note is added next to the code that
// chooses (boxedVars' reasons, the single exit of defer.go, the allocation of toInterface). A
// test of internal/explain checks that each kind of note shows up for a program that needs it, so
// that moving a decision without moving its note is caught.

// typeText returns the Go spelling of t in the package of g.
func (g *gen) typeText(t types.Type) string {
	if t == nil {
		return "?"
	}
	return types.TypeString(t, types.RelativeTo(g.pkg.Types))
}

// checkTail is the end of the message of a check that -nochecks removes.
func (g *gen) checkTail() string {
	if g.noChecks() {
		return " (removed by -nochecks: undefined behavior then)"
	}
	return " (gocpp build -nochecks removes it)"
}

// noteBoxed records why each variable of boxed lives in a box (box.go).
func (g *gen) noteBoxed(boxed map[*types.Var]boxReason) {
	if !g.noting() || len(boxed) == 0 {
		return
	}
	vars := make([]*types.Var, 0, len(boxed))
	for v := range boxed {
		vars = append(vars, v)
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Pos() < vars[j].Pos() })
	var clauses map[types.Object]*ast.CaseClause // the variable of a type switch has one object for each clause
	for _, v := range vars {
		if clauses == nil {
			clauses = make(map[types.Object]*ast.CaseClause)
			for n, obj := range g.info.Implicits {
				if cc, ok := n.(*ast.CaseClause); ok {
					clauses[obj] = cc
				}
			}
		}
		if cc := clauses[v]; cc != nil {
			// All the clauses' variables are declared at the switch header, but each box is made in
			// its own clause: the note goes to the clause.
			g.why(cc.Case, WhyBoxing, strings.Replace(boxText(v.Name(), boxed[v]), v.Name()+" is boxed", v.Name()+" is boxed in this case clause (the variable of a type switch is one variable for each clause)", 1))
			continue
		}
		name := v.Name()
		if name == "" {
			name = "an unnamed result" // gp_r0: the results of a function that needs variables for them
		}
		if boxed[v] == boxStack {
			g.why(v.Pos(), WhyBoxing, name+" stays on the C++ stack: its address is taken, but the escape "+
				"analysis found that the address does not escape, so pointers to it are gp::stack_ptr "+
				"values (no heap box, no counts)")
			continue
		}
		if boxed[v] == boxRefCaptured {
			g.why(v.Pos(), WhyBoxing, name+" stays on the C++ stack: only closures that are called where "+
				"they are made capture it, by reference (no heap box)")
			continue
		}
		g.why(v.Pos(), WhyBoxing, boxText(name, boxed[v]))
	}
}

// boxText is the sentence for a boxed variable name.
func boxText(name string, r boxReason) string {
	switch r {
	case boxAddr:
		return name + " is boxed: its address (or the address of a part of it) is taken, so it lives in a gp::ptr heap box that every pointer shares and that never dangles"
	case boxMethod:
		return name + " is boxed: a pointer-receiver method is called on it, which is (&" + name + ").M() and needs its address"
	case boxSliced:
		return name + " is boxed: an array in it is sliced, and the slice keeps the array alive"
	case boxCaptured:
		return name + " is boxed: a closure captures it and it changes, so the function and the closure share one box (a copy would miss the changes)"
	case boxLarge:
		return name + " is boxed: it is larger than 128 KiB, which could overflow the C++ stack"
	case boxShared:
		return name + " is boxed: a closure captures it and it is larger than 128 bytes, so it is shared instead of copied into the lambda"
	case boxCObject:
		return name + " is boxed: a closure captures a C/C++ object, which the box keeps in place and alive"
	case boxDefer:
		return name + " is boxed: a deferred call holds a C/C++ object or its address, and the defer frame keeps the box"
	}
	return name + " is boxed"
}

// noteClosure records the lambda of the func literal fl, number n, and how it captures each
// variable of the functions around it.
func (fg *funcGen) noteClosure(fl *ast.FuncLit, n int) {
	g := fg.g
	if !g.noting() {
		return
	}
	g.why(fl.Pos(), WhyClosures, fmt.Sprintf("func literal #%d is a C++ lambda wrapped in gp::func (the stack trace names it func%d); copies of the func value share the captured state", n, n))
	seen := make(map[*types.Var]bool)
	ast.Inspect(fl.Body, func(node ast.Node) bool {
		id, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		v, ok := g.info.Uses[id].(*types.Var)
		if !ok || v.IsField() || isPkgVar(v) || inside(fl, v.Pos()) || seen[v] {
			return true
		}
		seen[v] = true
		if fg.isBoxed(v) {
			g.why(id.Pos(), WhyClosures, "captures "+v.Name()+" by sharing its box ("+fg.boxedWhy(v)+")")
		} else {
			g.why(id.Pos(), WhyClosures, "captures "+v.Name()+" by copy into the lambda: it never changes after the closure exists and is at most 128 bytes")
		}
		return true
	})
}

// boxedWhy is the short reason that the variable v is boxed.
func (fg *funcGen) boxedWhy(v *types.Var) string {
	switch fg.boxed[v] {
	case boxAddr:
		return "its address is taken"
	case boxMethod:
		return "a pointer method is called on it"
	case boxSliced:
		return "an array in it is sliced"
	case boxCaptured:
		return "it changes after capture"
	case boxLarge:
		return "it is large"
	case boxShared:
		return "it is larger than 128 bytes"
	case boxCObject:
		return "a C/C++ object"
	case boxDefer:
		return "held by a defer frame"
	case boxStack:
		return "on the stack"
	case boxRefCaptured:
		return "on the stack, captured by reference"
	}
	return "boxed"
}

// noteDeferFrame records the defer frame of the function that starts at pos.
func (g *gen) noteDeferFrame(pos token.Pos, results bool) {
	if !g.noting() {
		return
	}
	tail := ""
	if results {
		tail = "; the results are variables so deferred closures can change them"
	}
	g.why(pos, WhyDefer, "this function has a defer: the body runs in try { } with a gp::defer_frame (gp_df); every exit goes through gp_df.gp_run(), which runs the deferred calls last in, first out, then rethrows a pending panic"+tail)
}

// noteDeferReturn records the exit of a return in a function with defers.
func (g *gen) noteDeferReturn(pos token.Pos, label string, results bool) {
	if !g.noting() {
		return
	}
	if !results {
		g.why(pos, WhyDefer, "return jumps to "+label+", which runs the deferred calls and then leaves the function")
		return
	}
	g.why(pos, WhyDefer, "return assigns the results first, then jumps to "+label+", which runs the deferred calls and returns the results (Go's order)")
}

// How a deferred or spawned call is stored (deferredCall), for noteDeferred.
type deferHow int

const (
	howFunc    deferHow = iota // a function of the program
	howRecover                 // recover itself
	howOpaque                  // a call on a C/C++ object
	howValue                   // a func value
	howLiteral                 // a func literal
	howThunk                   // a builtin, a method or a package function
)

// noteDeferred records a defer or go statement at pos; how says how its call is stored. The text
// of a go statement names go_fn and go_call, which have no frame: only a defer has gp_push.
func (fg *funcGen) noteDeferred(pos token.Pos, spawn bool, how deferHow) {
	g := fg.g
	if !g.noting() {
		return
	}
	if spawn {
		text := map[deferHow]string{
			howFunc:    "a function of the program: gp::go_fn(f, args...) passes the arguments on to it on the new goroutine",
			howRecover: "recover itself: it does nothing there, as in Go",
			howOpaque:  "a call on a C/C++ object, in a lambda that gp::go_call owns",
			howValue:   "a func value: copied now, called on the new goroutine by gp::go_call(f)",
			howLiteral: "a func literal: its lambda is passed to gp::go_call, so the stack trace names it func1, func2...",
			howThunk:   "a builtin, a method or a package function: a lambda that makes the call, with the arguments as parameters, passed to gp::go_call(lambda, args...)",
		}[how]
		g.why(pos, WhyGoroutines, "go: the function and its arguments are evaluated now, in this goroutine; the call runs on a new goroutine (an OS thread, ADR-007/026): "+text)
		return
	}
	text := map[deferHow]string{
		howFunc:    "a function of the program, called with its arguments (gp_push)",
		howRecover: "recover itself: it does nothing there, as in Go",
		howOpaque:  "a call on a C/C++ object, stored in a lambda that the frame keeps alive",
		howValue:   "a func value: copied now, called later (gp_push_call)",
		howLiteral: "a func literal: its lambda goes into the frame (gp_push_call), so the stack trace names it func1, func2...",
		howThunk:   "a builtin, a method or a package function: a lambda that makes the call, with the arguments as parameters (gp_push_call)",
	}[how]
	g.why(pos, WhyDefer, "defer: the function and its arguments are evaluated now and stored in the frame; the call runs when the function exits: "+text)
}

// noteMethodValue records a method value x.M or a method expression T.M (x, with the selection s).
func (g *gen) noteMethodValue(x *ast.SelectorExpr, s *selection, r recvPath) {
	if !g.noting() {
		return
	}
	name := x.Sel.Name
	switch {
	case s.Kind() == types.MethodExpr:
		g.why(x.Pos(), WhyMethods, "method expression "+types.ExprString(x)+": a lambda whose first parameter is the receiver, then the method's parameters; no receiver is bound")
	case isInterface(r.end):
		g.why(x.Pos(), WhyMethods, "method value "+name+" of an interface: binds the interface value now (gp::method_recv: a nil interface panics here, not where the func is called); the lambda calls the method through the method table")
	case r.ptrRecv:
		g.why(x.Pos(), WhyMethods, "method value "+name+": binds the receiver pointer now; the lambda calls the pointer method T::"+name+"(p, ...) on it later")
	default:
		g.why(x.Pos(), WhyMethods, "method value "+name+": binds a copy of the receiver now (gp_r), as Go does; a later change to the variable is not seen by the func")
	}
}

// noteAware records that the function at pos calls recover.
func (g *gen) noteAware(pos token.Pos) {
	g.why(pos, WhyRecover, "this function calls recover(): it takes a ticket (gp::defer_ctx gp_dc) at entry; recover returns the panic value only when a deferred call of a panicking function calls it directly, as in Go")
}

// noteMethodCall records how the call of the method fn is written: base is the receiver's type
// at the end of the path, ptrRecv says that fn has a pointer receiver, ptr that the receiver
// expression is a pointer.
func (g *gen) noteMethodCall(base types.Type, ptrRecv, ptr bool, fn *types.Func, pos token.Pos) {
	if !g.noting() {
		return
	}
	name := fn.Name()
	switch {
	case isInterface(base):
		g.why(pos, WhyInterfaces, "interface method call "+name+": the call is the wrapper member of the interface's struct, which runs gp::icall(x, slot): the method is found in the method table of the interface value (no C++ virtual functions); a call on a nil interface panics like Go"+g.checkTail())
	case ptrRecv:
		g.why(pos, WhyMethods, "pointer method "+name+": a static call T::"+name+"(p, ...), the receiver pointer is the first argument; no dynamic dispatch")
	case ptr:
		g.why(pos, WhyMethods, "value method "+name+" through a pointer: p->"+name+"() checks p for nil and panics like Go"+g.checkTail())
	default:
		g.why(pos, WhyMethods, "value method "+name+": a plain member call; the body works on a copy of the receiver, or on a const reference when it cannot change it")
	}
}

// noteToIface records the conversion of a value of type from to the interface type to.
func (g *gen) noteToIface(from, to types.Type, pos token.Pos) {
	if !g.noting() || from == nil || isNilType(from) {
		return
	}
	t := g.typeText(from)
	switch {
	case isInterface(from):
		g.why(pos, WhyInterfaces, "interface to interface ("+t+" to "+g.typeText(to)+"): gp::iface_cast shares the box (a reference count increment), no allocation")
	case g.foreignVal(from):
		g.why(pos, WhyInterop, "a C/C++ value ("+t+") goes into an interface: its descriptor is made from its C++ type (gp::to_iface_foreign), and the value is copied into a new box")
	case isPointer(from):
		g.why(pos, WhyInterfaces, "pointer "+t+" to "+g.typeText(to)+": no allocation, the interface shares the pointer's owner; a nil pointer still makes a non-nil interface")
	default:
		g.why(pos, WhyAlloc, "boxing "+t+" into "+g.typeText(to)+" allocates: gp::to_iface copies the value into a new reference-counted box; copies of the interface share it")
	}
}

// noteAssert records the type assertion to t.
func (g *gen) noteAssert(t types.Type, commaOK bool, pos token.Pos) {
	if !g.noting() {
		return
	}
	switch {
	case commaOK && isInterface(t):
		g.why(pos, WhyInterfaces, "v, ok := x.("+g.typeText(t)+"): gp::assert_iface2 asks whether the dynamic type has the methods; no panic, ok says")
	case commaOK:
		g.why(pos, WhyInterfaces, "v, ok := x.("+g.typeText(t)+"): gp::assert_type2 compares type descriptors; no panic, ok says")
	case isInterface(t):
		g.why(pos, WhyInterfaces, "x.("+g.typeText(t)+"): gp::assert_iface checks that the dynamic type has the methods, and panics with Go's message if not")
	default:
		g.why(pos, WhyInterfaces, "x.("+g.typeText(t)+"): gp::assert_type compares the type descriptor of the dynamic type, and panics with Go's message if it differs")
	}
}

// noteTypeSwitch records the type switch s.
func (g *gen) noteTypeSwitch(s *ast.TypeSwitchStmt) {
	if !g.noting() {
		return
	}
	g.why(s.Switch, WhyInterfaces, "type switch: an if/else chain of descriptor tests (gp::type_is, gp::type_has) in case order, on a const reference to the operand; v is declared only in the clauses that use it")
}

// noteSelect records the select statement s.
func (g *gen) noteSelect(s *ast.SelectStmt) {
	if !g.noting() {
		return
	}
	if len(s.Body.List) == 0 {
		g.why(s.Select, WhySelect, "select {} blocks this goroutine forever (gp::select())")
		return
	}
	g.why(s.Select, WhySelect, fmt.Sprintf("select with %d cases: each case becomes an object (gp::sel_send, gp::sel_recv, gp::sel_default) made in source order; gp::select picks one ready case at random (or waits) and the C++ switch runs its body", len(s.Body.List)))
}

// noteChan records a channel operation; fun is the runtime function that does it.
func (g *gen) noteChan(pos token.Pos, fun string) {
	if !g.noting() {
		return
	}
	var msg string
	switch fun {
	case "gp::send":
		msg = "send: blocks until a receiver is ready or the buffer has room; sending on a closed channel panics, on a nil channel blocks forever"
	case "gp::recv":
		msg = "receive: blocks until a value arrives; after close it gives the zero value"
	case "gp::recv2":
		msg = "receive with ok: like a receive, and ok is false once the channel is closed and empty"
	case "gp::make_chan":
		msg = "make(chan): allocates the channel; a gp::chan is a shared reference, so copies and directional views share one channel"
	case "gp::close":
		msg = "close: wakes the receivers; closing a nil or an already closed channel panics"
	default:
		return
	}
	g.why(pos, WhyChannels, msg)
}

// noteRange records how the range loop s over a value of type xt is written.
func (g *gen) noteRange(s *ast.RangeStmt, xt types.Type) {
	if !g.noting() {
		return
	}
	var msg string
	b := underBasic(xt)
	switch {
	case b != nil && b.Info()&types.IsString != 0:
		msg = "range over a string: gp::range_string decodes UTF-8 rune by rune; an invalid byte gives U+FFFD and steps one byte"
	case isSlice(xt):
		msg = "range over a slice: the iterator holds a copy of the slice header, so the loop covers the original length even if the body appends"
	case isMap(xt):
		msg = "range over a map: the iterator holds the table and starts at a random slot, so a key deleted before it is reached is skipped; the order is random, as in Go"
	case isChan(xt):
		msg = "range over a channel: receives until the channel is closed (gp::range_chan)"
	case arrayOf(xt) != nil:
		msg = "range over an array: the array is copied only where the body could change it"
	case b != nil && b.Info()&types.IsInteger != 0:
		msg = "range over an integer: a counting loop; each iteration has its own copy of the variable (Go 1.22)"
	default:
		return
	}
	g.why(s.For, WhyRange, msg)
}

// noteDeref records the dereference *p of a pointer.
func (g *gen) noteDeref(pos token.Pos) {
	if !g.noting() {
		return
	}
	g.why(pos, WhyChecks, "*p: dereferences a gp::ptr, which checks for nil and panics like Go"+g.checkTail())
}

// noteDivision records the division or remainder with the operator op.
func (g *gen) noteDivision(pos token.Pos, op token.Token, constDivisor bool) {
	if !g.noting() {
		return
	}
	name := "division"
	helper := "gp::div"
	if op == token.REM {
		name, helper = "remainder", "gp::rem"
	}
	if constDivisor {
		g.why(pos, WhyChecks, name+" by a constant that is not 0 or -1: the plain C++ operator, no check is needed")
		return
	}
	g.why(pos, WhyChecks, name+" of integers: "+helper+" panics for a zero divisor and gives Go's result for MIN / -1; these checks stay under -nochecks")
}

// noteShift records a shift by a count that is not a small constant.
func (g *gen) noteShift(pos token.Pos, op token.Token) {
	if !g.noting() {
		return
	}
	g.why(pos, WhyChecks, "shift by a count that is not a constant below the width: "+map[bool]string{false: "gp::shl", true: "gp::shr"}[op == token.SHR]+" panics for a negative count and gives Go's result for a count of the width or more; these checks stay under -nochecks")
}

// noteConcat records the string concatenation of n operands.
func (g *gen) noteConcat(pos token.Pos, n int) {
	if !g.noting() {
		return
	}
	if n > 2 {
		g.why(pos, WhyStrings, fmt.Sprintf("concatenation of %d strings: one gp::concat call allocates the result once (C++'s + would allocate for each +)", n))
		return
	}
	g.why(pos, WhyStrings, "string concatenation allocates the new string (a gp::string holds an owner, a pointer and a length)")
}

// noteIndex records the index expression x[i] of an operand of type t.
func (g *gen) noteIndex(pos token.Pos, t types.Type) {
	if !g.noting() {
		return
	}
	switch {
	case isString(t):
		g.why(pos, WhyChecks, "s[i] of a string: a bounds-checked byte read, panics with Go's \"index out of range\" message"+g.checkTail())
	case isSlice(t):
		g.why(pos, WhyChecks, "s[i] of a slice: gp::slice::operator[] checks the index and panics with Go's \"index out of range\" message"+g.checkTail())
	case arrayPtr(t) != nil:
		g.why(pos, WhyChecks, "p[i] of a pointer to an array: a nil check, then the index check of gp::array"+g.checkTail())
	case arrayOf(t) != nil:
		g.why(pos, WhyChecks, "a[i] of an array: gp::array::operator[] checks the index"+g.checkTail()+"; a constant index is checked by the compiler")
	}
}

// noteSliceExpr records the slice expression e on an operand of type t.
func (g *gen) noteSliceExpr(e *ast.SliceExpr, t types.Type) {
	if !g.noting() {
		return
	}
	if isString(t) {
		g.why(e.Lbrack, WhySlices, "s[a:b] of a string: gp::sub checks the bounds and shares the bytes of s: no copy")
		return
	}
	g.why(e.Lbrack, WhySlices, "s[a:b] of a slice: gp::sub checks the bounds against the capacity and shares the array (no copy); the result keeps the array alive")
}

// noteNew records new(T).
func (g *gen) noteNew(pos token.Pos, t types.Type) {
	if !g.noting() {
		return
	}
	g.why(pos, WhyAlloc, "new("+g.typeText(t)+") allocates a zeroed value in a reference-counted box (gp::new_); the pointer frees it with the last copy")
}

// noteMap records a map operation; what is "<operation> <runtime function>".
func (g *gen) noteMap(pos token.Pos, what string) {
	if !g.noting() {
		return
	}
	var msg string
	switch {
	case strings.HasSuffix(what, "gp::get"):
		msg = "m[k]: gp::get gives the zero value for a missing key and never inserts; a nil map reads as empty"
	case strings.HasSuffix(what, "gp::lookup"):
		msg = "v, ok := m[k]: gp::lookup gives the value and whether the key was there"
	case what == "store gp::set":
		msg = "m[k] = v: gp::set(m, k, v) stores the value, inserting the key if it is new (the key and the value are evaluated first, in Go's order); a write to a nil map panics, also under -nochecks"
	case what == "divide gp::set":
		msg = "m[k] /= y on integers: gp::set(m, k, gp::div(gp::get(m, k), y)) stores only after the division succeeded, so a zero divisor panics without inserting k; a write to a nil map panics, also under -nochecks"
	case what == "remainder gp::set":
		msg = "m[k] %= y on integers: gp::set(m, k, gp::rem(gp::get(m, k), y)) stores only after the remainder succeeded, so a zero divisor panics without inserting k; a write to a nil map panics, also under -nochecks"
	case what == "shift gp::ref":
		msg = "m[k] <<= n and m[k] >>= n: gp::ref(m, k) = gp::shl(gp::ref(m, k), n) (or gp::shr): the entry is inserted first, and a negative count panics like Go; a write to a nil map panics, also under -nochecks"
	case strings.HasSuffix(what, "gp::ref"):
		msg = "m[k]++, m[k] op= v and m[k] = append(m[k], ...) change the entry in place: gp::ref(m, k) gives a reference to it, inserting the zero value first when the key is missing; a write to a nil map panics, also under -nochecks"
	case strings.HasSuffix(what, "gp::map_of"):
		msg = "map literal: gp::map_of builds the map from the entries in gc's order (the constant keys first)"
	case strings.HasSuffix(what, "gp::make_map"):
		msg = "make(map): allocates the table; a gp::map is a shared reference, so copies are the same map"
	case strings.HasSuffix(what, "gp::erase"):
		msg = "delete(m, k): removes the key; nothing happens for a missing key or a nil map"
	default:
		return
	}
	g.why(pos, WhyMaps, msg)
}

// noteSliceBuiltin records make, append, copy or clear on slices.
func (g *gen) noteSliceBuiltin(c *ast.CallExpr, name string) {
	if !g.noting() {
		return
	}
	switch name {
	case "make":
		g.why(c.Pos(), WhyAlloc, "make([]T, n): allocates the zeroed backing array; the slice holds its owner, and a negative or too large n panics")
	case "append":
		g.why(c.Pos(), WhyAlloc, "append: writes in place when the capacity is enough, else allocates a bigger array (the capacity is Go's: its growth formula, element sizes and malloc size classes); the result may share the array with the argument")
	case "copy":
		g.why(c.Pos(), WhySlices, "copy: copies min(len(dst), len(src)) elements, handles overlap, no allocation")
	case "clear":
		g.why(c.Pos(), WhySlices, "clear: sets the elements to their zero values")
	}
}

// noteAddr records the address-of expression e.
func (fg *funcGen) noteAddr(e *ast.UnaryExpr) {
	g := fg.g
	if !g.noting() {
		return
	}
	switch x := ast.Unparen(e.X).(type) {
	case *ast.Ident:
		if v, ok := g.info.Uses[x].(*types.Var); ok && isPkgVar(v) {
			g.why(e.Pos(), WhyRefcount, "&"+x.Name+" of a package variable: gp::static_ptr, a pointer with no owner (the variable lives for the whole run)")
			return
		}
		g.why(e.Pos(), WhyRefcount, "&"+x.Name+" is the box of "+x.Name+" itself: the pointer shares its reference count, so the variable lives as long as any pointer to it")
	case *ast.CompositeLit:
		g.why(e.Pos(), WhyRefcount, "&T{...} allocates a new reference-counted box that holds the literal (gp::new_)")
	case *ast.StarExpr:
		g.why(e.Pos(), WhyChecks, "&*p is p, but it still checks p for nil, as *p does"+g.checkTail())
	default:
		g.why(e.Pos(), WhyRefcount, "&x.f or &s[i]: a pointer into an object shares the owner of the whole object (gp::ptr's aliasing constructor), so the object stays alive while the pointer does")
	}
}

// cxxIntKind maps the integer kinds that are one C++ type (int and int64 are int64_t, uint, uint64
// and uintptr are uint64_t) to one kind.
func cxxIntKind(k types.BasicKind) types.BasicKind {
	switch k {
	case types.Int:
		return types.Int64
	case types.Uint, types.Uintptr:
		return types.Uint64
	}
	return k
}

// noteConv records the conversion T(x) from type from to type to when it costs something.
func (g *gen) noteConv(pos token.Pos, to, from types.Type) {
	if !g.noting() || to == nil || from == nil {
		return
	}
	tb, fb := underBasic(to), underBasic(from)
	toStr := tb != nil && tb.Info()&types.IsString != 0
	fromStr := fb != nil && fb.Info()&types.IsString != 0
	switch {
	case toStr && isSlice(from), fromStr && isSlice(to):
		g.why(pos, WhyStrings, "converting between a string and a slice copies the bytes (or decodes and encodes the runes): it allocates, as in Go")
	case toStr && fb != nil && fb.Info()&types.IsInteger != 0:
		g.why(pos, WhyStrings, "string(integer) is the UTF-8 of the rune (gp::rune_to_string); a value that is not a code point gives \"\\uFFFD\"")
	case tb != nil && fb != nil && tb.Info()&types.IsInteger != 0 && fb.Info()&types.IsFloat != 0:
		g.why(pos, WhyChecks, "float to integer: gp::f2i truncates toward zero and gives amd64's result for NaN and values out of range, where C++ has undefined behavior")
	case tb != nil && fb != nil && tb.Info()&types.IsInteger != 0 && fb.Info()&types.IsInteger != 0 && cxxIntKind(tb.Kind()) != cxxIntKind(fb.Kind()):
		g.why(pos, WhyChecks, "integer conversion: a C++ functional cast, which wraps modulo 2^width like Go (-fwrapv)")
	}
}

// notePkgVar records how the package variable v starts: init is its initializer, or nil.
func (g *gen) notePkgVar(v *types.Var, init *types.Initializer) {
	if !g.noting() || v.Name() == "_" {
		return
	}
	switch {
	case !complete(v.Type()):
		g.why(v.Pos(), WhyPackages, "package variable "+v.Name()+" of a C/C++ type starts as its zero value (not constinit: a C++ class may have a constructor that is not constexpr); gp_var_init sets it in Go's order")
	case constInit(g.info, init) != nil:
		g.why(v.Pos(), WhyPackages, "package variable "+v.Name()+" is constinit: its initial value is a constant, so C++ has it at load time and no startup code runs")
	case nilInit(g.info, init) || init == nil:
		g.why(v.Pos(), WhyPackages, "package variable "+v.Name()+" is constinit: it starts as its zero value")
	case g.constLit(init) != nil:
		g.why(v.Pos(), WhyPackages, "package variable "+v.Name()+" is constinit: its initializer is a literal of constants, which C++ builds at compile time")
	default:
		g.why(v.Pos(), WhyPackages, "package variable "+v.Name()+" starts as its zero value (constinit) and gp_var_init assigns it at program start, in Go's initialization order (dependencies first)")
	}
}

// noteInitFunc records the init function that is called name.
func (g *gen) noteInitFunc(pos token.Pos, name string) {
	g.why(pos, WhyPackages, "func init is the C++ function "+name+" (a package can have several, and nothing can call them); gp_init_funcs calls them in source order after gp_var_init has set the package variables")
}

// noteLoopCopy records the new boxes of the variables vars of a for loop, whose variables are
// per iteration (Go 1.22); the loop's variables were boxed, so each iteration copies them.
func (g *gen) noteLoopCopy(vars []*types.Var, forPos token.Pos) {
	if !g.noting() {
		return
	}
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = v.Name()
	}
	g.why(forPos, WhyAlloc, "each iteration has its own "+strings.Join(names, ", ")+" (Go 1.22) and it is boxed (see its boxing note): before the post statement the loop copies it into a new box, one allocation per iteration; a loop variable that nothing captures stays a plain variable")
}

// noteOwnedResult records the call at pos whose result is a C/C++ value of a type that gocpp does
// not know, which the generic lambda of ownedValue owns.
func (g *gen) noteOwnedResult(pos token.Pos) {
	g.noteInterop(pos, "the result of a C/C++ call has a type that gocpp only learns from C++: the generic lambda [&]<typename gp_R>(std::type_identity<gp_R>) takes it as decltype(call), outside the lambda, and returns it as gp::values<gp_R> when it is a pair or tuple whose members must stay alive, or as it is; the call runs once, in one branch")
}

// noteName records that the declared name id is spelled differently in C++.
func (g *gen) noteName(id *ast.Ident) {
	if !g.noting() || id.Name == "_" {
		return
	}
	if c := g.escape(id.Name); c != id.Name {
		g.why(id.Pos(), WhyNames, id.Name+" is written "+c+" in C++: the name is reserved there (a keyword, a macro of the headers, or a name the generated code uses)")
	}
}

// noteCallValue records the call of a func value: stable says that nothing can change or free
// the func value while the call runs.
func (g *gen) noteCallValue(pos token.Pos, stable bool) {
	if !g.noting() {
		return
	}
	if stable {
		g.why(pos, WhyChecks, "call of a func value: gp::func's call operator checks for nil and panics like Go"+g.checkTail())
		return
	}
	g.why(pos, WhyChecks, "call of a func value that the callee could change (a boxed or shared variable): gp_call keeps a copy of it alive during the call; a nil func panics like Go"+g.checkTail())
}

// notePanic records a call of the builtin panic with an argument of type t.
func (g *gen) notePanic(pos token.Pos, t types.Type) {
	if !g.noting() {
		return
	}
	g.why(pos, WhyPanics, "panic: throws a C++ exception (gp::panic_error) that carries the value; deferred calls run while it unwinds, and recover can stop it; unrecovered, the runtime prints Go's message and trace and exits with status 2")
}

// noteRecover records the call of recover at pos.
func (g *gen) noteRecover(pos token.Pos) {
	if !g.noting() {
		return
	}
	g.why(pos, WhyRecover, "recover(): gp::recover(gp_dc) takes the pending panic with this function's ticket, only if a deferred call of a panicking function is the caller; else it returns nil")
}

// noteFmt records a call of the fmt function name.
func (g *gen) noteFmt(pos token.Pos, name string) {
	if !g.noting() || !g.opts.Why.first("fmt\x00"+g.fset.Position(pos).Filename+"\x00"+name+"\x00"+g.curInst) {
		return
	}
	g.why(pos, WhyInterfaces, "fmt."+name+": the runtime's gp::fmt functions take each operand with the descriptor of its Go type (gp::fmt::as), not boxed into an interface first (an operand of a non-pointer type with an Error method is, see the allocation note)")
}

// noteCCall records the call of a C/C++ function.
func (g *gen) noteCCall(pos token.Pos) {
	g.noteInterop(pos, "a call of a C/C++ function: the C++ compiler checks the arguments, C/C++ values keep C++'s rules (lifetime, no nil checks)")
}

// noteMemberCall records the call of a member of a C/C++ value or of a C++ callable.
func (g *gen) noteMemberCall(pos token.Pos) {
	g.noteInterop(pos, "a call of a member of a C/C++ value or a C++ callable: written as C++ writes it, the C++ compiler checks it")
}

// noteInterop records an interop decision.
func (g *gen) noteInterop(pos token.Pos, msg string) {
	g.why(pos, WhyInterop, msg)
}

// noteField records a promoted field: x is the selector, sel its selection.
func (g *gen) noteField(x *ast.SelectorExpr, sel *selection) {
	if !g.noting() {
		return
	}
	if sel.indirect {
		g.why(x.Sel.Pos(), WhyChecks, "p.f through a pointer: -> on a gp::ptr checks for nil and panics like Go"+g.checkTail())
	}
	if len(sel.index) < 2 {
		return
	}
	g.why(x.Sel.Pos(), WhyEmbedding, x.Sel.Name+" is promoted from an embedded field: the C++ names the path through the embedded fields, one member access for each step (-> after a pointer, which checks for nil)")
}

// noteInstanceUse records the use of the generic function r with its type arguments.
func (g *gen) noteInstanceUse(r *instRef, name string) {
	if !g.noting() {
		return
	}
	g.why(r.pos, WhyGenerics, fmt.Sprintf("%s is the C++ function %s for the type arguments %s: one C++ function for each list of type arguments (not a C++ template: int and int64 are one C++ type, but not one Go type)", r.gf.fn.Name(), name, g.typeArgsText(r.targs)))
}

// noteInstanceBody records the instance in that is lowered from its generic function's body.
func (g *gen) noteInstanceBody(in *instance) {
	if !g.noting() {
		return
	}
	g.why(in.gf.decl.Pos(), WhyGenerics, fmt.Sprintf("instance %s = %s[%s]: this body lowered again with the type parameters replaced by the type arguments", in.name, in.gf.fn.Name(), g.typeArgsText(in.targs)))
}

// typeArgsText returns the type arguments in Go's notation, separated by commas.
func (g *gen) typeArgsText(targs []types.Type) string {
	parts := make([]string, len(targs))
	for i, a := range targs {
		parts[i] = g.typeText(a)
	}
	return strings.Join(parts, ",")
}

// The shapes of the temporaries of the order pass (order.go), for noteOrder.
const (
	orderUnit  = iota // a call or another unit that gc evaluates first, in its place
	orderBlock        // x && y or x || y, whose right side has temporaries of its own
	orderPanic        // an operation that can panic, evaluated where gc evaluates it
)

// noteOrder records that the expression at pos is evaluated into the temporary t.
func (g *gen) noteOrder(pos token.Pos, t string, shape int) {
	if !g.noting() {
		return
	}
	switch shape {
	case orderBlock:
		g.why(pos, WhyOrder, t+" holds the left side of && or ||: the right side has calls of its own, which must run only when needed, so they sit in an if after it")
	case orderPanic:
		g.why(pos, WhyOrder, t+" holds a value that is read early: an operation that can panic must happen in Go's order, before the calls that come later in the statement")
	default:
		g.why(pos, WhyOrder, t+" holds the result of a call (or a value that a later call could change): Go evaluates it first, in order, where C++ leaves the order of the operands unspecified")
	}
}

// noteLend records how a call lends a pointer to the borrowed parameter or receiver of fn (borrow.go,
// borrowemit.go).
func (g *gen) noteLend(pos token.Pos, fn *types.Func, how lendHow) {
	if !g.noting() {
		return
	}
	name := fn.Name()
	switch how {
	case lendTemp:
		g.why(pos, WhyRefcount, "lent to "+name+" without a reference count: a temporary, or a variable that no call can change, keeps the object alive (gp::lend)")
	case lendBox:
		g.why(pos, WhyRefcount, "lent to "+name+" without a reference count: the box of a variable, which this function holds (gp::lend)")
	case lendRaw:
		g.why(pos, WhyRefcount, "lent to "+name+" as the raw address of a part of an object that stays alive during the call: no aliasing pointer, no reference count (gp::borrow)")
	case lendPure:
		g.why(pos, WhyRefcount, "lent to "+name+" without a copy: "+name+" drops no pointer while it runs (it is pure), so what the pointer was loaded from cannot lose the object")
	case lendCopy:
		g.why(pos, WhyRefcount, "copied for "+name+": the pointer is loaded from memory that the call could change, so the call holds its own reference (one count increment)")
	}
}

// noteBorrow records, for a function and each of its pointer parameters, whether the parameter
// is borrowed (a raw gp::bptr<T>) and if not, why.
func (g *gen) noteBorrow(f *funcInfo) {
	if !g.noting() || f.obj == nil {
		return
	}
	s := g.borrowOf(f.obj)
	if s == nil {
		return
	}
	sig := f.obj.Signature()
	one := func(v *types.Var, i int, isPtr bool) {
		if v == nil {
			return
		}
		name := v.Name()
		if name == "" || name == "_" {
			return
		}
		ref := ""
		if s.isRef(i) {
			ref = "; it is passed by const reference (a local that no call can change, or a temporary, is passed as it is: no copy, no count; anything else is copied first)"
		}
		if !isPtr {
			if ref != "" {
				g.why(v.Pos(), WhyRefcount, name+" is a const reference: the body never assigns it"+strings.TrimPrefix(ref, "; it is passed by const reference"))
			}
			return
		}
		if s.borrowed(i) {
			g.why(v.Pos(), WhyRefcount, name+" is borrowed: the body only reads and writes through it, compares it or lends it on, so the C++ parameter is a raw gp::bptr<T> and a call increments no reference count")
			return
		}
		why := s.why[i]
		if why == "" {
			why = s.notBorrowed
		}
		if why == "" {
			why = "it is used as a value"
		}
		g.why(v.Pos(), WhyRefcount, name+" keeps an owning gp::ptr<T> (copying it is one count increment): "+why+ref)
	}
	if r := sig.Recv(); r != nil {
		one(r, -1, g.borrowType(r.Type()))
	}
	for i := range sig.Params().Len() {
		p := sig.Params().At(i)
		one(p, i, g.borrowType(p.Type()))
	}
}
