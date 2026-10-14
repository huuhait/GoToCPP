package lower

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Declarations: functions, package variables and the package's initialization.

// A funcInfo is a function of the package, with what prepare chose for it.
type funcInfo struct {
	decl *ast.FuncDecl
	obj  *types.Func
	name string // the C++ name

	names   *funcNames
	result  cppir.Type    // the C++ result type: void, int64_t, std::tuple<int64_t, bool>
	params  []cppir.Param // the C++ parameters
	results []string      // the C++ names of the named results, or nil
}

// prepare chooses the C++ names of the function's parameters and named results, and its C++
// signature. Parameters are passed by value, as real copies, like Go's. Several results are
// a std::tuple; named results are local variables that start at their zero value.
func (f *funcInfo) prepare(g *gen) {
	f.names = newFuncNames()
	// A new name must not hide anything that the function refers to (funcNames).
	ast.Inspect(f.decl, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			obj := g.info.Defs[id]
			if obj == nil {
				obj = g.info.Uses[id]
			}
			switch obj.(type) {
			case *types.Var, *types.Func:
				f.names.reserve(Escape(obj.Name()))
			}
		}
		return true
	})
	// The function's scope holds the parameters, the named results and the outermost
	// declarations of the body, in C++ as in Go. funcDecl closes it.
	f.names.openScope()

	sig := f.obj.Signature()
	f.params = make([]cppir.Param, sig.Params().Len())
	for i := range sig.Params().Len() {
		p := sig.Params().At(i)
		f.params[i].Type = g.cxxType(p.Type(), f.decl.Pos())
		if p.Name() != "" && p.Name() != "_" {
			f.params[i].Name = f.names.declare(p)
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
	f.result = g.resultType(sig, f.decl.Pos())
}

// funcDecl adds the definition of a function.
func (g *gen) funcDecl(f *funcInfo) {
	body := f.decl.Body
	g.markCounts(body)
	fg := newFuncGen(g, f.names, f)
	fg.flow = newFlow(g.info, body, fg.postInHead)
	fn := &cppir.Func{Pos: g.pos(f.decl.Pos()), Result: f.result, Name: f.name, Params: f.params, Body: &cppir.Block{}}
	fg.push(&fn.Body.Stmts, &fn.Body.End)
	if f.results != nil {
		res := f.obj.Signature().Results()
		for i := range res.Len() {
			r := res.At(i)
			fg.add(varDecl(g.pos(r.Pos()), g.cxxType(r.Type(), r.Pos()), f.results[i], &cppir.InitList{}))
		}
	}
	fg.fresh = f.results == nil // the named results are in the function's block (declLines)
	fg.stmts(body.List, body.Rbrace)
	fg.pop(body.Rbrace)
	g.decl(fn, &cppir.Blank{})
	f.names.closeScope()
}

// packageVars adds the package variables. Each one is constinit, so C++ runs no dynamic
// initialization of its own (whose order across files C++ doesn't define): a variable whose
// initializer is a constant gets its value at compile time, and the others start at their
// zero value and get their values in gp_var_init, in Go's order (types.Info.InitOrder).
func (g *gen) packageVars() {
	byVar := make(map[*types.Var]*types.Initializer)
	for _, init := range g.info.InitOrder {
		for _, v := range init.Lhs {
			byVar[v] = init
		}
	}
	for _, v := range g.vars {
		typ := g.cxxType(v.Type(), v.Pos())
		var value cppir.Expr = &cppir.InitList{} // the zero value
		if c := constInit(g.info, byVar[v]); c != nil {
			if basicOf(v.Type()).Info()&types.IsString != 0 {
				// "..."_gs can't be constexpr (the owner's destructor isn't), but this
				// constructor is.
				s := constant.StringVal(c)
				value = &cppir.InitList{Elems: []cppir.Expr{cxxName("gp::static_bytes"), lit(cppir.CString(s)), lit(strconv.Itoa(len(s)))}}
			} else {
				value = g.constant(c, v.Type(), v.Pos(), false)
			}
		}
		d := varDecl(g.pos(v.Pos()), typ, g.names[v], value)
		d.Specs = []string{"constinit"}
		g.decl(d)
	}
	for _, init := range g.info.InitOrder {
		if constInit(g.info, init) == nil {
			g.varInits = true
		}
	}
	if len(g.vars) > 0 {
		g.decl(&cppir.Blank{})
	}
}

// constInit returns the constant value of the initializer init if it gives one variable a
// constant, or nil.
func constInit(info *types.Info, init *types.Initializer) constant.Value {
	if init == nil || len(init.Lhs) != 1 || init.Lhs[0].Name() == "_" {
		return nil
	}
	if basicOf(init.Lhs[0].Type()) == nil {
		return nil
	}
	return info.Types[init.Rhs].Value
}

// initFuncs adds gp_var_init, which sets the package variables that constInit doesn't, and
// gp_init_funcs, which calls the init functions in source order (gp_init_all, after the
// namespace, calls both; docs/spec/cpp-mapping.md, "Package initialization"). They are
// generated code: their own lines have no place in the .gp source, but the statements of
// gp_var_init are mapped to the initializers.
func (g *gen) initFuncs() {
	if g.varInits {
		fg := newFuncGen(g, newFuncNames(), nil)
		fg.names.openScope()
		fn := &cppir.Func{Result: typeName("void"), Name: "gp_var_init", Body: &cppir.Block{}}
		fg.push(&fn.Body.Stmts, nil)
		for _, init := range g.info.InitOrder {
			if constInit(g.info, init) != nil {
				continue
			}
			g.markCounts(init.Rhs)
			targets := make([]target, len(init.Lhs))
			for i, v := range init.Lhs {
				if v.Name() != "_" {
					targets[i] = varTarget(g.names[v])
				}
			}
			fg.bind(init.Rhs.Pos(), targets, []ast.Expr{init.Rhs}, false)
		}
		g.decl(fn, &cppir.Blank{})
	}
	if len(g.initFns) > 0 {
		body := &cppir.Block{}
		for _, n := range g.initFns {
			body.Stmts = append(body.Stmts, &cppir.ExprStmt{X: call(n)})
		}
		g.decl(&cppir.Func{Result: typeName("void"), Name: "gp_init_funcs", Body: body}, &cppir.Blank{})
	}
}
