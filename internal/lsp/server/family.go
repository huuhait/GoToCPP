package server

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// What the user is told about a search that reaches further than the name they
// asked for (step 11.8, review round 1; narrowed in the LSP speed work, ADR-052).
//
// The family of a method is gopls's: the methods that a use ties together (a
// value of one type passed, assigned, returned, sent, compared, converted or
// asserted where an interface with the method is needed: see ties). A type
// that could be used as the interface, but no use in the module needs to be,
// is not in the family, so renaming Lener.Len leaves Counterer.Len alone
// when nothing converts a Counterer to a Lener. A rename of a method says
// which other declarations it changed, so the user can undo it.
//
// Before ADR-052 the family was every method with the name and the shape of
// a method of an interface of the module (the type would satisfy it), whether
// or not a use needs it: Options.WideFamily (gocpp lsp -wide-family) brings
// that back, with the older message that says which of the declarations no
// use needs. It was slow (0.7-0.9 s for a method that all the types of a
// 30k-line module share, where gopls takes 4 ms) and wider than gopls.

// otherDeclarations tells the user which declarations beyond the one they
// asked about a rename of a method changed, with an info message.
func (rn *renamer) otherDeclarations() {
	sym := rn.sym
	if len(sym.objs) != 1 || rn.e.notify == nil {
		return
	}
	asked, ok := sym.objs[0].(*types.Func)
	if !ok || recvBase(asked) == nil {
		return
	}
	type decl struct {
		name string
		pos  token.Position
		typ  types.Type // the receiver's type (never an interface)
	}
	var others []decl
	for _, h := range rn.res.hits {
		fn, ok := h.obj.(*types.Func)
		if !ok || !h.def || recvBase(fn) == nil || origin(fn) == origin(asked) {
			continue
		}
		others = append(others, decl{objectName(fn), h.position(), recvBase(fn)})
	}
	if len(others) == 0 {
		return
	}
	var required map[string]bool
	if rn.s.wideFamily {
		required = rn.requiredTypes()
	}
	where := func(d decl) string {
		return fmt.Sprintf("%s (%s:%d)", d.name, filepath.Base(d.pos.Filename), d.pos.Line)
	}
	var all, unused []string
	for i, d := range others {
		if i < 5 {
			all = append(all, where(d))
		}
		if required != nil && !types.IsInterface(d.typ) && !required[typeKey(d.typ)] {
			unused = append(unused, where(d))
		}
	}
	if !rn.s.wideFamily {
		msg := fmt.Sprintf("gocpp rename: %s was also renamed in %d other method declaration(s) that a use in the module ties to the same interface, or that a type assertion, type switch or errors.As on the interface can reach: %s", sym.name, len(others), strings.Join(all, ", "))
		if len(others) > len(all) {
			msg += ", ..."
		}
		msg += ". Undo the rename if some of them are unrelated."
		rn.showInfo(msg)
		return
	}
	msg := fmt.Sprintf("gocpp rename: %s was also renamed in %d other method declaration(s) that have its name and shape: %s", sym.name, len(others), strings.Join(all, ", "))
	if len(others) > len(all) {
		msg += ", ..."
	}
	msg += ". gocpp renames every method of the module that has the name and shape of a method of the family's interfaces, which is wider than gopls (only methods that a use ties to the interface)."
	if len(unused) > 0 {
		msg += " Not needed by any use that gocpp found (no value of the type is converted to or asserted from the interface): " + strings.Join(unused[:min(5, len(unused))], ", ")
		if len(unused) > 5 {
			msg += ", ..."
		}
		msg += "."
	}
	msg += " Undo the rename if some of them are unrelated."
	rn.showInfo(msg)
}

func (rn *renamer) showInfo(msg string) {
	if err := rn.e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Info, Message: msg}); err != nil {
		rn.e.log.Printf("window/showMessage: %v", err)
	}
}

// typeKey names a named type (or the type a pointer points to) independently
// of the check it is from: its package path and name; "" for any other type.
func typeKey(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() != nil {
		return strings.TrimSuffix(n.Obj().Pkg().Path(), "_test") + "." + n.Obj().Name()
	}
	return ""
}

// requiredTypes returns the types (typeKey) of the module that a use needs to
// be the interface of the family: a value of the type is passed, assigned,
// returned, sent, put in a composite literal or compared where an interface
// with a method of the family's name is wanted, converted to one, or asserted
// from one.
func (rn *renamer) requiredTypes() map[string]bool {
	name := rn.sym.name
	required := make(map[string]bool)
	for _, v := range rn.res.views {
		for _, t := range v.ties() {
			if t.src == nil || types.IsInterface(t.src) || t.asserted {
				continue
			}
			it, ok := t.dst.Underlying().(*types.Interface)
			if !ok {
				continue
			}
			for m := range it.Methods() {
				if m.Name() == name {
					if k := typeKey(t.src); k != "" {
						required[k] = true
					}
					break
				}
			}
		}
	}
	return required
}

// A tie is a use that needs the type dst (a named interface) of a value of
// the type src (a named type, a pointer to one, or another named interface):
// the use makes src's methods the implementations of dst's. gopls ties the
// methods of a family the same way (the implements relation alone, a type that
// merely could be used as the interface, does not).
type tie struct {
	src, dst types.Type
	names    map[string]bool // the names of dst's methods
	// asserted marks a record that dst, an interface, is the target of a type
	// assertion or a type switch case (src is nil): a type that reaches it
	// only through the assertion is not tied to it.
	asserted bool
	// pos is where the assertion, type switch case or errors.As call is (for
	// asserted ties): a message that names the site says where.
	pos token.Pos
}

// ties returns the ties of the view's files: the uses of the package that
// need an interface of a value of another type. They are computed once per
// checked package (cache.Package.Memo), so the references and renames of a
// method that every type of the module shares don't read the module again.
func (v *view) ties() []tie { return v.tieInfo().ties }

// A tieInfo is what scanTies found in a package: the ties, and the types of the
// values that can be the dynamic value of an interface: those that the package
// converts to an interface (any interface, the empty one too, which ties
// nothing) in any form that a value flows to a place of that type: an
// assignment, a declaration, a return, an argument, an element of a composite
// literal, a channel send, a range variable, the key of a map index, the
// values of a call that returns several, the forms that return a value and
// whether it is there, and a type argument of an instantiation (the constraint
// is an interface). A comparison with an interface does not count: it keeps
// nothing. The types that these are made of count too (record). A value of a type that is none of these can
// be the dynamic value of no interface, so no type assertion takes it out.
// Struct types (named or not, local, generic, an alias) are recorded as they
// are: the lookup of a method on them finds what they promote.
type tieInfo struct {
	ties      []tie
	converted []types.Type
}

func (v *view) tieInfo() *tieInfo {
	if v.info == nil {
		return &tieInfo{}
	}
	if v.pkg == nil || v.test {
		return scanTies(v.types, v.info, v.files)
	}
	return v.pkg.Memo(tiesKey{}, func() any { return scanTies(v.types, v.info, v.files) }).(*tieInfo)
}

// converted returns the types of the values that the view's package converts to
// an interface, or lets flow to a place that does (see scanTies).
func (v *view) converted() []types.Type { return v.tieInfo().converted }

type tiesKey struct{}

// scanTies finds the ties of the files: a value that is passed, assigned,
// returned, sent, put in a composite literal, compared or converted where an
// interface is wanted, and the types that a type assertion or a type switch
// takes out of an interface.
func scanTies(self *types.Package, info *types.Info, files map[string]*ast.File) *tieInfo {
	var out []tie
	var converted []types.Type
	convSeen := make(map[types.Type]bool)
	seen := make(map[[2]types.Type]bool)
	// A tie is between a type that has methods (any type: a named one, a
	// pointer to one, an unnamed struct that embeds one) and an interface with
	// methods (a named one, an interface literal, the constraint of a type
	// parameter): the empty interface ties nothing.
	usable := func(t types.Type) bool {
		switch t := types.Unalias(t).(type) {
		case *types.Basic:
			return false
		case *types.Named:
			return t.TypeParams().Len() == 0 || t.TypeArgs().Len() > 0
		}
		return true
	}
	// record notes that a value of the type src can be the dynamic value of an
	// interface, and so can the values it holds: reflection reads the fields of
	// a struct, the elements of a slice, an array, a channel or a map, the value
	// a pointer points to and the results of a function, and takes them out as
	// interfaces (reflect.ValueOf(s).Field(0).Interface(), fmt printing a nested
	// struct). So the types that a type is made of are reached with it, as far
	// as they go (a type of the standard library is not looked into: none of its
	// types is in the module, and its graph is large). A pointer to a type counts
	// as the type, and the type as its pointer: the lookup of a method with the
	// type addressable finds both (the pointer is only to deduplicate). A
	// struct that embeds a type has its methods, so it is the type that the
	// methods promote from, too (the lookup of a method on it finds them).
	var record func(src types.Type)
	record = func(src types.Type) {
		if src == nil {
			return
		}
		t := types.Unalias(src)
		if p, ok := t.(*types.Pointer); ok {
			t = types.Unalias(p.Elem())
		}
		switch t.(type) {
		case *types.Basic, *types.Tuple, *types.Interface:
			return
		}
		if types.IsInterface(t) || convSeen[t] {
			return
		}
		convSeen[t] = true
		converted = append(converted, t)
		if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil && foreignStd(n.Obj().Pkg(), self) {
			return
		}
		switch u := t.Underlying().(type) {
		case *types.Struct:
			for i := range u.NumFields() {
				record(u.Field(i).Type())
			}
		case *types.Pointer:
			record(u.Elem())
		case *types.Slice:
			record(u.Elem())
		case *types.Array:
			record(u.Elem())
		case *types.Chan:
			record(u.Elem())
		case *types.Map:
			record(u.Key())
			record(u.Elem())
		case *types.Signature:
			for i := range u.Params().Len() {
				record(u.Params().At(i).Type())
			}
			for i := range u.Results().Len() {
				record(u.Results().At(i).Type())
			}
		}
	}
	// add records the tie of src to the interface dst; conv: a value of src is
	// converted to dst here (not: a type assertion's target and its operand).
	add := func(src, dst types.Type, conv bool) {
		if src == nil || dst == nil || !types.IsInterface(dst) {
			return
		}
		if _, tup := src.(*types.Tuple); tup {
			return
		}
		if conv {
			record(src)
		}
		if !usable(dst) || !usable(src) {
			return
		}
		it, ok := dst.Underlying().(*types.Interface)
		if !ok || it.NumMethods() == 0 || types.Identical(src, dst) {
			return
		}
		k := [2]types.Type{src, dst}
		if !seen[k] {
			seen[k] = true
			names := make(map[string]bool, it.NumMethods())
			for m := range it.Methods() {
				names[m.Name()] = true
			}
			out = append(out, tie{src: src, dst: dst, names: names})
		}
	}
	asserted := func(t types.Type, pos token.Pos) {
		if t == nil || !types.IsInterface(t) {
			return
		}
		it, ok := t.Underlying().(*types.Interface)
		if !ok || it.NumMethods() == 0 {
			return
		}
		names := make(map[string]bool, it.NumMethods())
		for m := range it.Methods() {
			names[m.Name()] = true
		}
		out = append(out, tie{dst: t, names: names, asserted: true, pos: pos})
	}
	mark := func(e ast.Expr, target types.Type) {
		if e == nil || target == nil {
			return
		}
		if t := info.TypeOf(e); t != nil {
			add(t, target, true)
		}
	}
	// markTuple marks the values of a call that returns several (or of a
	// comma-ok form), which flow to dsts one by one.
	markTuple := func(e ast.Expr, dsts []types.Type) {
		if e == nil {
			return
		}
		if tup, ok := info.TypeOf(e).(*types.Tuple); ok {
			for i := range min(tup.Len(), len(dsts)) {
				add(tup.At(i).Type(), dsts[i], true)
			}
		}
	}
	// Go's order, so that the list is the same for every run.
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var sigs []*types.Signature
		var walk func(n ast.Node) bool
		walk = func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if sig, ok := info.TypeOf(n.Name).(*types.Signature); ok && n.Body != nil {
					sigs = append(sigs, sig)
					ast.Inspect(n.Body, walk)
					sigs = sigs[:len(sigs)-1]
				}
				return false
			case *ast.FuncLit:
				if sig, ok := info.TypeOf(n).(*types.Signature); ok && n.Body != nil {
					sigs = append(sigs, sig)
					ast.Inspect(n.Body, walk)
					sigs = sigs[:len(sigs)-1]
				}
				return false
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) && n.Tok == token.ASSIGN {
					for i := range n.Rhs {
						mark(n.Rhs[i], info.TypeOf(n.Lhs[i]))
					}
				} else if len(n.Rhs) == 1 && n.Tok == token.ASSIGN {
					// a, err = f(); v, ok = m[k]; v, ok = <-ch
					dsts := make([]types.Type, len(n.Lhs))
					for i, l := range n.Lhs {
						dsts[i] = info.TypeOf(l)
					}
					markTuple(n.Rhs[0], dsts)
				}
			case *ast.ValueSpec:
				if n.Type != nil && len(n.Values) > 0 {
					for _, val := range n.Values {
						mark(val, info.TypeOf(n.Type))
					}
					if len(n.Values) == 1 && len(n.Names) > 1 { // var a, err T = f()
						dsts := make([]types.Type, len(n.Names))
						for i := range dsts {
							dsts[i] = info.TypeOf(n.Type)
						}
						markTuple(n.Values[0], dsts)
					}
				}
			case *ast.ReturnStmt:
				if len(sigs) > 0 {
					res := sigs[len(sigs)-1].Results()
					if res.Len() == len(n.Results) {
						for i, r := range n.Results {
							mark(r, res.At(i).Type())
						}
					} else if len(n.Results) == 1 && res.Len() > 1 { // return f()
						dsts := make([]types.Type, res.Len())
						for i := range dsts {
							dsts[i] = res.At(i).Type()
						}
						markTuple(n.Results[0], dsts)
					}
				}
			case *ast.SendStmt:
				if ch, ok := underlying(info.TypeOf(n.Chan)).(*types.Chan); ok {
					mark(n.Value, ch.Elem())
				}
			case *ast.CallExpr:
				if isErrorsAs(info, n) {
					// errors.As(err, &target) takes a value of the interface type
					// out of the chain of err, as a type assertion does.
					if p, ok := underlying(info.TypeOf(n.Args[1])).(*types.Pointer); ok {
						asserted(p.Elem(), n.Pos())
					}
				}
				if tv, ok := info.Types[n.Fun]; ok && tv.IsType() && len(n.Args) == 1 {
					mark(n.Args[0], tv.Type) // a conversion
				} else if ok && n.Fun != nil {
					if sig, ok := underlying(info.TypeOf(n.Fun)).(*types.Signature); ok {
						params := sig.Params()
						if len(n.Args) == 1 && params.Len() > 1 || len(n.Args) == 1 && sig.Variadic() {
							// h(f()): the values of f go to the parameters
							dsts := make([]types.Type, 0, params.Len())
							for i := range params.Len() {
								pt := params.At(i).Type()
								if sl, ok := pt.(*types.Slice); ok && sig.Variadic() && i == params.Len()-1 {
									pt = sl.Elem()
								}
								dsts = append(dsts, pt)
							}
							if tup, ok := info.TypeOf(n.Args[0]).(*types.Tuple); ok {
								for i := range tup.Len() {
									j := min(i, len(dsts)-1)
									if j >= 0 {
										add(tup.At(i).Type(), dsts[j], true)
									}
								}
							}
						}
						for i, a := range n.Args {
							switch {
							case i < params.Len()-1 || i == params.Len()-1 && !sig.Variadic():
								mark(a, params.At(i).Type())
							case params.Len() > 0 && sig.Variadic():
								if sl, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok && !n.Ellipsis.IsValid() {
									mark(a, sl.Elem())
								}
							}
						}
					}
				}
			case *ast.CompositeLit:
				switch t := underlying(info.TypeOf(n)).(type) {
				case *types.Slice:
					for _, el := range n.Elts {
						mark(litValue(el), t.Elem())
					}
				case *types.Array:
					for _, el := range n.Elts {
						mark(litValue(el), t.Elem())
					}
				case *types.Map:
					for _, el := range n.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							mark(kv.Value, t.Elem())
							mark(kv.Key, t.Key())
						}
					}
				case *types.Struct:
					for i, el := range n.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							if id, ok := kv.Key.(*ast.Ident); ok {
								if fv, ok := info.Uses[id].(*types.Var); ok {
									mark(kv.Value, fv.Type())
								}
							}
						} else if i < t.NumFields() {
							mark(el, t.Field(i).Type())
						}
					}
				}
			case *ast.RangeStmt:
				if n.Tok == token.ASSIGN {
					var kt, vt types.Type
					switch u := underlying(info.TypeOf(n.X)).(type) {
					case *types.Slice:
						vt = u.Elem()
					case *types.Array:
						vt = u.Elem()
					case *types.Pointer:
						if a, ok := underlying(u.Elem()).(*types.Array); ok {
							vt = a.Elem()
						}
					case *types.Map:
						kt, vt = u.Key(), u.Elem()
					case *types.Chan:
						kt = u.Elem()
					case *types.Signature: // range over func
						if u.Params().Len() == 1 {
							if y, ok := underlying(u.Params().At(0).Type()).(*types.Signature); ok {
								if y.Params().Len() > 0 {
									kt = y.Params().At(0).Type()
								}
								if y.Params().Len() > 1 {
									vt = y.Params().At(1).Type()
								}
							}
						}
					}
					if n.Key != nil && kt != nil {
						add(kt, info.TypeOf(n.Key), true)
					}
					if n.Value != nil && vt != nil {
						add(vt, info.TypeOf(n.Value), true)
					}
				}
			case *ast.IndexExpr:
				if m, ok := underlying(info.TypeOf(n.X)).(*types.Map); ok {
					mark(n.Index, m.Key()) // m[t]
				}
			case *ast.SwitchStmt:
				// A comparison converts the operand that is not an interface for the
				// time of the comparison only: it ties the types (as gopls does) and
				// puts no value in an interface (the value of the operand is not
				// kept), so nothing is recorded as converted.
				if n.Tag != nil {
					tt := info.TypeOf(n.Tag)
					for _, c := range n.Body.List {
						if cc, ok := c.(*ast.CaseClause); ok {
							for _, e := range cc.List {
								if et := info.TypeOf(e); et != nil && tt != nil {
									add(et, tt, false)
									add(tt, et, false)
								}
							}
						}
					}
				}
			case *ast.BinaryExpr:
				if n.Op == token.EQL || n.Op == token.NEQ {
					if tx, ty := info.TypeOf(n.X), info.TypeOf(n.Y); tx != nil && ty != nil {
						add(tx, ty, false)
						add(ty, tx, false)
					}
				}
			case *ast.TypeAssertExpr:
				if n.Type != nil {
					add(info.TypeOf(n.Type), info.TypeOf(n.X), false)
					asserted(info.TypeOf(n.Type), n.Pos())
				}
			case *ast.TypeSwitchStmt:
				var x ast.Expr
				switch a := n.Assign.(type) {
				case *ast.ExprStmt:
					x = a.X
				case *ast.AssignStmt:
					if len(a.Rhs) == 1 {
						x = a.Rhs[0]
					}
				}
				if ta, ok := x.(*ast.TypeAssertExpr); ok {
					for _, c := range n.Body.List {
						for _, te := range c.(*ast.CaseClause).List {
							add(info.TypeOf(te), info.TypeOf(ta.X), false)
							asserted(info.TypeOf(te), te.Pos())
						}
					}
				}
			}
			return true
		}
		ast.Inspect(files[name], walk)
	}
	// An instantiation needs its type arguments to satisfy the constraints of
	// the type parameters: the constraint is an interface that the argument
	// implements (the methods of a constraint such as interface{ Area()
	// float64 } are tied to the argument's, as gopls does).
	ids := make([]*ast.Ident, 0, len(info.Instances))
	for id := range info.Instances {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Pos() < ids[j].Pos() })
	for _, id := range ids {
		inst := info.Instances[id]
		var tparams *types.TypeParamList
		switch o := info.Uses[id].(type) {
		case *types.Func:
			if sig, ok := o.Type().(*types.Signature); ok {
				tparams = sig.TypeParams()
			}
		case *types.TypeName:
			if n, ok := types.Unalias(o.Type()).(*types.Named); ok {
				tparams = n.TypeParams()
			}
		}
		if tparams == nil || inst.TypeArgs == nil {
			continue
		}
		for i := range min(tparams.Len(), inst.TypeArgs.Len()) {
			add(inst.TypeArgs.At(i), tparams.At(i).Constraint(), true)
		}
	}
	return &tieInfo{ties: out, converted: converted}
}

// foreignStd reports whether pkg is a package of the standard library, as seen
// from self (a module whose path has no dot, like "example" or "m", is its
// own: a package that shares the first element of self's path is not).
func foreignStd(pkg, self *types.Package) bool {
	if pkg == nil || !isStdPath(pkg.Path()) {
		return false
	}
	if self != nil {
		first := func(p string) string { f, _, _ := strings.Cut(p, "/"); return f }
		if first(pkg.Path()) == first(self.Path()) {
			return false
		}
	}
	return true
}

// isErrorsAs reports whether the call is errors.As(err, target).
func isErrorsAs(info *types.Info, call *ast.CallExpr) bool {
	if len(call.Args) != 2 {
		return false
	}
	var id *ast.Ident
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.Ident:
		id = f // a dot import
	default:
		return false
	}
	fn, ok := info.Uses[id].(*types.Func)
	return ok && fn.Name() == "As" && fn.Pkg() != nil && fn.Pkg().Path() == "errors"
}

// underlying returns the underlying type of t, or nil when t is nil (the
// type of an expression that did not check).
func underlying(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	return types.Unalias(t).Underlying()
}

// litValue returns the value of an element of a composite literal.
func litValue(e ast.Expr) ast.Expr {
	if kv, ok := e.(*ast.KeyValueExpr); ok {
		return kv.Value
	}
	return e
}

// otherModules returns the folders of the modules (gocpp.mod files) under the
// workspace folders, other than the module of the document, sorted. The walk
// is cached until a gocpp.mod file or a folder is created or deleted
// (importIndex.changed).
func (s *Server) otherModules(q *query) []string {
	cur := ""
	if q.lp.Module != nil {
		cur = q.lp.Module.Dir
	}
	s.symbols.mu.Lock()
	roots := slices.Clone(s.symbols.roots)
	s.symbols.mu.Unlock()
	gen := s.comp.index.generation()
	s.modsMu.Lock()
	defer s.modsMu.Unlock()
	key := strings.Join(roots, "\x00")
	if s.mods.gen != gen || s.mods.key != key || s.mods.dirs == nil {
		s.mods.gen, s.mods.key = gen, key
		s.mods.dirs = findModules(roots)
	}
	var out []string
	for _, d := range s.mods.dirs {
		if d != cur {
			out = append(out, d)
		}
	}
	return out
}

// findModules returns the folders with a gocpp.mod under the roots (not
// looking into hidden folders, testdata and the folders of the go command's
// kind, and at most maxIndexDirs folders in all).
func findModules(roots []string) []string {
	seen := make(map[string]bool)
	dirs := 0
	var visit func(dir string)
	visit = func(dir string) {
		if dirs >= maxIndexDirs || seen[dir] {
			return
		}
		seen[dir] = true
		dirs++
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			n := e.Name()
			switch {
			case e.IsDir():
				if !strings.HasPrefix(n, ".") && !strings.HasPrefix(n, "_") && n != "testdata" && n != "node_modules" {
					visit(filepath.Join(dir, n))
				}
			case n == "gocpp.mod":
				seen["mod:"+dir] = true
			}
		}
	}
	for _, root := range roots {
		visit(root)
	}
	var mods []string
	for k := range seen {
		if d, ok := strings.CutPrefix(k, "mod:"); ok {
			mods = append(mods, d)
		}
	}
	sort.Strings(mods)
	if mods == nil {
		mods = []string{}
	}
	return mods
}

// warnModules tells the user, once for each set of modules and request kind
// (always for a rename), that the search covers the module of the document
// only, when the workspace has other modules.
func (s *Server) warnModules(e env, what string, q *query, sym *symbol, every bool) {
	if sym.scope == scopeLocal || sym.scope == scopeFamily && s.wideFamily || e.notify == nil {
		return
	}
	others := s.otherModules(q)
	if len(others) == 0 {
		return
	}
	cur := "this module"
	if q.lp.Module != nil {
		cur = "the module in " + q.lp.Module.Dir
	}
	msg := fmt.Sprintf("gocpp %s: only %s is searched. The workspace has other modules, whose uses of this name are not included: %s.", what, cur, strings.Join(others[:min(3, len(others))], ", "))
	if len(others) > 3 {
		msg += ", ..."
	}
	if !every {
		key := what + "\x00" + msg
		s.modsMu.Lock()
		told := s.modsTold[key]
		if s.modsTold == nil {
			s.modsTold = make(map[string]bool)
		}
		s.modsTold[key] = true
		s.modsMu.Unlock()
		if told {
			return
		}
	}
	if err := e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Warning, Message: msg}); err != nil {
		e.log.Printf("window/showMessage: %v", err)
	}
}

// stdMethods are the methods of well-known standard interfaces: a method
// renamed to one of these names, with that signature, can make a type satisfy
// the interface, which changes what the standard library does with its values
// without a compile-time error (fmt calls String and Error, io.Copy calls
// Read and Write, sort.Sort calls Len, Less and Swap).
var stdMethods = map[string]struct {
	sig   string   // the signature, without the name
	iface string   // the interface that it can complete
	with  []string // the other methods that the type must have
	what  string   // what changes
}{
	"String": {"func() string", "fmt.Stringer", nil, "fmt prints the value with it"},
	"Error":  {"func() string", "error", nil, "the type can be used as an error, and fmt prints it with Error"},
	"Read":   {"func(p []byte) (n int, err error)", "io.Reader", nil, "io functions read from the value"},
	"Write":  {"func(p []byte) (n int, err error)", "io.Writer", nil, "io functions and fmt.Fprint write to the value"},
	"Close":  {"func() error", "io.Closer", nil, "code that closes an io.Closer closes the value"},
	"Len":    {"func() int", "sort.Interface", []string{"Less", "Swap"}, "sort.Sort accepts the value"},
	"Less":   {"func(i int, j int) bool", "sort.Interface", []string{"Len", "Swap"}, "sort.Sort accepts the value"},
	"Swap":   {"func(i int, j int)", "sort.Interface", []string{"Len", "Less"}, "sort.Sort accepts the value"},
}

// errorsProtocol says what calls the method fn by name when the standard
// library does, through an interface that is written in its code and has no
// name that a search could find: the Unwrap, Is and As methods of an error
// (errors.Is, errors.As and errors.Unwrap), as a predicate: "errors.Is calls it".
// It returns "" for any other method.
func errorsProtocol(fn *types.Func) string {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || types.IsInterface(recvBase(fn)) {
		return ""
	}
	typ := func(t *types.Var) string {
		return types.TypeString(t.Type(), func(*types.Package) string { return "" })
	}
	params, results := sig.Params(), sig.Results()
	switch fn.Name() {
	case "Unwrap":
		if params.Len() == 0 && results.Len() == 1 && (typ(results.At(0)) == "error" || typ(results.At(0)) == "[]error") {
			return "errors.Is, errors.As and errors.Unwrap call it"
		}
	case "Is":
		if params.Len() == 1 && results.Len() == 1 && typ(params.At(0)) == "error" && typ(results.At(0)) == "bool" {
			return "errors.Is calls it"
		}
	case "As":
		if params.Len() == 1 && results.Len() == 1 && (typ(params.At(0)) == "any" || typ(params.At(0)) == "interface{}") && typ(results.At(0)) == "bool" {
			return "errors.As calls it"
		}
	}
	return ""
}

// newImplements warns the user, with a showMessage, about the methods that the
// rename gives the name and signature of a method of a well-known standard
// interface, so that their types now satisfy it: the program compiles, and
// behaves differently.
func (rn *renamer) newImplements() {
	std, ok := stdMethods[rn.newName]
	if !ok || rn.e.notify == nil {
		return
	}
	renamed := make(map[*types.Func]bool)
	for _, h := range rn.res.hits {
		if fn, ok := h.obj.(*types.Func); ok && h.def {
			renamed[origin(fn).(*types.Func)] = true
		}
	}
	seen := make(map[string]bool)
	for fn := range renamed {
		sig, ok := fn.Type().(*types.Signature)
		rt := recvBase(fn)
		if !ok || sig.Recv() == nil || rt == nil || types.IsInterface(rt) || fn.Name() == rn.newName {
			continue
		}
		plain := types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic())
		if !sigEqual(plain, std.sig) {
			continue
		}
		have := make(map[string]bool)
		ms := types.NewMethodSet(types.NewPointer(rt))
		for i := range ms.Len() {
			have[ms.At(i).Obj().Name()] = true
		}
		delete(have, fn.Name())
		ok = true
		for _, w := range std.with {
			ok = ok && have[w]
		}
		if !ok {
			continue
		}
		key := objectName(fn)
		if seen[key] {
			continue
		}
		seen[key] = true
		msg := fmt.Sprintf("gocpp rename: %s is now called %s and now implements %s: %s. This changes what the program does without a compile-time error.", objectName(fn), rn.newName, std.iface, std.what)
		if err := rn.e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Warning, Message: msg}); err != nil {
			rn.e.log.Printf("window/showMessage: %v", err)
		}
	}
}

// sigEqual reports whether the signature sig, written without parameter
// names, is the one in want (which may name them).
func sigEqual(sig *types.Signature, want string) bool {
	typesOnly := func(t *types.Tuple) []string {
		var out []string
		for i := range t.Len() {
			out = append(out, types.TypeString(t.At(i).Type(), func(p *types.Package) string { return p.Name() }))
		}
		return out
	}
	got := "func(" + strings.Join(typesOnly(sig.Params()), ",") + ")(" + strings.Join(typesOnly(sig.Results()), ",") + ")"
	return got == wantTypes(want)
}

// wantTypes turns "func(p []byte) (n int, err error)" into
// "func([]byte,int)(...)" form: the types only, comma-joined.
func wantTypes(sig string) string {
	e, err := parser.ParseExpr(sig)
	ft, ok := e.(*ast.FuncType)
	if err != nil || !ok {
		return ""
	}
	list := func(fl *ast.FieldList) string {
		var out []string
		if fl != nil {
			for _, f := range fl.List {
				for range max(1, len(f.Names)) {
					out = append(out, types.ExprString(f.Type))
				}
			}
		}
		return strings.Join(out, ",")
	}
	return "func(" + list(ft.Params) + ")(" + list(ft.Results) + ")"
}
