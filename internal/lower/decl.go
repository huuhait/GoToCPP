package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/load"
)

// Declarations: functions, package variables and the package's initialization.

// A funcInfo is a function of the package, with what prepare chose for it, or a func literal
// (closures.go), with what its lambda's signature chose.
type funcInfo struct {
	decl *ast.FuncDecl // nil for a func literal
	obj  *types.Func   // nil for a func literal
	name string        // the C++ name (of a function of the package)

	sig   *types.Signature
	ftype *ast.FuncType // the function's type in the source (the result variables of the declaration)
	body  *ast.BlockStmt
	pos   token.Pos // the func keyword
	// top is the outermost node that the function is in: its own declaration, or for a func
	// literal the declaration or the package variable's initializer that holds it (panicid.go).
	top ast.Node

	names   *funcNames
	result  cppir.Type    // the C++ result type: void, int64_t, std::tuple<int64_t, bool>
	params  []cppir.Param // the C++ parameters
	results []string      // the C++ names of the named results, or nil
	// aliases are the names of the results that a variable of an inner scope hides at a return
	// statement with values: gp_res_<name>, a reference to the result that the return assigns
	// through (defer.go: resultAliases), or "" (nil: none).
	aliases []string

	boxed map[*types.Var]boxReason // the variables that live in a box (box.go)
	args  []*types.Var             // the boxed parameters, in order
	esc   *escapeInfo              // what the escape analysis kept on the stack (escape.go), or nil
	addr  *addrAnalysis            // the address analysis of top (cbox.go), shared with the literals and bodies in it

	defers *deferInfo // the single-exit shape of a function with defers (defer.go), or nil

	method *methodInfo // a method's receiver and its type (methods.go), or nil for a function
}

// prepare chooses the C++ names of the function's parameters and named results, and its C++
// signature. Parameters are passed by value, as real copies, like Go's (one larger than 128 KiB
// as a const reference, which the function copies into a box at its entry). Several results are
// a std::tuple; named results are local variables that start at their zero value. It also
// runs the boxing analysis (box.go): a boxed parameter keeps its name as a C++ parameter, and
// funcDecl copies it into its box, a variable of the same name in a block (ADR-022's shape).
func (f *funcInfo) prepare(g *gen) {
	f.sig, f.body, f.pos, f.top = f.obj.Signature(), f.decl.Body, f.decl.Pos(), f.decl
	f.ftype = f.decl.Type
	f.boxed = maps.Clone(g.boxedOf(f.decl)) // (prepare adds to it)
	f.esc = g.escapeOf(f.decl)
	for v := range f.esc.stack {
		f.boxed[v] = boxStack // (escape.go: the variable stays on the stack)
	}
	f.names = g.newFuncNames()
	// A new name must not hide anything that the function refers to (funcNames).
	ast.Inspect(f.decl, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			obj := g.info.Defs[id]
			if obj == nil {
				obj = g.info.Uses[id]
			} else {
				g.noteName(id) // why.go
			}
			switch obj := obj.(type) {
			case *types.Var:
				if !obj.IsField() { // a field is always reached by x.f: it hides nothing
					f.names.reserve(g.escape(obj.Name()))
				}
			case *types.Func:
				f.names.reserve(g.escape(obj.Name()))
			}
		}
		return true
	})
	// The function's scope holds the parameters, the named results and the outermost
	// declarations of the body, in C++ as in Go. funcDecl closes it.
	f.names.openScope()
	// A parameter named like a type hides it in the parameters after it (typedecl.go), and so
	// does a member of a method's struct (methods.go).
	defer g.hide(f.names.taken)()
	recv := f.prepareRecv(g) // methods.go: the receiver of a pointer method is a parameter
	defer g.hideMembers(f.members())()
	f.signature(g)
	f.params = append(recv, f.params...)
}

// signature chooses the C++ names of the parameters and named results of the function (or func
// literal) f, in the innermost scope of f.names, and its C++ parameters and result type (prepare).
func (f *funcInfo) signature(g *gen) {
	sig := f.sig
	var macro *load.MacroSig
	if f.obj != nil {
		macro = g.macroSig(f.obj) // a function declaration only: a func literal is never a macro
	}
	f.params = make([]cppir.Param, sig.Params().Len())
	var bsum *borrowSum // the parameters that the function borrows (borrow.go)
	if f.obj != nil {
		bsum = g.borrowOf(f.obj)
	}
	for i := range sig.Params().Len() {
		p := sig.Params().At(i)
		f.params[i].Type = g.varType(p, f.pos)
		if viewIndex(macro, i) {
			f.params[i].Type = typeName("gp::static_view") // generics.go: viewIndex
			if g.viewVars == nil {
				g.viewVars = make(map[*types.Var]bool)
			}
			g.viewVars[p] = true
		} else if bsum.borrowed(i) {
			f.params[i].Type = borrowedType(f.params[i].Type)
		} else if bsum.isRef(i) {
			f.params[i].Type = &cppir.Ref{Elem: &cppir.Const{Elem: f.params[i].Type}} // borrow.go
		}
		if largeVar(p.Type()) {
			// A parameter larger than 128 KiB: a copy on the C++ stack could overflow it
			// (D24), so the caller passes a reference, and the function copies the value
			// into a box at its entry (paramBoxes), before any of its code runs: the
			// parameter is still a copy of the argument, as in Go.
			f.params[i].Type = &cppir.Ref{Elem: &cppir.Const{Elem: f.params[i].Type}}
			if f.boxed[p] == 0 && p.Name() != "" && p.Name() != "_" {
				f.boxed[p] = boxLarge
			}
		}
		if p.Name() != "" && p.Name() != "_" {
			f.params[i].Name = f.names.declare(p)
			if r := f.boxed[p]; r != 0 && r != boxStack {
				f.args = append(f.args, p) // (a parameter that stays on the stack is the C++ one)
			}
		}
	}
	res := sig.Results()
	if res.Len() > 0 && res.At(0).Name() != "" {
		f.results = make([]string, res.Len())
		for i := range res.Len() {
			r := res.At(i)
			if r.Name() == "_" {
				f.results[i] = f.names.temp("r") // still returned by a bare return
			} else {
				f.results[i] = f.names.declare(r)
			}
		}
	}
	f.result = g.resultType(sig, f.pos)
	f.prepareRangeFuncResults(g) // rangefunc.go
	f.prepareDefers(g)
}

// funcDecl adds the definition of a function.
func (g *gen) funcDecl(f *funcInfo) {
	fn := g.funcBuild(f)
	g.addFunc(f, fn)
}

// funcBuild returns the definition of a function.
func (g *gen) funcBuild(f *funcInfo) *cppir.Func {
	defer g.hide(f.names.taken)() // a variable named like a type hides it (typedecl.go)
	defer g.hideMembers(f.members())()
	body := f.body
	g.noteConversions(body, f.sig) // named.go
	g.markCounts(body)
	g.noteBoxed(f.boxed)                             // why.go
	g.noteBorrow(f)                                  // whynotes.go
	g.noteEscape(f)                                  // escape.go
	g.numberLits(f.decl, make(map[*ast.FuncLit]int)) // closures.go
	fg := newFuncGen(g, f.names, f)
	fn := &cppir.Func{Pos: g.pos(f.pos), Result: f.result, Name: f.name, Params: f.params, Quals: f.quals(), Body: &cppir.Block{}}
	if hasDirective(f.decl, "//go:noinline") {
		// Go's directive: the function is never inlined, so its frame and its name are its own
		// (runtime.Caller, FuncForPC and the trace of an optimized build: gocpp/rt.hpp).
		fn.Specs = append(fn.Specs, "GP_NOINLINE")
	}
	if g.isRecursive(f.decl) {
		// A recursion without end overflows the stack as in Go (recursion.go).
		fn.Specs = append(fn.Specs, "GP_RECURSIVE")
		fn.Body.Stmts = append(fn.Body.Stmts, &cppir.ExprStmt{X: call("GP_RECURSION_MARK")})
	}
	fg.push(&fn.Body.Stmts, &fn.Body.End)
	fg.funcBody()
	fg.pop(body.Rbrace)
	f.names.closeScope()
	return fn
}

// funcBody lowers the body of the function (or func literal) fg.fn into the open statement
// list, the C++ function's block or the lambda's: the named results, the single-exit shape of a
// function with defers (defer.go), the boxes of the parameters, and the statements.
func (fg *funcGen) funcBody() {
	g, f := fg.g, fg.fn
	body := f.body
	fg.flow = newFlow(g.info, body, fg.postAtEnd)
	fg.takeTicket() // recover.go: an aware function takes its ticket first
	if f.results != nil {
		res := f.sig.Results()
		for i := range res.Len() {
			r := res.At(i)
			typ := g.varType(r, r.Pos())
			switch {
			case f.defers != nil && fg.isSlot(r):
				// A result of a C/C++ type that no name can set: gp::result_slot<T> (result.hpp).
				fg.add(varDecl(g.pos(r.Pos()), &cppir.TypeName{Name: "gp::result_slot", Args: []cppir.Node{typ}}, f.results[i], nil))
			case f.boxed[r] != 0 && cObject(r.Type()):
				fg.add(cBoxDecl(g.pos(r.Pos()), typ, f.results[i], nil)) // cbox.go
			case f.boxed[r] != 0:
				fg.add(varDecl(g.pos(r.Pos()), ptrType(typ), f.results[i], newBox(typ, nil)))
			default:
				fg.add(varDecl(g.pos(r.Pos()), typ, f.results[i], &cppir.InitList{}))
			}
		}
	}
	fg.declareAliases()         // defer.go
	fg.fresh = f.results == nil // the named results are in the function's block (declLines)
	fg.recvCopy()               // methods.go: a value receiver's copy, T v = *this;
	fg.openDefers()             // defer.go
	boxes := fg.paramBoxes()
	fg.stmts(body.List, body.Rbrace)
	if boxes {
		fg.names.closeScope()
		fg.pop(body.Rbrace)
	}
	fg.closeDefers(body.Rbrace)
}

// paramBoxes copies the boxed parameters into their boxes, which keep the parameters' names, in
// a block that holds the body, as a shadowing declaration does (ADR-022, declLines):
//
//	int64_t twice(int64_t n) {
//	    gp::ptr<int64_t> gp_t0 = gp::new_<int64_t>(n); { gp::ptr<int64_t> n = std::move(gp_t0);
//	        ...the body...
//	    }
//	}
//
// (std::move is written as a cast: moved.) So a debugger shows the parameter by its name in the
// frame line (twice (n=21), the value it came with, like Go's tracebacks) and the box by the
// same name in the body (print *n). A class type moves the parameter into the box. It reports
// whether it opened the block.
func (fg *funcGen) paramBoxes() bool {
	f := fg.fn
	if len(f.args) == 0 {
		return false
	}
	g := fg.g
	p := g.pos(f.pos)
	line := &cppir.Line{Pos: p}
	sc := &cppir.Scope{Pos: p}
	for _, v := range f.args {
		name := fg.names.names[v]
		typ := g.varType(v, v.Pos())
		var arg cppir.Expr = cxxName(name)
		if isClass(v.Type()) && !largeVar(v.Type()) {
			arg = moved(typ, arg) // a large parameter is a const reference (prepare): copied
		}
		tmp := fg.names.temp("t")
		box := ptrType(typ)
		if cObject(v.Type()) {
			// A C/C++ object: the parameter is a copy already, which moves into the box (cbox.go).
			line.Stmts = append(line.Stmts, cBoxDecl(cppir.Pos{}, typ, tmp, movedName(name)))
			box = boxType(typ)
		} else {
			line.Stmts = append(line.Stmts, varDecl(cppir.Pos{}, box, tmp, newBox(typ, arg)))
		}
		boxed := moved(box, cxxName(tmp))
		if isAutoType(box) {
			boxed = movedName(tmp) // a yield parameter whose C++ type is deduced (rangefunc.go)
		}
		sc.Head = append(sc.Head, varDecl(cppir.Pos{}, box, name, boxed))
		if h := fg.holdBox(v, name); h != nil {
			sc.Head = append(sc.Head, h) // the frame holds the box of a parameter whose address is taken
		}
	}
	line.Stmts = append(line.Stmts, sc)
	fg.add(line)
	fg.push(&sc.Stmts, &sc.End)
	fg.names.openScope()
	for _, v := range f.args {
		fg.names.enter(fg.names.names[v])
	}
	return true
}

// moved returns static_cast<typ&&>(x): std::move(x) for x of the C++ type typ. g++ -O0 folds
// a call of std::move but leaves a row of <bits/move.h> in the function's line table, which
// TestDebug rejects; the cast has no code of its own.
func moved(typ cppir.Type, x cppir.Expr) cppir.Expr {
	return &cppir.Cast{Kind: "static_cast", Type: &cppir.Ref{Elem: typ, RValue: true}, X: x}
}

// movedName is moved for the variable name, whose C++ type gocpp doesn't name (auto, a C/C++
// value): static_cast<decltype(name)&&>(name).
func movedName(name string) cppir.Expr {
	return moved(typeName("decltype("+name+")"), cxxName(name))
}

// packageVars adds the package variables. Each one is constinit, so C++ runs no dynamic
// initialization of its own (whose order across files C++ doesn't define): a variable whose
// initializer is a constant gets its value at compile time, one whose initializer is nil (var
// s []int = nil) is its zero value, and the others start at their zero value and get their
// values in gp_var_init, in Go's order (types.Info.InitOrder). A slice literal needs its
// array on the heap, so it is never constinit; an array or a struct literal of number and bool
// constants is (constLit, pkgvars.go).
func (g *gen) packageVars() {
	byVar := make(map[*types.Var]*types.Initializer)
	for _, init := range g.info.InitOrder {
		for _, v := range init.Lhs {
			byVar[v] = init
		}
	}
	for _, v := range g.vars {
		g.notePkgVar(v, byVar[v]) // why.go
		if cmpHasC(v.Type()) {
			// A C/C++ class by value (also inside a struct or an array of ours): its constructor
			// may not be constexpr, so constinit is not possible: gp::lazy storage (lazyVar).
			g.lazyVar(v)
			continue
		}
		if !complete(v.Type()) {
			// A C/C++ type (ctypes.go): its zero value, and the value in gp_var_init. Not
			// constinit: a C++ class may have a constructor that isn't constexpr.
			g.addVar(varDecl(g.pos(v.Pos()), g.varType(v, v.Pos()), g.names[v], &cppir.InitList{}))
			continue
		}
		typ := g.cxxType(v.Type(), v.Pos())
		var value cppir.Expr = &cppir.InitList{} // the zero value
		if c := constInit(g.info, byVar[v]); c != nil {
			if underBasic(v.Type()).Info()&types.IsString != 0 {
				// "..."_gs can't be constexpr (the owner's destructor isn't), but this
				// constructor is.
				s := constant.StringVal(c)
				value = &cppir.InitList{Elems: []cppir.Expr{cxxName("gp::static_bytes"), lit(cppir.CString(s)), lit(strconv.Itoa(len(s)))}}
			} else {
				value = g.constant(c, v.Type(), v.Pos(), false)
			}
		} else if x := g.constLit(byVar[v]); x != nil {
			value = x // an array or a struct literal of constants (pkgvars.go)
		}
		d := varDecl(g.pos(v.Pos()), typ, g.names[v], value)
		d.Specs = []string{"constinit"}
		g.addVar(d)
	}
	for _, init := range g.info.InitOrder {
		if constInit(g.info, init) == nil && !nilInit(g.info, init) && g.constLit(init) == nil {
			g.varInits = true
		}
	}
	if len(g.vars) > 0 {
		g.out.vars = append(g.out.vars, &cppir.Blank{})
		g.out.varExt = append(g.out.varExt, &cppir.Blank{})
	}
}

// addVar adds the definition d of a package variable. A package of several translation units
// declares it extern in its header (split layout); the definition is in pkg_init.cpp.
func (g *gen) addVar(d *cppir.VarDecl) {
	g.out.vars = append(g.out.vars, d)
	g.out.varExt = append(g.out.varExt, &cppir.VarDecl{Specs: []string{"extern"}, Type: d.Type, Vars: []cppir.Declarator{{Name: d.Vars[0].Name}}})
}

// addFunc adds the definition fn of the function (or method) f to the file of its declaration.
func (g *gen) addFunc(f *funcInfo, fn *cppir.Func) {
	// (The name the file was added with, not Position's: a //line directive changes that.)
	file := g.fileOf(f.decl.Pos())
	if g.out.byFile == nil {
		g.out.byFile = make(map[string][]cppir.Decl)
	}
	if _, ok := g.out.byFile[file]; !ok {
		g.out.files = append(g.out.files, file)
	}
	g.out.byFile[file] = append(g.out.byFile[file], fn, &cppir.Blank{})
	g.out.funcs = append(g.out.funcs, fn, &cppir.Blank{})
}

// constInit returns the constant value of the initializer init if it gives one variable a
// constant, or nil.
func constInit(info *tinfo, init *types.Initializer) constant.Value {
	if init == nil || len(init.Lhs) != 1 || init.Lhs[0].Name() == "_" {
		return nil
	}
	if underBasic(init.Lhs[0].Type()) == nil {
		return nil // (a named type over a basic type is constant too: Celsius(1.5))
	}
	return info.Types[init.Rhs].Value
}

// nilInit reports whether the initializer init gives one variable nil, its zero value.
func nilInit(info *tinfo, init *types.Initializer) bool {
	return init != nil && len(init.Lhs) == 1 && info.Types[init.Rhs].IsNil()
}

// initFuncs adds gp_var_init, which sets the package variables that constInit doesn't, and
// gp_init_funcs, which calls the init functions in source order (gp_init_all, after the
// namespace, calls both; docs/spec/cpp-mapping.md, "Package initialization"). They are
// generated code: their own lines have no place in the .gp source, but the statements of
// gp_var_init are mapped to the initializers.
func (g *gen) initFuncs() {
	if g.varInits {
		fg := newFuncGen(g, g.newFuncNames(), nil)
		g.prepareVarInits(fg)
		defer g.hide(fg.names.taken)() // a closure's variable named like a type hides it
		fg.names.openScope()
		fn := &cppir.Func{Result: typeName("void"), Name: "gp_var_init", Body: &cppir.Block{}}
		fg.push(&fn.Body.Stmts, nil)
		for _, store := range g.lazyVars { // (lazyVar, pkgvars.go) the zero values, before any initializer reads one
			fg.add(&cppir.ExprStmt{X: &cppir.Call{Fun: &cppir.Member{X: cxxName(store), Name: "gp_ensure"}}})
		}
		for _, init := range g.info.InitOrder {
			if constInit(g.info, init) != nil || nilInit(g.info, init) || g.constLit(init) != nil {
				continue
			}
			fg.top = init.Rhs
			g.markCounts(init.Rhs)
			if len(init.Lhs) == 1 {
				g.noteConversion(init.Rhs, init.Lhs[0].Type()) // named.go
			}
			g.noteConversions(init.Rhs, nil)
			if fg.largeInit(init) { // built in the variable (pkgvars.go)
				continue
			}
			targets := make([]target, len(init.Lhs))
			for i, v := range init.Lhs {
				if v.Name() != "_" {
					targets[i] = varTarget(g.names[v])
					targets[i].gtype = v.Type()
				}
			}
			fg.bind(init.Rhs.Pos(), targets, nil, []ast.Expr{init.Rhs}, false)
		}
		g.out.inits = append(g.out.inits, fn, &cppir.Blank{})
	}
	if len(g.initFns) > 0 {
		body := &cppir.Block{}
		for _, n := range g.initFns {
			body.Stmts = append(body.Stmts, &cppir.ExprStmt{X: call(n)})
		}
		g.out.inits = append(g.out.inits, &cppir.Func{Result: typeName("void"), Name: "gp_init_funcs", Body: body}, &cppir.Blank{})
	}
}

// prepareVarInits prepares gp_var_init's funcGen for the func literals in the initializers of
// the package variables (step 3.11): their variables are boxed by the same analysis as a
// function's (box.go), their names hide nothing they refer to (prepare), and they are numbered
// in source order across the package, as Go's main.init.funcN (closures.go).
func (g *gen) prepareVarInits(fg *funcGen) {
	fg.boxed = make(map[*types.Var]boxReason)
	counts := make(map[*ast.FuncLit]int)
	for _, f := range g.pkg.Load.Files {
		for _, d := range f.AST.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				for _, v := range spec.(*ast.ValueSpec).Values {
					g.numberLits(v, counts)
					for x, r := range boxedVars(g.info, v) {
						fg.boxed[x] = r
					}
					g.noteBoxed(fg.boxed) // why.go
					ast.Inspect(v, func(n ast.Node) bool {
						if id, ok := n.(*ast.Ident); ok {
							switch obj := g.info.Uses[id].(type) {
							case *types.Var:
								if !obj.IsField() {
									fg.names.reserve(g.escape(obj.Name()))
								}
							case *types.Func:
								fg.names.reserve(g.escape(obj.Name()))
							}
							if obj, ok := g.info.Defs[id].(*types.Var); ok {
								fg.names.reserve(g.escape(obj.Name()))
							}
						}
						return true
					})
				}
			}
		}
	}
}

// newBox returns gp::new_<typ>(value), a new box (or new(T)) that holds value, or the zero
// value if value is nil.
func newBox(typ cppir.Type, value cppir.Expr) cppir.Expr {
	fun := &cppir.Name{Name: "gp::new_", Args: []cppir.Node{typ}}
	if value == nil {
		return &cppir.Call{Fun: fun}
	}
	return &cppir.Call{Fun: fun, Args: []cppir.Expr{value}}
}

// localNew returns gp::local_new<typ>(value): a gp::local_ptr<typ> that holds a new typ made from
// value (or the zero value if value is nil) in the variable itself, on the stack (escape.go).
func localNew(typ cppir.Type, value cppir.Expr) cppir.Expr {
	fun := &cppir.Name{Name: "gp::local_new", Args: []cppir.Node{typ}}
	if value == nil {
		return &cppir.Call{Fun: fun}
	}
	return &cppir.Call{Fun: fun, Args: []cppir.Expr{value}}
}

// localPtrType returns gp::local_ptr<typ>, the type of a pointer variable that holds its object.
func localPtrType(typ cppir.Type) cppir.Type {
	return &cppir.TypeName{Name: "gp::local_ptr", Args: []cppir.Node{typ}}
}

// isClass reports whether the C++ type of t is a class type (gp::string, gp::ptr), which a
// value that is not used again is moved into, rather than one of C++'s fundamental types (or a
// named type over one, a gp::newtype that holds only the number).
func isClass(t types.Type) bool {
	b := underBasic(t)
	return b == nil || b.Info()&types.IsString != 0
}

// hasDirective reports whether the doc comment of the function decl has the directive line text
// (//go:noinline), as Go reads it: a line of its own in the comment group just before the func.
func hasDirective(decl *ast.FuncDecl, text string) bool {
	if decl == nil || decl.Doc == nil {
		return false
	}
	for _, c := range decl.Doc.List {
		if c.Text == text || strings.HasPrefix(c.Text, text+" ") {
			return true
		}
	}
	return false
}
