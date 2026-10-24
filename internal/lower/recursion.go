package lower

import (
	"go/ast"
	"go/types"
)

// Recursion in an optimized build (ADR-136 amendment, D7). A Go program whose recursion never
// ends dies with "fatal error: stack overflow". An optimized C++ build would not: the compilers
// turn a call in tail position into a jump, and a recursion that ends in a call of itself into a
// loop (return f(n+1), and clang also return f(n+1) + x), and clang may drop a recursion that has
// no side effect, as C++'s forward-progress rule lets it assume that the call returns
// (return f(n+1) then returns at once). So a function that can call itself, directly or through
// other functions of its package, carries GP_RECURSIVE (no tail calls in it: clang's
// disable_tail_calls, g++'s optimize("no-optimize-sibling-calls")) and starts with
// GP_RECURSION_MARK() (clang: an empty asm volatile, a side effect that costs no instruction), both in
// gocpp/rt.hpp. The other functions keep their tail calls.
//
// What is covered: direct recursion and cycles through calls of the package's own functions and
// methods (by name, also inside the func literals of their bodies, which count as the function's
// own calls). Not covered: a cycle through another package, through a func value or an interface
// method, or a func literal that calls itself through a variable.

// isRecursive reports whether the function of decl is in a cycle of calls of its package.
func (g *gen) isRecursive(decl *ast.FuncDecl) bool {
	if decl == nil || decl.Body == nil {
		return false
	}
	self, _ := g.info.Defs[decl.Name].(*types.Func)
	if self == nil {
		return false
	}
	if g.recursive == nil {
		g.recursive = g.findRecursive()
	}
	return g.recursive[self]
}

// findRecursive returns the functions of the package that are in a cycle of the call graph: the
// strongly connected components (Tarjan) of more than one function, and the functions that call
// themselves.
func (g *gen) findRecursive() map[*types.Func]bool {
	g.declOf(nil) // fills g.funcDecls
	edges := make(map[*types.Func][]*types.Func)
	self := make(map[*types.Func]bool)
	for fn, decl := range g.funcDecls {
		seen := make(map[*types.Func]bool)
		ast.Inspect(decl.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := g.staticCallee(c)
			if callee == nil || g.funcDecls[callee] == nil || seen[callee] {
				return true
			}
			seen[callee] = true
			if callee == fn {
				self[fn] = true
			}
			edges[fn] = append(edges[fn], callee)
			return true
		})
	}
	out := make(map[*types.Func]bool)
	for fn := range self {
		out[fn] = true
	}
	// Tarjan's algorithm, recursive (the depth is at most the number of functions).
	index := make(map[*types.Func]int)
	low := make(map[*types.Func]int)
	onStack := make(map[*types.Func]bool)
	var stack []*types.Func
	next := 0
	var visit func(v *types.Func)
	visit = func(v *types.Func) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range edges[v] {
			if _, ok := index[w]; !ok {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] == index[v] {
			var comp []*types.Func
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			if len(comp) > 1 {
				for _, w := range comp {
					out[w] = true
				}
			}
		}
	}
	for fn := range g.funcDecls {
		if _, ok := index[fn]; !ok {
			visit(fn)
		}
	}
	return out
}

// staticCallee returns the function or method that the call c calls by name (f(x), pkg.F(x),
// x.M(), f[T](x)), as its generic origin, or nil for a call of a func value, a conversion or a
// builtin.
func (g *gen) staticCallee(c *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	case *ast.IndexExpr:
		switch x := fun.X.(type) {
		case *ast.Ident:
			id = x
		case *ast.SelectorExpr:
			id = x.Sel
		}
	case *ast.IndexListExpr:
		switch x := fun.X.(type) {
		case *ast.Ident:
			id = x
		case *ast.SelectorExpr:
			id = x.Sel
		}
	}
	if id == nil {
		return nil
	}
	fn, _ := g.info.Uses[id].(*types.Func)
	if fn == nil {
		return nil
	}
	return fn.Origin()
}
