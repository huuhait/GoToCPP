package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"sort"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Composite literals with C/C++ types (Phase 04 README §2.5, step 4.6):
//
//	SDL.SDL_Rect{0, 0, w, h}           ::SDL_Rect{0, 0, w, h}   (C++'s braced initialization: an
//	                                   aggregate, a constructor or an initializer_list, C++ decides)
//	SDL.SDL_Rect{w: 640, h: f()}       [&] { ::SDL_Rect gp_t0{};
//	                                   static_assert(gp::cgo::field_ok<decltype(gp_t0.w)>);
//	                                   gp_t0.w = {640}; (the same for h) return gp_t0; }()
//	std.vector[int]{1, 2, 3}           ::std::vector<int64_t>{1, 2, 3}
//	std.vector[lib.Point]{{1, 2}}      ::std::vector<::Point>{{1, 2}}  (an inner literal without
//	                                   its type is a braced list: C++ decides what it is)
//	std.map[std.string, int]{"a": 1}   gp::cgo::map_t<::std::map<::std::string, int64_t>>{{"a", 1}}
//	                                   (map_t stops a type that is no map: std.vector[int]{2: 7})
//	lib.struct_point{1, 2}             std::type_identity_t<struct ::point>{1, 2}
//	[]C.int{1, 2}, [2]lib.Point{{1, 2}}, map[string]C.int{"a": 1}
//	                                   gp::slice_of<int>({1, 2}), gp::array<::Point, 2>{::Point{1,
//	                                   2}}, gp::map_of<gp::string, int>({{"a", 1}}) (gocpp's
//	                                   literals, with the C++ types of their type expressions)
//
// A keyed literal of a C/C++ type has one of two forms, by its keys. When every key is a name, the
// names are fields, set by assignments in the literal's order (Go's order of evaluation, also of
// the calls in the values): GCC rejects designated initializers out of declaration order, which
// opaque mode doesn't know. gp::cgo::field_ok stops an array field with a clear message (an array
// can't be assigned; Phase 12 writes .w = 640 and allows them). When no key is a name, the
// literal is a C++ map's ({"a": 1}): its entries are the pairs of an initializer_list, in order
// (a key in a variable is written as a pair, std.map[std.string, int]{{k, 1}}). The values are
// braced, {v}, so that C++'s narrowing rules apply as in a positional literal (g++ makes
// narrowing an error, as clang++ does: build.GeneratedFlags).
//
// The elements are C/C++ values: an untyped constant is a C literal (markCValues), nil is
// nullptr, and a typed gocpp value goes in by C++'s rules (gocpp doesn't know the fields' types;
// Phase 12 does). In a gocpp literal, an element whose type has C/C++ parts (P{d: d} with a field
// d lib.Point, []C.int{n}) follows cgo's rules (the guard, cguard.go: a typed gocpp number into
// a C.int is an error).
//
// &T{...} of a C/C++ type is gp::cgo::box_keep<T>([&]() -> T { return T{...}; }): a gp::cptr, a
// gocpp pointer that owns the new object, built in place in a box (step 4.10, cunsafe.go: cNew;
// the box of step 4.8) that also holds what the pointers in its members point to, and converts to
// the raw T* for C and C++ (borrowed). A *T of a C/C++ type that gocpp code declares is a gp::cptr
// too, so &T{...} flows into variables, fields, results and appends.

// A litForm is the form of a composite literal of a C/C++ type: positional, keyed by fields, or
// keyed by the keys of a C++ map.
type litForm int

const (
	litPositional litForm = iota
	litFields
	litPairs
	litMixed // both names and other keys, or keyed and positional elements: an error
)

// cLitForm returns the form of the literal e of a C/C++ type.
func cLitForm(e *ast.CompositeLit) litForm {
	names, others, plain := 0, 0, 0
	for _, elt := range e.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		switch {
		case !ok:
			plain++
		case isIdent(kv.Key):
			names++
		default:
			others++
		}
	}
	switch {
	case names+others == 0:
		return litPositional
	case plain > 0 || names > 0 && others > 0:
		return litMixed
	case names > 0:
		return litFields
	}
	return litPairs
}

// isIdent reports whether e is a name (not in parentheses).
func isIdent(e ast.Expr) bool {
	_, ok := e.(*ast.Ident)
	return ok
}

// elidedTypes returns the type expressions of the composite literals of the package whose type
// is left out ({1, 2} in []lib.Point{{1, 2}}: lib.Point), from the type expression of the literal
// around them; nil for one inside a literal of a C/C++ type, whose elements' types gocpp doesn't
// know (std.vector[lib.Point]{{1, 2}}: C++ decides).
func (g *gen) elidedTypes() map[*ast.CompositeLit]ast.Expr {
	if g.elided != nil {
		return g.elided
	}
	g.elided = make(map[*ast.CompositeLit]ast.Expr)
	for _, f := range g.pkg.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			te, known := g.litTypeExpr(lit)
			if s, ok := ast.Unparen(te).(*ast.StarExpr); ok {
				te = s.X // the elements of &T{...} left out ([]*T{{...}}): T's
			}
			var key, value ast.Expr
			switch t := ast.Unparen(te).(type) {
			case *ast.ArrayType:
				value = t.Elt
			case *ast.MapType:
				key, value = t.Key, t.Value
			default:
				known = false // a C/C++ type (or a gocpp named type: go/types knows its parts)
			}
			set := func(x, te ast.Expr) {
				if inner, ok := x.(*ast.CompositeLit); ok && inner.Type == nil {
					if known {
						g.elided[inner] = te
					} else {
						g.elided[inner] = nil
					}
				}
			}
			for _, elt := range lit.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					set(kv.Key, key)
					elt = kv.Value
				}
				set(elt, value)
			}
			return true
		})
	}
	return g.elided
}

// litTypeExpr returns the type expression of the composite literal e: its own, or the one that
// the literal around it gives an element whose type is left out (*T for &T{...}); and false if
// it has none that gocpp knows (an element of a literal of a C/C++ type, elidedTypes).
func (g *gen) litTypeExpr(e *ast.CompositeLit) (ast.Expr, bool) {
	if e.Type != nil {
		return e.Type, true
	}
	te, ok := g.elidedTypes()[e]
	return te, ok && te != nil
}

// cxxLit reports whether e is a literal of a C/C++ type (lib.Point{1, 2}, std.vector[int]{1}),
// also one whose type is left out inside such a literal, and not a gocpp literal with C/C++
// parts ([]C.int{1}, P{d: d}).
func (g *gen) cxxLit(e *ast.CompositeLit) bool {
	if !g.pkg.Opaque.IsOpaque(e) {
		return false
	}
	te, ok := g.litTypeExpr(e)
	if !ok {
		return true
	}
	if s, ok := ast.Unparen(te).(*ast.StarExpr); ok {
		te = s.X
	}
	return g.isCType(te)
}

// cPairs reports whether e is a literal of a C/C++ type whose keys are a C++ map's
// (std.map[std.string, int]{"a": 1}): it is a list of pairs, {{k, v}, ...}, which C++ evaluates in
// order, keys too (order.go).
func (g *gen) cPairs(e *ast.CompositeLit) bool {
	return g.cxxLit(e) && cLitForm(e) == litPairs
}

// typeWith returns the C++ of the type t, whose type expression is te (nil if there is none):
// cxxType's if t is complete; else the type expression's, with the length of an array literal
// [...]C.int{1, 2} from t.
func (g *gen) typeWith(t types.Type, te ast.Expr, pos token.Pos) cppir.Type {
	if complete(t) {
		return g.cxxType(t, pos)
	}
	if at, ok := ast.Unparen(te).(*ast.ArrayType); ok {
		if _, ok := at.Len.(*ast.Ellipsis); ok {
			if a, ok := under(t).(*types.Array); ok {
				return arrayType(g.typeWith(a.Elem(), at.Elt, pos), a.Len())
			}
		}
	}
	if te == nil {
		g.unsupported(pos, "this use of a C/C++ type (a later step of Phase 04)")
		return typeName("gp_unsupported")
	}
	return g.typeExpr(te)
}

// litPartType returns the C++ of the type of a part of the composite literal e of the gocpp type
// t (with C/C++ parts; the T of &T{...} left out): of t itself (part == nil), or of its elements'
// or keys' type, part selecting it from t and from e's type expression.
func (g *gen) litPartType(e *ast.CompositeLit, t types.Type, part func(types.Type, ast.Expr) (types.Type, ast.Expr)) cppir.Type {
	te := g.litElemTypeExpr(e)
	if part != nil {
		t, te = part(t, te)
	}
	return g.typeWith(t, te, e.Lbrace)
}

// elemOf and keyOf are parts for litPartType: the element type, and the key type of a map.
func elemOf(t types.Type, te ast.Expr) (types.Type, ast.Expr) {
	var x ast.Expr
	switch y := ast.Unparen(te).(type) {
	case *ast.ArrayType:
		x = y.Elt
	case *ast.MapType:
		x = y.Value
	}
	switch u := under(t).(type) {
	case *types.Slice:
		return u.Elem(), x
	case *types.Array:
		return u.Elem(), x
	case *types.Map:
		return u.Elem(), x
	}
	return t, nil
}

func keyOf(t types.Type, te ast.Expr) (types.Type, ast.Expr) {
	var x ast.Expr
	if m, ok := ast.Unparen(te).(*ast.MapType); ok {
		x = m.Key
	}
	if m, ok := under(t).(*types.Map); ok {
		return m.Key(), x
	}
	return t, nil
}

// pkgVarLit returns the composite literal with a C/C++ type, also a gocpp one with C/C++ parts,
// that the package variable v without a type is declared from (var origin = lib.Vec{y: 1}), or
// nil.
func (g *gen) pkgVarLit(v *types.Var) *ast.CompositeLit {
	if v.Parent() != g.pkg.Types.Scope() {
		return nil
	}
	for _, init := range g.info.InitOrder {
		if len(init.Lhs) == 1 && init.Lhs[0] == v {
			if lit, ok := ast.Unparen(init.Rhs).(*ast.CompositeLit); ok && lit.Type != nil && g.pkg.Opaque.IsOpaque(lit) {
				return lit
			}
			return nil
		}
	}
	return nil
}

// cLit returns the C++ of the literal e of a C/C++ type (cxxLit).
func (fg *funcGen) cLit(e *ast.CompositeLit) cppir.Expr {
	g := fg.g
	te, known := g.litTypeExpr(e)
	var decl, typ cppir.Type // the C++ type, and as written before a braced list
	if known {
		decl = g.typeExpr(te)
		typ = litCxxType(decl)
	}
	elem := func(x ast.Expr) cppir.Expr { return fg.cMoved(x, fg.cValueOf(x)) }
	switch cLitForm(e) {
	case litPositional:
		elems := make([]cppir.Expr, len(e.Elts))
		for i, x := range e.Elts {
			elems[i] = elem(x)
		}
		return &cppir.InitList{Type: typ, Elems: elems}
	case litPairs:
		if typ == nil {
			break // reported by the guard (cLitOK)
		}
		// gp::cgo::map_t<T>{{k, v}, ...}: map_t's static_assert stops a type without mapped_type
		// (std.vector[int]{2: 7} is Go's index key, which a braced list can't say).
		elems := make([]cppir.Expr, len(e.Elts))
		for i, x := range e.Elts {
			kv := x.(*ast.KeyValueExpr)
			elems[i] = &cppir.InitList{Elems: []cppir.Expr{elem(kv.Key), elem(kv.Value)}}
		}
		return &cppir.InitList{Type: &cppir.TypeName{Name: "gp::cgo::map_t", Args: []cppir.Node{decl}}, Elems: elems}
	case litFields:
		if typ == nil {
			break // reported by the guard (cLitOK)
		}
		// [&] { T gp_t0{}; static_assert(gp::cgo::field_ok<decltype(gp_t0.w)>); gp_t0.w = {640};
		// ...; return gp_t0; }(): the stores in the literal's order. field_ok stops an array field
		// with a clear message; decltype names the field's type without a reference to it, so a
		// bit-field works too.
		if x := fg.typedFieldsLit(e, typ); x != nil {
			return x // typed mode: a designated initializer in declaration order (typedlit.go)
		}
		t := fg.names.temp("t")
		body := &cppir.Block{Inline: true}
		body.Stmts = append(body.Stmts, varDecl(cppir.Pos{}, decl, t, &cppir.InitList{}))
		for _, x := range e.Elts {
			kv := x.(*ast.KeyValueExpr)
			name := kv.Key.(*ast.Ident)
			g.noteCName(name.Name, name.Pos())
			ft := g.cNode(typeName("decltype(" + t + "." + name.Name + ")"))
			check := call("static_assert", &cppir.Name{Name: "gp::cgo::field_ok", Args: []cppir.Node{ft}})
			field := g.cNode(&cppir.Member{X: cxxName(t), Name: name.Name}).(*cppir.Member)
			v := elem(kv.Value)
			if _, ok := ast.Unparen(kv.Value).(*ast.CompositeLit); !ok {
				v = &cppir.InitList{Elems: []cppir.Expr{v}} // {v}: C++'s narrowing rules
			}
			body.Stmts = append(body.Stmts, &cppir.ExprStmt{X: check}, &cppir.ExprStmt{X: &cppir.Assign{Op: "=", X: field, Y: v}})
		}
		body.Stmts = append(body.Stmts, &cppir.Return{X: cxxName(t)})
		return &cppir.Call{Fun: &cppir.Lambda{Default: "&", Body: body}}
	}
	g.errorf(e.Lbrace, "internal error: a literal of a C/C++ type that the guard let through (cguard.go: cLitOK)")
	return placeholder
}

// uniqueKeys reports whether the C/C++ type r is a map of the C++ standard library whose keys are
// unique: std::map or std::unordered_map.
func uniqueKeys(r cRef) bool {
	n, ok := r.x.(*cppir.Name)
	return ok && (n.Name == "::std::map" || n.Name == "::std::unordered_map")
}

// litCxxType returns the C++ type typ as it can be written before a braced list, T{...}: a type
// that is more than one name (struct ::point, unsigned int) goes through
// std::type_identity_t<T>, as C++'s functional notation needs a simple type.
func litCxxType(typ cppir.Type) cppir.Type {
	if tn, ok := typ.(*cppir.TypeName); ok {
		for _, r := range tn.Name {
			if r == ' ' {
				return &cppir.TypeName{Name: "std::type_identity_t", Args: []cppir.Node{typ}}
			}
		}
		return typ
	}
	return &cppir.TypeName{Name: "std::type_identity_t", Args: []cppir.Node{typ}}
}

// cLitOK reports whether lowering translates the opaque composite literal e (one of a C/C++
// type, or a gocpp literal with C/C++ parts: []C.int{1}), and reports why not if it doesn't.
func (g *gen) cLitOK(e *ast.CompositeLit) bool {
	te, known := g.litTypeExpr(e)
	if s, ok := ast.Unparen(te).(*ast.StarExpr); ok && known && g.isCType(s.X) {
		// []*lib.Point{{1, 2}}: &lib.Point{1, 2}.
		t := g.srcText(s.X)
		g.userError(e.Lbrace, "an element {...} of a literal of *%s is &%s{...} in Go, which gocpp doesn't write for a C/C++ type: write each element as &%s{...}", t, t, t)
		return false
	}
	if !g.cxxLit(e) {
		return true // a gocpp literal: go/types checked it
	}
	tname := "this C/C++ type"
	if known {
		tname = g.srcText(te)
		if sel, ok := ast.Unparen(te).(*ast.SelectorExpr); ok && g.isC(sel.X) && cBuiltins[sel.Sel.Name] != "" {
			g.userError(e.Lbrace, "invalid composite literal type %s", tname) // C.int{1}, as in Go
			return false
		}
	}
	switch cLitForm(e) {
	case litMixed:
		g.userError(e.Lbrace, "mixture of field:value, key:value and value elements in a literal of %s", tname)
		return false
	case litFields, litPairs:
		if !known {
			g.userError(e.Lbrace, "missing type in composite literal: a keyed literal in a literal of a C/C++ type needs its type (T{f: v})")
			return false
		}
	}
	switch cLitForm(e) {
	case litFields:
		seen := make(map[string]bool)
		for _, elt := range e.Elts {
			name := elt.(*ast.KeyValueExpr).Key.(*ast.Ident)
			if seen[name.Name] {
				g.userError(name.Pos(), "duplicate field name %s in struct literal", name.Name)
				return false
			}
			seen[name.Name] = true
		}
	case litPairs:
		// Go's error for a duplicate constant key, for the maps whose keys are unique (a
		// std::map keeps the first of two equal keys of a braced list, silently); a multimap
		// takes both.
		if r, ok := g.cRefOf(ast.Unparen(te)); !ok || !uniqueKeys(r) {
			break
		}
		seen := make(map[string]bool)
		for _, elt := range e.Elts {
			k := elt.(*ast.KeyValueExpr).Key
			v := g.info.Types[k].Value
			if v == nil {
				continue
			}
			if v.Kind() == constant.Float {
				v = constant.ToFloat(v) // (1.0 and 1 as float keys)
			}
			id := v.ExactString()
			if seen[id] {
				g.userError(k.Pos(), "duplicate key %s in map literal", g.srcText(k))
				return false
			}
			seen[id] = true
		}
	}
	return true
}

// litTarget returns where the element x of the composite literal lit goes (x is a positional
// element, or the key or the value of a keyed one): the type, the field if it sets a field of a
// gocpp struct, the type expression of an element or a key of a gocpp container with C/C++ parts,
// and the words of Go's messages ("struct literal"); and false if gocpp doesn't know it (an
// element of a literal of a C/C++ type: C++'s rules).
func (g *gen) litTarget(lit *ast.CompositeLit, x ast.Expr) (valueTarget, bool) {
	if g.cxxLit(lit) {
		return valueTarget{}, false
	}
	t := g.litType(lit)
	// The element that x is in, by its place (the elements are in source order: a large table
	// is not searched element by element for each of its elements).
	idx := sort.Search(len(lit.Elts), func(i int) bool { return lit.Elts[i].End() > x.Pos() })
	if idx == len(lit.Elts) || t == nil {
		return valueTarget{}, false
	}
	var key ast.Expr
	isKey := false
	if kv, ok := lit.Elts[idx].(*ast.KeyValueExpr); ok {
		switch x {
		case kv.Key:
			key, isKey = kv.Key, true
		case kv.Value:
			key = kv.Key
		default:
			return valueTarget{}, false
		}
	} else if lit.Elts[idx] != x {
		return valueTarget{}, false
	}
	switch u := under(t).(type) {
	case *types.Struct:
		if isKey {
			return valueTarget{}, false // a field name
		}
		i := idx
		if key != nil {
			i = fieldIndex(u, key.(*ast.Ident).Name)
		}
		if i < 0 || i >= u.NumFields() {
			return valueTarget{}, false
		}
		f := u.Field(i)
		return valueTarget{to: f.Type(), v: f.Origin(), context: "struct literal"}, true
	case *types.Slice, *types.Array:
		if isKey {
			return valueTarget{}, false // an index: a constant
		}
		et, ete := elemOf(t, g.litElemTypeExpr(lit))
		return valueTarget{to: et, texpr: ete, context: "array or slice literal"}, true
	case *types.Map:
		part := elemOf
		if isKey {
			part = keyOf
		}
		pt, pte := part(t, g.litElemTypeExpr(lit))
		return valueTarget{to: pt, texpr: pte, context: "map literal"}, true
	}
	return valueTarget{}, false
}

// litElemTypeExpr returns the type expression of the composite literal lit that its elements'
// types come from: T's of &T{...} left out, nil if there is none.
func (g *gen) litElemTypeExpr(lit *ast.CompositeLit) ast.Expr {
	te, _ := g.litTypeExpr(lit)
	if s, ok := ast.Unparen(te).(*ast.StarExpr); ok {
		return s.X
	}
	return te
}
