package vet

import (
	"go/ast"
	"go/types"
	"maps"
	"reflect"
)

// Replacing the C/C++ arguments of calls.
//
// printf checks the arguments of a call against the verbs of the format, so
// a call with one argument that comes from C/C++ (v.size(), a std::string)
// still has other arguments worth checking. pruner.rewrite makes a copy of
// the statement in which each argument of a call that touches invalid types
// is a new identifier with the type of the argument (when go/types has one:
// int(v.size()) has type int) or else any, which printf accepts for every
// verb. The statements that stay unchanged are shared with the original
// syntax; the nodes that are copied get the entries of the originals in a
// copy of the type information.

var anyType = types.Universe.Lookup("any").Type()

// rewritten returns the copy of n in which the calls have no C/C++ arguments,
// or nil when n would still touch invalid types (or had nothing to replace).
func (c *pruner) rewritten(n ast.Node) ast.Node {
	nn := c.rewrite(n)
	if nn == n || c.touches(nn) {
		return nil
	}
	return nn
}

// fixed returns rewritten(n) or, when that is nil, n.
func fixed[T ast.Node](c *pruner, n T) T {
	if rv := reflect.ValueOf(n); !c.stmts || !rv.IsValid() || rv.IsNil() {
		return n
	}
	if nn := c.rewritten(n); nn != nil {
		return nn.(T)
	}
	return n
}

// writable returns the type information that has the new nodes: a copy of
// the one of the package, made at the first need.
func (c *pruner) writable() *types.Info {
	if !c.cloned {
		info := *c.info
		info.Types = maps.Clone(c.info.Types)
		info.Selections = maps.Clone(c.info.Selections)
		c.info = &info
		c.cloned = true
	}
	return c.info
}

// copied makes the copy of the node orig (a pointer to a struct) and gives
// it the type information of orig.
func (c *pruner) copied(orig reflect.Value) reflect.Value {
	cp := reflect.New(orig.Type().Elem())
	cp.Elem().Set(orig.Elem())
	info := c.writable()
	switch o := orig.Interface().(type) {
	case ast.Expr:
		if tv, ok := info.Types[o]; ok {
			info.Types[cp.Interface().(ast.Expr)] = tv
		}
		if sel, ok := o.(*ast.SelectorExpr); ok {
			if s := info.Selections[sel]; s != nil {
				info.Selections[cp.Interface().(*ast.SelectorExpr)] = s
			}
		}
	}
	return cp
}

// rewrite returns n, or a copy of it with the C/C++ arguments of calls
// replaced.
func (c *pruner) rewrite(n ast.Node) ast.Node {
	v := reflect.ValueOf(n)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return n
	}
	var cp reflect.Value
	if call, ok := n.(*ast.CallExpr); ok {
		return c.rewriteCall(call)
	}
	st := v.Elem()
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		switch f.Kind() {
		case reflect.Interface, reflect.Pointer:
			if f.IsNil() {
				continue
			}
			child, ok := f.Interface().(ast.Node)
			if !ok {
				continue
			}
			if nc := c.rewrite(child); nc != child {
				if !cp.IsValid() {
					cp = c.copied(v)
				}
				cp.Elem().Field(i).Set(reflect.ValueOf(nc))
			}
		case reflect.Slice:
			if !f.Type().Elem().Implements(reflect.TypeFor[ast.Node]()) {
				continue
			}
			var ns reflect.Value
			for j := 0; j < f.Len(); j++ {
				el := f.Index(j)
				if el.IsNil() {
					continue
				}
				child := el.Interface().(ast.Node)
				if nc := c.rewrite(child); nc != child {
					if !ns.IsValid() {
						ns = reflect.MakeSlice(f.Type(), f.Len(), f.Len())
						reflect.Copy(ns, f)
					}
					ns.Index(j).Set(reflect.ValueOf(nc))
				}
			}
			if ns.IsValid() {
				if !cp.IsValid() {
					cp = c.copied(v)
				}
				cp.Elem().Field(i).Set(ns)
			}
		}
	}
	if cp.IsValid() {
		return cp.Interface().(ast.Node)
	}
	return n
}

// rewriteCall rewrites a call: first inside, then the arguments that still
// touch invalid types are replaced, if the function called doesn't.
func (c *pruner) rewriteCall(call *ast.CallExpr) ast.Node {
	if failedBuiltin(c.info, call) {
		return call // go/ssa and satisfy need the signature of the builtin
	}
	fun := c.rewrite(call.Fun).(ast.Expr)
	args := make([]ast.Expr, len(call.Args))
	changed := fun != call.Fun
	for i, a := range call.Args {
		na := c.rewrite(a).(ast.Expr)
		if !c.touches(fun) && c.touches(na) {
			na = c.dummy(a, call.Ellipsis.IsValid() && i == len(call.Args)-1)
		}
		args[i] = na
		if na != a {
			changed = true
		}
	}
	if !changed {
		return call
	}
	cp := c.copied(reflect.ValueOf(call)).Interface().(*ast.CallExpr)
	cp.Fun, cp.Args = fun, args
	return cp
}

// dummy returns an identifier that stands for the expression e, of the type
// of e if it has a complete one, else any ([]any for the last argument of
// a call with ...).
func (c *pruner) dummy(e ast.Expr, spread bool) ast.Expr {
	tv := c.info.Types[e] // the mode of a value comes from here: it can't be made
	typ := tv.Type
	if typ == nil || hasInvalid(typ, c.memo) {
		typ = anyType
		if spread {
			typ = types.NewSlice(anyType)
		}
	}
	id := &ast.Ident{NamePos: e.Pos(), Name: "_"}
	tv.Type, tv.Value = typ, nil
	c.writable().Types[id] = tv
	return id
}
