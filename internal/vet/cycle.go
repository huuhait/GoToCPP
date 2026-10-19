package vet

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// cycleAnalyzer finds reference cycles that gocpp never frees.
//
// gocpp has no garbage collector (D1): objects are reference counted, so
// objects that keep each other alive leak. The classic shapes are a closure
// that captures the variable that holds it (var f func(); f = func() { f() }),
// a struct whose field holds a closure that captures the struct
// (s.cb = func() { s.run() }), a map or slice of closures that captures the
// map or the slice, and two objects with strong pointers to each other (a
// parent and its child).
//
// The analysis is intraprocedural and syntactic: in one function (with the
// function literals inside it) it collects the stores
//
//	X.f = v, X[i] = v, X.f = append(X.f, v)   a store into the object X
//	ch <- v                                   a send holds the value
//	f = func() { ... }, c := &N{p: v}         a variable that is the object
//
// (also with := and var) where v is a closure that captures a local
// variable Y, a pointer, map, slice or channel variable Y, &Y, a method value
// Y.m of a pointer receiver, a composite literal that holds one of these, or
// the result of a call that returns a func and was given a closure
// (g = decorate(func() { g() })), and makes an edge from the root variable of
// the target (X) to Y. A cycle in the edges, a self-loop included, is
// reported at the store that closes it, with the path. p = q is a move, no
// store.
//
// An edge is dropped when something ends it: the function sets the same
// target to nil in a statement that always runs after the store (at the top
// level of the function, or deferred, or at the top level of the closure
// that is stored), or a method of the package sets that field of the type
// to nil (func (s *S) Close() { s.cb = nil }). A nil in an if, a loop, before
// the store or in another closure doesn't count: it may never run.
//
// Only local variables count: a closure doesn't capture a package-level
// variable, it names it. And code that touches C/C++ values is skipped
// (the guard): the C++ side owns those. A cycle across functions is not
// found.
var cycleAnalyzer = &analysis.Analyzer{
	Name:             "cycle",
	Doc:              "report reference cycles that gocpp's reference counting never frees (a closure that captures the variable that holds it, a parent and a child that point at each other)",
	RunDespiteErrors: true,
	Run:              runCycle,
}

func runCycle(pass *analysis.Pass) (any, error) {
	memo := make(map[types.Type]bool)
	cleaners := findCleaners(pass)
	for _, f := range pass.Files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			c := &cycleFinder{pass: pass, memo: memo, cleaners: cleaners}
			c.function(fd.Body)
		}
	}
	return nil, nil
}

type cycleFinder struct {
	pass  *analysis.Pass
	memo  map[types.Type]bool
	edges []edge

	// nils are the stores of nil that end what a store made: the targets
	// that the function sets to nil in a statement that always runs, with
	// the position of the statement (a deferred one runs last: laterThanAll).
	nils map[string][]token.Pos

	// cleaners are the fields that a method of the package sets to nil
	// (func (s *S) Close() { s.cb = nil }), by the type that has the field.
	cleaners map[cleanKey]bool
}

// laterThanAll is the position of a deferred store of nil.
const laterThanAll = token.Pos(1 << 30)

// A cleanKey is a field of a named type.
type cleanKey struct {
	typ   *types.Named
	field string
}

// An edge says that the object that the variable from stands for keeps the
// variable to alive.
type edge struct {
	from, to *types.Var
	pos      token.Pos // the store
	target   string    // the target of the store, as written: s.cb
	what     string    // what is stored: "a closure that captures s", "b"
	closure  bool      // the store is a closure
}

// A site is a place that stores a value in an object: lhs = rhs, a
// declaration with a value, or a send lhs <- rhs.
type site struct {
	pos      token.Pos
	lhs, rhs ast.Expr
}

func (c *cycleFinder) function(body *ast.BlockStmt) {
	info := c.pass.TypesInfo
	c.nils = make(map[string][]token.Pos)
	var sites []site
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if (n.Tok == token.ASSIGN || n.Tok == token.DEFINE) && len(n.Lhs) == len(n.Rhs) {
				for i, lhs := range n.Lhs {
					sites = append(sites, site{n.Pos(), lhs, n.Rhs[i]})
				}
			}
		case *ast.ValueSpec:
			if len(n.Names) == len(n.Values) {
				for i, name := range n.Names {
					sites = append(sites, site{n.Pos(), name, n.Values[i]})
				}
			}
		case *ast.SendStmt:
			sites = append(sites, site{n.Pos(), n.Chan, n.Value})
		}
		return true
	})
	// A store of nil breaks the cycle only if it always runs, after the
	// store (or at the end, deferred): at the top level of the function.
	// One in an if, a loop or a closure may never run.
	isNil := func(e ast.Expr) bool { tv, ok := info.Types[e]; return ok && tv.IsNil() }
	for _, st := range body.List {
		switch st := st.(type) {
		case *ast.AssignStmt:
			if st.Tok == token.ASSIGN && len(st.Lhs) == len(st.Rhs) {
				for i, r := range st.Rhs {
					if isNil(r) {
						t := types.ExprString(st.Lhs[i])
						c.nils[t] = append(c.nils[t], st.Pos())
					}
				}
			}
		case *ast.DeferStmt:
			ast.Inspect(st.Call, func(n ast.Node) bool {
				if as, ok := n.(*ast.AssignStmt); ok && as.Tok == token.ASSIGN && len(as.Lhs) == len(as.Rhs) {
					for i, r := range as.Rhs {
						if isNil(r) {
							t := types.ExprString(as.Lhs[i])
							c.nils[t] = append(c.nils[t], laterThanAll)
						}
					}
				}
				return true
			})
		}
	}
	// A closure that clears the target at its top level (h.f = func() { h.f = nil; ... }) ends the cycle when it
	// runs: this is the way to make a func that runs once.
	for _, s := range sites {
		if fl, ok := ast.Unparen(s.rhs).(*ast.FuncLit); ok {
			for _, st := range fl.Body.List {
				if as, ok := st.(*ast.AssignStmt); ok && as.Tok == token.ASSIGN && len(as.Lhs) == len(as.Rhs) {
					for i, r := range as.Rhs {
						if isNil(r) {
							t := types.ExprString(as.Lhs[i])
							c.nils[t] = append(c.nils[t], laterThanAll)
						}
					}
				}
			}
		}
	}
	for _, s := range sites {
		c.store(s)
	}
	slices.SortStableFunc(c.edges, func(a, b edge) int { return int(a.pos) - int(b.pos) })
	c.report()
}

// broken reports whether the function ends what a store at pos made in the
// target: it sets it to nil afterwards, in a statement that always runs, or
// a method of the type that has the field does.
func (c *cycleFinder) broken(lhs ast.Expr, target string, pos token.Pos) bool {
	for _, p := range c.nils[target] {
		if p > pos {
			return true
		}
	}
	if sel, ok := ast.Unparen(lhs).(*ast.SelectorExpr); ok {
		if t, ok := c.pass.TypesInfo.Types[sel.X]; ok {
			if n := namedOf(t.Type); n != nil && c.cleaners[cleanKey{n, sel.Sel.Name}] {
				return true
			}
		}
	}
	return false
}

// namedOf returns the named type of t or of what t points to.
func namedOf(t types.Type) *types.Named {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	n, _ := types.Unalias(t).(*types.Named)
	return n
}

// findCleaners finds the methods that set a field of their receiver to nil.
func findCleaners(pass *analysis.Pass) map[cleanKey]bool {
	info := pass.TypesInfo
	out := make(map[cleanKey]bool)
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || len(fd.Recv.List) == 0 || len(fd.Recv.List[0].Names) == 0 {
				continue
			}
			recv, _ := info.Defs[fd.Recv.List[0].Names[0]].(*types.Var)
			named := (*types.Named)(nil)
			if recv != nil {
				named = namedOf(recv.Type())
			}
			if named == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i, l := range as.Lhs {
					sel, ok := ast.Unparen(l).(*ast.SelectorExpr)
					if !ok {
						continue
					}
					if id, ok := sel.X.(*ast.Ident); ok && info.Uses[id] == recv {
						if tv, ok := info.Types[as.Rhs[i]]; ok && tv.IsNil() {
							out[cleanKey{named, sel.Sel.Name}] = true
						}
					}
				}
				return true
			})
		}
	}
	return out
}

// store records the edges of a store.
func (c *cycleFinder) store(s site) {
	lhs, rhs := s.lhs, s.rhs
	root := c.rootVar(lhs)
	if root == nil || !isLocal(root) || hasInvalid(root.Type(), c.memo) {
		return
	}
	target := types.ExprString(lhs)
	if c.broken(lhs, target, s.pos) {
		return
	}
	_, plain := ast.Unparen(lhs).(*ast.Ident)
	// p = q moves a pointer (a func, a map, a slice, a channel), it doesn't
	// store one. And p = &T{... p ...} makes a new object that holds the old
	// value of p: the variable is only the name of what it holds now.
	move := c.isMove(rhs)
	_, fresh := freshObject(rhs)
	for _, h := range c.holders(rhs) {
		if plain && (move || fresh && h.obj == root && !h.closure) {
			continue
		}
		if hasInvalid(h.obj.Type(), c.memo) {
			continue
		}
		c.edges = append(c.edges, edge{root, h.obj, s.pos, target, h.what, h.closure})
	}
}

// isMove reports whether e is a variable (maybe converted): storing it
// copies a reference.
func (c *cycleFinder) isMove(e ast.Expr) bool {
	e = ast.Unparen(e)
	if call, ok := e.(*ast.CallExpr); ok && len(call.Args) == 1 && c.isConversion(call) {
		return c.isMove(call.Args[0])
	}
	_, ok := e.(*ast.Ident)
	return ok
}

// freshObject reports whether e makes a new object: a composite literal or
// the address of one.
func freshObject(e ast.Expr) (*ast.CompositeLit, bool) {
	e = ast.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = ast.Unparen(u.X)
	}
	cl, ok := e.(*ast.CompositeLit)
	return cl, ok
}

// A holder is a variable that a stored value keeps alive.
type holder struct {
	obj     *types.Var
	what    string
	closure bool
}

// holders returns the local variables that the value e keeps alive.
func (c *cycleFinder) holders(e ast.Expr) []holder {
	info := c.pass.TypesInfo
	e = ast.Unparen(e)
	switch e := e.(type) {
	case *ast.FuncLit:
		var out []holder
		seen := make(map[*types.Var]bool)
		ast.Inspect(e.Body, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			v, _ := info.Uses[id].(*types.Var)
			if v == nil || v.IsField() || !isLocal(v) || seen[v] {
				return true
			}
			if v.Pos() >= e.Pos() && v.Pos() < e.End() {
				return true // declared inside the literal: not captured
			}
			seen[v] = true
			out = append(out, holder{v, "a closure that captures " + v.Name(), true})
			return true
		})
		return out
	case *ast.CompositeLit:
		var out []holder
		for _, el := range e.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				el = kv.Value
			}
			out = append(out, c.holders(el)...)
		}
		return out
	case *ast.UnaryExpr:
		if e.Op != token.AND {
			return nil
		}
		if cl, ok := ast.Unparen(e.X).(*ast.CompositeLit); ok {
			return c.holders(cl)
		}
		if v := c.identVar(e.X); v != nil {
			return []holder{{v, "&" + v.Name(), false}}
		}
	case *ast.Ident:
		v := c.identVar(e)
		if v == nil {
			return nil
		}
		switch v.Type().Underlying().(type) {
		case *types.Pointer, *types.Map, *types.Slice, *types.Chan:
			// The variable holds the object (or the map, the slice, the
			// channel), and what is stored holds the variable's value.
			return []holder{{v, v.Name(), false}}
		case *types.Signature:
			return []holder{{v, "the func " + v.Name(), true}}
		}
	case *ast.SelectorExpr:
		// A method value x.m holds x.
		// Only when the receiver is a pointer (or an interface): a value
		// receiver is copied into the method value.
		if sel := info.Selections[e]; sel != nil && sel.Kind() == types.MethodVal && holdsReceiver(sel) {
			if v := c.identVar(e.X); v != nil {
				return []holder{{v, "the method value " + types.ExprString(e), true}}
			}
		}
	case *ast.CallExpr:
		if id, ok := ast.Unparen(e.Fun).(*ast.Ident); ok {
			if _, ok := info.Uses[id].(*types.Builtin); ok && id.Name == "append" && len(e.Args) > 1 {
				var out []holder
				for i, a := range e.Args[1:] {
					if e.Ellipsis.IsValid() && i == len(e.Args)-2 {
						break // append(s, t...) copies the elements of t, it doesn't hold t
					}
					out = append(out, c.holders(a)...)
				}
				return out
			}
		}
		// A call that returns a func holds the funcs that it was given
		// (a decorator: g = twice(func() { g() })).
		if tv, ok := info.Types[e]; ok && !tv.IsType() && tv.Type != nil {
			if _, isFunc := tv.Type.Underlying().(*types.Signature); isFunc && !c.isConversion(e) {
				var out []holder
				for _, a := range e.Args {
					if fl, ok := ast.Unparen(a).(*ast.FuncLit); ok {
						out = append(out, c.holders(fl)...)
					}
				}
				if out != nil {
					return out
				}
			}
		}
		// A conversion T(x) holds what x holds.
		// (To a number, a string or unsafe.Pointer, it holds nothing:
		// uintptr(unsafe.Pointer(p)) is an address, not a reference.)
		if tv, ok := info.Types[e.Fun]; ok && tv.IsType() && len(e.Args) == 1 {
			if _, basic := tv.Type.Underlying().(*types.Basic); basic {
				return nil
			}
			return c.holders(e.Args[0])
		}
	}
	return nil
}

func (c *cycleFinder) identVar(e ast.Expr) *types.Var {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	v, _ := c.pass.TypesInfo.ObjectOf(id).(*types.Var)
	if v == nil || v.IsField() || !isLocal(v) {
		return nil
	}
	return v
}

// rootVar returns the variable that a store target is part of: s for s.f,
// s.f[i], (*s).f and s itself. A package-qualified name has none.
func (c *cycleFinder) rootVar(e ast.Expr) *types.Var {
	info := c.pass.TypesInfo
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok {
				if _, ok := info.Uses[id].(*types.PkgName); ok {
					return nil
				}
			}
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.Ident:
			if x.Name == "_" {
				return nil
			}
			v, _ := info.ObjectOf(x).(*types.Var)
			if v == nil || v.IsField() {
				return nil
			}
			return v
		default:
			return nil
		}
	}
}

// isLocal reports whether v is a variable of a function (also a parameter
// or a result), which a closure captures; a package-level variable is not
// captured.
func isLocal(v *types.Var) bool {
	return v.Pkg() != nil && v.Parent() != nil && v.Parent() != v.Pkg().Scope() && !v.IsField()
}

// report finds the cycles: each edge, in source order, that closes a cycle
// with the edges before it is reported, once for each set of variables.
func (c *cycleFinder) report() {
	reported := make(map[string]bool)
	for i, e := range c.edges {
		path := c.path(e.to, e.from, c.edges[:i])
		if path == nil && e.from != e.to {
			continue
		}
		cycle := append([]edge{e}, path...)
		var names []string
		for _, x := range cycle {
			names = append(names, x.from.Name())
		}
		slices.Sort(names)
		key := strings.Join(names, ",")
		if reported[key] {
			continue
		}
		reported[key] = true
		c.diagnose(cycle)
	}
}

// path returns the edges of a shortest path from from to to among edges,
// nil if there is none or from == to (the empty path: a self-loop closes
// itself).
func (c *cycleFinder) path(from, to *types.Var, edges []edge) []edge {
	if from == to {
		return nil
	}
	prev := map[*types.Var]*edge{from: nil}
	queue := []*types.Var{from}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for i := range edges {
			e := &edges[i]
			if e.from != v {
				continue
			}
			if _, seen := prev[e.to]; seen {
				continue
			}
			prev[e.to] = e
			if e.to == to {
				var out []edge
				for x := to; prev[x] != nil; x = prev[x].from {
					out = append(out, *prev[x])
				}
				slices.Reverse(out)
				return out
			}
			queue = append(queue, e.to)
		}
	}
	return nil
}

// diagnose reports the cycle that its first edge closes; the other edges,
// in the order of the cycle, follow.
func (c *cycleFinder) diagnose(cycle []edge) {
	closes := cycle[0]
	// The path of the cycle, from the closing edge on.
	var steps []string
	anyClosure := false
	for _, e := range cycle {
		steps = append(steps, fmt.Sprintf("%s holds %s", e.target, e.what))
		anyClosure = anyClosure || e.closure
	}
	var b strings.Builder
	if len(cycle) == 1 && closes.closure && closes.from == closes.to {
		fmt.Fprintf(&b, "reference cycle: %s holds %s, which keeps %s alive", closes.target, closes.what, closes.from.Name())
	} else {
		fmt.Fprintf(&b, "reference cycle: %s", strings.Join(steps, ", and "))
	}
	b.WriteString("; gocpp frees objects by reference counting, so objects that keep each other alive are never freed. ")
	if anyClosure {
		fmt.Fprintf(&b, "Break it: capture a weak pointer instead (weak.Make(%s) in the closure and wp.Value() inside it), pass the func as a parameter, or set %s = nil when you are done", closes.from.Name(), closes.target)
	} else {
		fmt.Fprintf(&b, "Break it: make one of the pointers weak (weak.Pointer[T], weak.Make(p) and wp.Value()), or set it to nil before you drop the objects")
	}
	d := analysis.Diagnostic{Pos: closes.pos, Category: "cycle", Message: b.String()}
	for _, e := range cycle[1:] {
		d.Related = append(d.Related, analysis.RelatedInformation{Pos: e.pos, Message: fmt.Sprintf("%s holds %s", e.target, e.what)})
	}
	c.pass.Report(d)
}

// isConversion reports whether the call is a conversion T(x).
func (c *cycleFinder) isConversion(call *ast.CallExpr) bool {
	tv, ok := c.pass.TypesInfo.Types[call.Fun]
	return ok && tv.IsType()
}

// holdsReceiver reports whether the method value of the selection keeps its
// receiver alive: the method has a pointer receiver, or is a method of an
// interface. A method with a value receiver gets a copy.
func holdsReceiver(sel *types.Selection) bool {
	fn, ok := sel.Obj().(*types.Func)
	if !ok {
		return true
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return true
	}
	t := types.Unalias(recv.Type())
	if _, ok := t.(*types.Pointer); ok {
		return true
	}
	return types.IsInterface(t)
}
