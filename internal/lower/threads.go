package lower

import (
	"go/ast"
	"go/token"
	"go/types"
)

// The whole-program facts that decide whether a pointer loaded from memory may be lent to a pure
// callee without a copy (borrowemit.go, hotLoad; ADR-086 section 4).
//
// A pure callee cannot drop an owner by itself, so a pointer loaded from a package variable, a
// field or an element stays valid during the call unless ANOTHER THREAD replaces it meanwhile.
// Copying defensively preserves the call's owner but does NOT make a racing load/store safe: Go
// constrains single-word races, whereas racing gp::ptr copies can corrupt counts (D10). Two facts
// follow, both over the whole
// program (the packages are lowered together, ADR-025; a library build is open to foreign
// threads and has neither):
//
//  1. threaded: whether the program can run a second thread at all. It has a `go` statement or a
//     reference to a thread source (threadSources: time.AfterFunc, runtime.SetFinalizer,
//     sync.WaitGroup.Go, context.AfterFunc) in a package of the program itself, or reaches one in
//     the standard library from there (references to functions, the initializers). Unknown func,
//     interface and iterator calls, and native func/interface callbacks not explicitly reviewed
//     as synchronous, are potential thread sources too. Without a second thread nothing can
//     swap a pointer under a callee, and the lending is as before.
//  2. what a thread could swap: the package variables, struct fields and slice/map/array element
//     types that some statement of the program assigns (mutatedGlobal, mutatedField,
//     mutatedElem). An argument loaded through one of these is copied for the call.
//     Assignments in the bodies of init functions (not in their closures) do not count when no
//     init function of the program can start a thread (initThreads): they finish before main.
//
// Not covered, and said so in the ADR and D10: reflect and unsafe writes, an address that unknown
// code holds (`p := &x.f` and a store through p is a store to a field: it is found, but an alias
// created by a C library is not), foreign threads (cgo callbacks), and goroutines that a library
// the program links starts on its own.

// threadSources are the functions (go/types full names) that start a goroutine or run a function
// of the program on another thread without a go statement of their own in Go code. A function
// with a go statement in its body is a source by itself. TestThreadSources checks the list
// against the standard library and the runtime, so that a new native that starts a thread has to
// be listed or declared harmless.
var threadSources = map[string]bool{
	"time.AfterFunc":       true, // the timer thread calls f
	"time.startFunc":       true, // (the native under AfterFunc)
	"runtime.SetFinalizer": true, // the finalizer runs on its own thread
	"(*sync.WaitGroup).Go": true, // go f()
	"context.AfterFunc":    true, // go f()

	// These retain callbacks for existing Print/Scan/draining threads, rather than starting
	// workers. They are still conservative thread sources, not synchronous exemptions: a
	// callback can run on another thread while the registering/closing caller is active.
	"os.setFmtStdout":  true, // fmtstd.cpp: print_slow invokes the retained write callback
	"os.setFmtStdin":   true, // fmtscan.cpp: with_stdin invokes the retained reader callback
	"os.fmtDeferClose": true, // fmtstd.cpp: finish_close invokes it on the last draining thread
}

// synchronousNatives have reviewed callback behavior that cannot overlap a caller. Unknown
// native callbacks are thread sources by default; keep this list narrow (TestThreadSources).
var synchronousNatives = map[string]bool{
	"iter.newcoro":           true, // lock-step coroutine: only it or its caller runs at a time
	"runtime.setPanicValues": true, // invoked on the panicking thread
	"time.callOnce":          true, // synchronous zone-table initialization
	"time.startChan":         true, // its callback is the library's own channel send, not user code
}

func callbackSignature(sig *types.Signature) bool {
	seen := make(map[types.Type]bool)
	var callback func(types.Type) bool
	callback = func(ty types.Type) bool {
		if seen[ty] {
			return false
		}
		seen[ty] = true
		switch u := under(ty).(type) {
		case *types.Signature, *types.Interface, *types.TypeParam:
			return true
		case *types.Pointer:
			return callback(u.Elem())
		case *types.Slice:
			return callback(u.Elem()) // fmt's ...any can invoke String/Error/Format callbacks
		case *types.Array:
			return callback(u.Elem())
		case *types.Map:
			return callback(u.Key()) || callback(u.Elem())
		case *types.Chan:
			return callback(u.Elem())
		case *types.Struct:
			for i := range u.NumFields() {
				if callback(u.Field(i).Type()) {
					return true
				}
			}
		}
		return false
	}
	for i := range sig.Params().Len() {
		if callback(sig.Params().At(i).Type()) {
			return true
		}
	}
	return false
}

// threadInfo is the result for one program.
type threadInfo struct {
	library     bool // a library build: foreign code can call in and assign the exported variables
	threaded    bool // the program can run a second thread
	initThreads bool // an init function (or an initializer) can start one

	mutGlobals map[*types.Var]bool // the package variables that are assigned (outside exempt init code)
	mutFields  map[*types.Var]bool // the struct fields (originals) that are assigned
	mutElems   map[string]bool     // the types of the slice, map and array elements that are stored
}

// threads returns the thread facts of the program of g (computed once). A package lowered alone
// (the unit tests of this package) is a program of one package.
func (g *gen) threads() *threadInfo {
	if g.prog != nil {
		if g.prog.thr == nil {
			g.prog.thr = computeThreads(g.prog.gens, g.opts.Library)
		}
		return g.prog.thr
	}
	if g.thr == nil {
		g.thr = computeThreads([]*gen{g}, g.opts.Library)
	}
	return g.thr
}

type declRef struct {
	d *ast.FuncDecl
	g *gen
}

func computeThreads(gens []*gen, library bool) *threadInfo {
	t := &threadInfo{
		library:    library,
		mutGlobals: make(map[*types.Var]bool),
		mutFields:  make(map[*types.Var]bool),
		mutElems:   make(map[string]bool),
	}
	decls := make(map[*types.Func]declRef)
	for _, g := range gens {
		for _, f := range g.pkg.Load.Files {
			for _, d := range f.AST.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				fn, _ := g.info.Defs[fd.Name].(*types.Func)
				if fn == nil {
					continue
				}
				decls[fn] = declRef{fd, g}
			}
		}
	}
	// reach walks the code of the roots and everything it references in the program's functions and
	// reports whether it finds a thread source. Only the standard library is followed through
	// references for the program's own code: all of that is a root anyway.
	reach := func(roots func(visit func(g *gen, n ast.Node))) bool {
		seen := make(map[*types.Func]bool)
		var work []declRef
		found := false
		var scan func(g *gen, n ast.Node)
		scan = func(g *gen, n ast.Node) {
			ast.Inspect(n, func(n ast.Node) bool {
				if found {
					return false
				}
				switch x := n.(type) {
				case *ast.GoStmt:
					found = true
				case *ast.RangeStmt:
					if isRangeFunc(g.info, x) {
						found = true // an implicit call of an unknown iterator/callback
					}
				case *ast.CallExpr:
					if g.info.Types[x.Fun].IsType() {
						return true
					}
					if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok {
						if _, builtin := g.info.Uses[id].(*types.Builtin); builtin {
							return true
						}
					}
					if _, literal := ast.Unparen(x.Fun).(*ast.FuncLit); literal {
						return true // the body is visited below
					}
					// A call through a func value or an interface can invoke code that the
					// reference walk cannot resolve, especially during initialization. Do not
					// infer that it is synchronous from the absence of a named thread source.
					if g.calleeFunc(x) == nil {
						found = true
					}
				case *ast.Ident:
					fn, ok := g.info.Uses[x].(*types.Func)
					if !ok {
						return true
					}
					fn = fn.Origin()
					if threadSources[fn.FullName()] {
						found = true
						return false
					}
					if r := fn.Signature().Recv(); r != nil && types.IsInterface(r.Type()) {
						found = true
						return false
					}
					if dr, ok := decls[fn]; ok {
						if !seen[fn] {
							seen[fn] = true
							work = append(work, dr)
						}
					} else if !synchronousNatives[fn.FullName()] && callbackSignature(fn.Signature()) {
						// No body to inspect: a native may retain a callback or arrange for
						// another thread to invoke it. Interface arguments can hide callbacks too.
						found = true
					}
				}
				return true
			})
		}
		roots(scan)
		for len(work) > 0 && !found {
			dr := work[len(work)-1]
			work = work[:len(work)-1]
			scan(dr.g, dr.d.Body)
		}
		return found
	}
	if library {
		t.threaded, t.initThreads = true, true
	} else {
		t.threaded = reach(func(visit func(*gen, ast.Node)) {
			for _, g := range gens {
				for _, f := range g.pkg.Load.Files {
					for _, d := range f.AST.Decls {
						fd, isFunc := d.(*ast.FuncDecl)
						switch {
						case !g.pkg.Load.Std:
							visit(g, d) // all the code of the program's own packages
						case isFunc && fd.Recv == nil && fd.Name.Name == "init":
							visit(g, d)
						case !isFunc:
							visit(g, d) // the initializers of the standard packages' variables
						}
					}
				}
			}
		})
		t.initThreads = t.threaded && reach(func(visit func(*gen, ast.Node)) {
			for _, g := range gens {
				for _, f := range g.pkg.Load.Files {
					for _, d := range f.AST.Decls {
						fd, isFunc := d.(*ast.FuncDecl)
						if !isFunc || fd.Recv == nil && fd.Name.Name == "init" {
							visit(g, d)
						}
					}
				}
			}
		})
	}
	if t.threaded {
		for _, g := range gens {
			for _, f := range g.pkg.Load.Files {
				for _, d := range f.AST.Decls {
					fd, isFunc := d.(*ast.FuncDecl)
					exempt := isFunc && fd.Recv == nil && fd.Name.Name == "init" && !t.initThreads && !library
					if !isFunc && !isVarDecl(d) {
						continue
					}
					t.markMutated(g, d, exempt)
				}
			}
		}
	}
	return t
}

func isVarDecl(d ast.Decl) bool {
	gd, ok := d.(*ast.GenDecl)
	return ok && gd.Tok == token.VAR
}

// elemKey names the element type of a slice, map or array for mutElems.
func elemKey(t types.Type) string { return types.TypeString(t, nil) }

// markMutated records what the code n of g assigns: package variables, struct fields and element
// types, whole or in part, by an assignment, ++, range with =, & (an alias that a store may use),
// slicing of an array variable, copy/clear/append/delete of elements, or a call of a pointer method on an
// addressable value (its receiver is &x). With exempt, the assignments that are not in a
// function literal are not recorded (an init function that starts no thread); the address-taking
// forms always are.
func (t *threadInfo) markMutated(g *gen, n ast.Node, exempt bool) {
	var mark func(e ast.Expr)
	markStruct := func(ty types.Type) {
		var walk func(ty types.Type)
		walk = func(ty types.Type) {
			switch u := under(ty).(type) {
			case *types.Struct:
				for i := range u.NumFields() {
					t.mutFields[u.Field(i).Origin()] = true
					walk(u.Field(i).Type())
				}
			case *types.Array:
				t.mutElems[elemKey(u.Elem())] = true
				walk(u.Elem())
			}
		}
		// Only inline storage is followed, never pointers: legal types cannot have a cycle
		// here. A depth limit would miss a field replaced by a deeply nested whole-value store.
		walk(ty)
	}
	mark = func(e ast.Expr) {
		if ty := g.info.TypeOf(e); ty != nil {
			markStruct(ty) // a struct stored as a whole changes every field of it
		}
		for {
			e = ast.Unparen(e)
			switch x := e.(type) {
			case *ast.Ident:
				if v, ok := g.info.Uses[x].(*types.Var); ok && isPkgVar(v) {
					t.mutGlobals[v] = true
				} else if v, ok := g.info.Defs[x].(*types.Var); ok && isPkgVar(v) {
					t.mutGlobals[v] = true
				}
				return
			case *ast.SelectorExpr:
				if v, ok := g.info.Uses[x.Sel].(*types.Var); ok && isPkgVar(v) {
					t.mutGlobals[v] = true // pkg.V
					return
				}
				if sel := g.info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
					if f, ok := sel.Obj().(*types.Var); ok {
						t.mutFields[f.Origin()] = true
					}
				}
				if isPointer(g.info.TypeOf(x.X)) {
					return // a field of the object, not of the variable
				}
				e = x.X
			case *ast.IndexExpr:
				xt := g.info.TypeOf(x.X)
				switch u := under(xt).(type) {
				case *types.Array:
					t.mutElems[elemKey(u.Elem())] = true
					e = x.X
					continue
				case *types.Slice:
					t.mutElems[elemKey(u.Elem())] = true
				case *types.Map:
					t.mutElems[elemKey(u.Elem())] = true
				case *types.Pointer:
					if a, ok := under(u.Elem()).(*types.Array); ok {
						t.mutElems[elemKey(a.Elem())] = true
					}
				}
				return
			case *ast.StarExpr:
				return // (a struct stored through the pointer: markStruct marked its fields)
			default:
				return
			}
		}
	}
	var visit func(n ast.Node, inLit bool)
	visit = func(n ast.Node, inLit bool) {
		ast.Inspect(n, func(n ast.Node) bool {
			skip := exempt && !inLit
			switch x := n.(type) {
			case *ast.FuncLit:
				visit(x.Body, true)
				return false
			case *ast.AssignStmt:
				if x.Tok != token.DEFINE && !skip {
					for _, l := range x.Lhs {
						mark(l)
					}
				}
			case *ast.IncDecStmt:
				if !skip {
					mark(x.X)
				}
			case *ast.RangeStmt:
				if x.Tok == token.ASSIGN && !skip {
					if x.Key != nil {
						mark(x.Key)
					}
					if x.Value != nil {
						mark(x.Value)
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					mark(x.X)
				}
			case *ast.SliceExpr:
				if isArray(g.info.TypeOf(x.X)) {
					mark(x.X) // a slice of an array variable aliases it
				}
			case *ast.CallExpr:
				if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok && (id.Name == "copy" || id.Name == "clear" || id.Name == "append" || id.Name == "delete") && len(x.Args) > 0 {
					if _, builtin := g.info.Uses[id].(*types.Builtin); builtin && !skip {
						if s, ok := under(g.info.TypeOf(x.Args[0])).(*types.Slice); ok {
							t.mutElems[elemKey(s.Elem())] = true
						} else if m, ok := under(g.info.TypeOf(x.Args[0])).(*types.Map); ok {
							t.mutElems[elemKey(m.Elem())] = true
						}
					}
				}
			case *ast.SelectorExpr:
				// x.M() of a pointer method on an addressable value takes its address.
				if sel := g.info.Selections[x]; sel != nil && sel.Kind() == types.MethodVal && !isPointer(g.info.TypeOf(x.X)) {
					if fn, ok := sel.Obj().(*types.Func); ok {
						if r := fn.Signature().Recv(); r != nil && isPointer(r.Type()) {
							mark(x.X)
						}
					}
				}
			}
			return true
		})
	}
	visit(n, false)
}

// mutatedGlobal reports whether the program assigns the package variable v (see the top).
func (g *gen) mutatedGlobal(v *types.Var) bool {
	t := g.threads()
	return t.mutGlobals[v] || t.library && v.Exported()
}

// hotFields includes every implicit embedded field in a selection, not just its last field.
// In a library, foreign callers can also replace fields/elements without any visible Go store.
func (t *threadInfo) hotFields(ty types.Type, index []int) bool {
	if !t.threaded {
		return false
	}
	for _, i := range index {
		if p, ok := under(ty).(*types.Pointer); ok {
			ty = p.Elem()
		}
		st, ok := under(ty).(*types.Struct)
		if !ok {
			return true // an unresolved path must not establish a no-replacement proof
		}
		f := st.Field(i)
		if t.library || t.mutFields[f.Origin()] {
			return true
		}
		ty = f.Type()
	}
	return false
}

func (fg *funcGen) hotRecv(x *ast.SelectorExpr, r recvPath) bool {
	return fg.hotLoad(x.X) || fg.g.threads().hotFields(r.t, r.path)
}

// hotLoad reports whether the value of e is loaded through something that another thread may
// replace while a callee runs, in a program that can have another thread: a package variable, a
// struct field or a slice, map or array element that the program assigns, anywhere on the load
// path (including a heap holder reached from a local). Lending that pointer raw to a pure callee would let the swap free the
// object under the callee, so such an argument is copied for the call, like an argument of an
// impure callee (ADR-086, section 4).
func (fg *funcGen) hotLoad(e ast.Expr) bool {
	return fg.g.hotLoad(e)
}

// hotLoad is funcGen.hotLoad for the analyses that run before a function is lowered.
func (g *gen) hotLoad(e ast.Expr) bool {
	t := g.threads()
	if !t.threaded {
		return false
	}
	for {
		e = ast.Unparen(e)
		switch x := e.(type) {
		case *ast.Ident:
			v, ok := g.info.Uses[x].(*types.Var)
			return ok && isPkgVar(v) && g.mutatedGlobal(v)
		case *ast.SelectorExpr:
			if v, ok := g.info.Uses[x.Sel].(*types.Var); ok && isPkgVar(v) {
				return g.mutatedGlobal(v) // pkg.V
			}
			sel := g.info.Selections[x]
			if sel != nil && sel.Kind() == types.FieldVal && t.hotFields(sel.Recv(), sel.Index()) {
				return true
			}
			e = x.X
		case *ast.IndexExpr:
			switch u := under(g.info.TypeOf(x.X)).(type) {
			case *types.Slice:
				if t.library || t.mutElems[elemKey(u.Elem())] {
					return true
				}
			case *types.Map:
				if t.library || t.mutElems[elemKey(u.Elem())] {
					return true
				}
			case *types.Array:
				if t.library || t.mutElems[elemKey(u.Elem())] {
					return true
				}
			case *types.Pointer:
				if a, ok := under(u.Elem()).(*types.Array); ok && (t.library || t.mutElems[elemKey(a.Elem())]) {
					return true
				}
			}
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.TypeAssertExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return false
			}
			e = x.X
		case *ast.CallExpr:
			if g.info.Types[x.Fun].IsType() && len(x.Args) == 1 {
				e = x.Args[0] // a conversion
				continue
			}
			return false
		default:
			return false
		}
	}
}
