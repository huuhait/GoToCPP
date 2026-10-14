package cppgen

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strconv"
	"strings"
)

// Declarations: functions, package variables and the package's initialization.

// A funcInfo is a function of the package, with what prepare chose for it.
type funcInfo struct {
	decl *ast.FuncDecl
	obj  *types.Func
	name string // the C++ name

	names   *funcNames
	header  string   // the C++ declarator: std::tuple<int64_t, bool> add(int64_t a, int64_t b)
	results []string // the C++ names of the named results, or nil
}

// prepare chooses the C++ names of the function's parameters and named results, and makes
// its header. Parameters are passed by value, as real copies, like Go's. Several results are
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
	params := make([]string, sig.Params().Len())
	for i := range sig.Params().Len() {
		p := sig.Params().At(i)
		params[i] = g.cxxType(p.Type(), f.decl.Pos())
		if p.Name() != "" && p.Name() != "_" {
			params[i] += " " + f.names.declare(p, nil)
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
				f.results[i] = f.names.declare(r, nil)
			}
		}
	}
	f.header = g.resultType(sig, f.decl.Pos()) + " " + f.name + "(" + strings.Join(params, ", ") + ")"
}

// funcDecl writes the definition of a function.
func (g *gen) funcDecl(f *funcInfo) {
	body := f.decl.Body
	fg := &funcGen{g: g, names: f.names, fn: f, flow: newFlow(g.info, body), contLabels: make(map[ast.Stmt]string)}
	g.w.line(g.pos(f.decl.Pos()), f.header+" {")
	g.w.indent()
	if f.results != nil {
		res := f.obj.Signature().Results()
		for i := range res.Len() {
			r := res.At(i)
			g.w.line(g.pos(r.Pos()), g.cxxType(r.Type(), r.Pos())+" "+f.results[i]+"{};")
		}
	}
	fg.stmts(body.List)
	g.w.dedent()
	g.w.line(g.pos(body.Rbrace), "}")
	g.w.blank()
	f.names.closeScope()
}

// packageVars writes the package variables. Each one is constinit, so C++ runs no dynamic
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
		decl := "constinit " + typ + " " + g.names[v]
		init := byVar[v]
		if value := constInit(g.info, init); value != nil {
			if basicOf(v.Type()).Info()&types.IsString != 0 {
				// "..."_gs can't be constexpr (the owner's destructor isn't), but this
				// constructor is.
				s := constant.StringVal(value)
				decl += "{gp::static_bytes, " + cString(s) + ", " + strconv.Itoa(len(s)) + "}"
			} else {
				decl += " = " + g.constant(value, v.Type(), v.Pos(), false).text
			}
		} else {
			decl += "{}"
		}
		g.w.line(g.pos(v.Pos()), decl+";")
	}
	for _, init := range g.info.InitOrder {
		if constInit(g.info, init) == nil {
			g.varInits = true
		}
	}
	if len(g.vars) > 0 {
		g.w.blank()
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

// initFuncs writes gp_var_init, which sets the package variables that constInit doesn't, and
// gp_init_funcs, which calls the init functions in source order (gp_init_all, after the
// namespace, calls both; docs/spec/cpp-mapping.md, "Package initialization").
func (g *gen) initFuncs() {
	if g.varInits {
		fg := &funcGen{g: g, names: newFuncNames()}
		fg.names.openScope()
		g.w.plain("void gp_var_init() {")
		g.w.indent()
		for _, init := range g.info.InitOrder {
			if constInit(g.info, init) != nil {
				continue
			}
			targets := make([]target, len(init.Lhs))
			for i, v := range init.Lhs {
				if v.Name() != "_" {
					targets[i] = target{name: g.names[v]}
				}
			}
			fg.bind(init.Rhs.Pos(), targets, []ast.Expr{init.Rhs}, false)
		}
		g.w.dedent()
		g.w.plain("}")
		g.w.blank()
	}
	if len(g.initFns) > 0 {
		g.w.plain("void gp_init_funcs() {")
		g.w.indent()
		for _, name := range g.initFns {
			g.w.plain(name + "();")
		}
		g.w.dedent()
		g.w.plain("}")
		g.w.blank()
	}
}
