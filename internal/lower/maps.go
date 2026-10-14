package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Maps (Phase 03 README §2.3, step 3.7; the runtime is gocpp/map.hpp):
//
//	Go                          C++
//	map[K]V                     gp::map<K, V>
//	var m map[K]V, m = nil      gp::map<K, V> m{};  m = nullptr
//	m == nil, m != nil          m == nullptr, m != nullptr
//	make(map[K]V), (.., n)      gp::make_map<K, V>(), gp::make_map<K, V>(n)
//	map[K]V{k1: v1, k2: v2}     gp::map_of<K, V>({{k1, v1}, {k2, v2}}), in gc's order (mapEntries)
//	v := m[k]                   V v = gp::get(m, k);  (the zero value if k is missing; no insert)
//	v, ok := m[k]               auto gp_t0 = gp::lookup(m, k); V v = std::get<0>(gp_t0); ...
//	m[k] = v                    gp::set(m, k, v);  (a nil map panics)
//	m[k] += y, m[k]++           gp::ref(m, k) += y;  gp::ref(m, k)++;  (insert-or-get: mapSimple)
//	m[k] /= y, m[k] %= y        gp::set(m, k, gp::div(gp::get(m, k), y));  (y == 0 doesn't insert k)
//	m[k] <<= n, m[k] >>= n      gp::ref(m, k) = gp::shl(gp::ref(m, k), n);  (inserts k first)
//	m[k] = append(m[k], x)      gp::slice<T>& gp_r0 = gp::ref(m, k); gp_r0 = gp::append(gp_r0, x);
//	                            (gp_r0 moved into gp::append: mapAppend)
//	delete(m, k)                gp::erase(m, k)
//	len(m), clear(m)            gp::len(m), gp::clear(m)
//	for k, v := range m         for (auto gp_it0 = gp::range_map(m); gp::range_next(gp_it0);)
//	                            { K k = gp_it0.gp_key(); V v = gp_it0.gp_value(); ...
//	                            (control.go: rangeIter, over a snapshot of the entries)
//
// A map is a class type (gp::map holds an owner), so a call of a user function with a map
// argument is qualified (funcCallName), and a temporary is moved into its variable.
//
// Go's order of evaluation (order.go), as gc's: m[k] reads the table, which any call can change
// (also the table of a local map: another variable can share it), so it is read after the calls
// of its scope, like *p and s[i] (gc doesn't copy OINDEXMAP); len(m) and make are evaluated in
// their places among the calls. The key and the value of a store, m[k] = v, are evaluated before
// the entry is taken, and the value before the key is read (gc copies the value into a temporary
// first); gp::set gets all three as arguments, so the store comes after them. A map literal is
// evaluated in its place among the calls, entry by entry (collectMapLit).

// mapType returns gp::map<key, elem>, the C++ type of a map from key to elem.
func mapType(key, elem cppir.Type) cppir.Type {
	return &cppir.TypeName{Name: "gp::map", Args: []cppir.Node{key, elem}}
}

// isMap reports whether t is a map type (through names and aliases: type Set map[string]bool).
func isMap(t types.Type) bool {
	_, ok := under(t).(*types.Map)
	return ok
}

// mapRead returns the C++ of the read m[k]: gp::get(m, k), the value, or gp::lookup(m, k), the
// value and whether k is there, as a std::tuple, where m[k] gives two values (v, ok := m[k]:
// go/types records a tuple type for it). Neither inserts. The key converts to K (a parameter
// that is not deduced), so a constant needs no exact type.
func (fg *funcGen) mapRead(e *ast.IndexExpr) cppir.Expr {
	fun := "gp::get"
	if isTuple(fg.g.info.TypeOf(e)) {
		fun = "gp::lookup"
	}
	return call(fun, fg.expr(e.X), fg.expr(e.Index))
}

// mapIndex returns e as m[k] if it is an index expression on a map, or nil.
func (fg *funcGen) mapIndex(e ast.Expr) *ast.IndexExpr {
	x, ok := ast.Unparen(e).(*ast.IndexExpr)
	if !ok || !isMap(fg.g.info.TypeOf(x.X)) {
		return nil
	}
	return x
}

// mapRef returns gp::ref(m, k) for the map entry x: the value of k, inserted first as the zero
// value if it isn't there (a nil map panics).
func (fg *funcGen) mapRef(x *ast.IndexExpr) cppir.Expr {
	return call("gp::ref", fg.expr(x.X), fg.expr(x.Index))
}

// mapLit returns the C++ of the map literal e: gp::map_of<K, V>({{k1, v1}, ...}), whose braced
// list C++ evaluates in order, entry by entry, in gc's order (mapEntries). Elided types of
// composite keys and values (map[string][]int{"a": {1, 2}}) are go/types' to fill in.
func (fg *funcGen) mapLit(e *ast.CompositeLit) cppir.Expr {
	mt := under(fg.g.info.TypeOf(e)).(*types.Map)
	key, elem := fg.g.cxxType(mt.Key(), e.Pos()), fg.g.cxxType(mt.Elem(), e.Pos())
	var pairs []cppir.Expr
	for _, kv := range fg.mapEntries(e) {
		pairs = append(pairs, &cppir.InitList{Elems: []cppir.Expr{fg.expr(kv.Key), fg.expr(kv.Value)}})
	}
	return templateCall("gp::map_of", []cppir.Node{key, elem}, &cppir.InitList{Elems: pairs})
}

// mapEntries returns the entries of the map literal e in the order that gc stores them
// (walk/order.go, OMAPLIT): the static ones first, then the others in source order. The spec
// leaves the order open, but it can be seen when two keys are equal and only one of them is a
// constant: k := 0.0; map[float64]int{-k: 1, 0: 2} stores 0: 2 first and gives {-0: 1}.
func (fg *funcGen) mapEntries(e *ast.CompositeLit) []*ast.KeyValueExpr {
	statics, dynamics := fg.splitEntries(e)
	return append(statics, dynamics...)
}

// splitEntries returns the static entries of the map literal e (gc's isStaticCompositeLiteral:
// the key and the value are constants, nil or array or struct literals of static elements) and
// the others, each in source order.
func (fg *funcGen) splitEntries(e *ast.CompositeLit) (statics, dynamics []*ast.KeyValueExpr) {
	for _, elt := range e.Elts {
		kv := elt.(*ast.KeyValueExpr)
		if fg.staticLit(kv.Key) && fg.staticLit(kv.Value) {
			statics = append(statics, kv)
		} else {
			dynamics = append(dynamics, kv)
		}
	}
	return statics, dynamics
}

// staticLit reports whether x is static for gc (walk/complit.go: isStaticCompositeLiteral): a
// constant, nil, or an array or struct literal whose elements are static (a slice or map
// literal never is, nor &T{...}).
func (fg *funcGen) staticLit(x ast.Expr) bool {
	x = ast.Unparen(x)
	if tv := fg.g.info.Types[x]; tv.Value != nil || tv.IsNil() {
		return true
	}
	lit, ok := x.(*ast.CompositeLit)
	if t := fg.g.info.TypeOf(lit); !ok || !isArray(t) && !isStruct(t) {
		return false
	}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			elt = kv.Value
		}
		if !fg.staticLit(elt) {
			return false
		}
	}
	return true
}

// collectMapLit adds the items of the map literal x to the scope (order.go). gc stores the
// entries that are not static one by one, each like an assignment m[k] = v, so each entry is
// evaluated completely before the next one: the literal is one unit, evaluated in its place
// among the calls, with a scope for each entry that is not static, its key and value (C++
// evaluates the braced pair {k, v} in order). If an entry needs temporaries of its own, the
// literal goes into a temporary entry by entry (emitMapLit): the entries' temporaries in front
// of the braced list would run before the entries before them.
func (fg *funcGen) collectMapLit(sc *orderScope, x *ast.CompositeLit, root int, up *orderStep) {
	_, dynamics := fg.splitEntries(x)
	var inner []*orderScope
	var eff effect
	force := false
	for _, kv := range dynamics {
		in := fg.analyze(true, []ast.Expr{kv.Key, kv.Value})
		inner = append(inner, in)
		eff = eff.or(in.effect())
		force = force || in.pre
	}
	if !eff.any() {
		for _, in := range inner {
			sc.calls = append(sc.calls, in.calls...)
		}
		return
	}
	sc.items = append(sc.items, &orderItem{e: x, unit: true, eff: eff, root: root, up: up, inner: inner, force: force})
}

// emitMapLit writes the temporary of the map literal e, a unit of the order (order.go:
// emitItem). Without temporaries in its entries, it is the braced list, auto gp_t0 =
// gp::map_of<K, V>({...});. With them, it is gc's form: the static entries first, then each
// other entry with its own order, as the assignment gp::set(gp_t0, k, v):
//
//	gp::map<K, V> gp_t0 = gp::map_of<K, V>({{1, 2}}); auto gp_t1 = f(); gp::set(gp_t0, x, gp_t1);
func (fg *funcGen) emitMapLit(it *orderItem, e *ast.CompositeLit, pre *[]cppir.Stmt) {
	entries := false
	for _, in := range it.inner {
		entries = entries || in.pre
	}
	t := fg.names.temp("t")
	if !entries {
		for _, in := range it.inner {
			fg.emitScope(in, pre)
		}
		*pre = append(*pre, varDecl(cppir.Pos{}, typeName("auto"), t, fg.expr(e)))
		fg.temps[e] = t
		return
	}
	mt := under(fg.g.info.TypeOf(e)).(*types.Map)
	key, elem := fg.g.cxxType(mt.Key(), e.Pos()), fg.g.cxxType(mt.Elem(), e.Pos())
	statics, dynamics := fg.splitEntries(e)
	var start cppir.Expr
	if len(statics) == 0 {
		start = templateCall("gp::make_map", []cppir.Node{key, elem}, lit(strconv.Itoa(len(dynamics))))
	} else {
		var pairs []cppir.Expr
		for _, kv := range statics {
			pairs = append(pairs, &cppir.InitList{Elems: []cppir.Expr{fg.expr(kv.Key), fg.expr(kv.Value)}})
		}
		start = templateCall("gp::map_of", []cppir.Node{key, elem}, &cppir.InitList{Elems: pairs})
	}
	// A literal of a named map type (step 3.9) is one in its temporary of that type.
	*pre = append(*pre, varDecl(cppir.Pos{}, fg.g.cxxType(fg.g.info.TypeOf(e), e.Pos()), t, fg.g.wrapNamed(fg.g.info.TypeOf(e), start, e.Pos())))
	for _, kv := range dynamics {
		// The arguments of gp::set are not in order: the entry's own scope, as a store.
		*pre = append(*pre, fg.order(false, kv.Key, kv.Value)...)
		*pre = append(*pre, &cppir.ExprStmt{X: call("gp::set", cxxName(t), fg.expr(kv.Key), fg.expr(kv.Value))})
	}
	fg.temps[e] = t
}

// collectMapIndex adds the items of m[k] to the scope (order.go): those of m and k, and the read
// of the table, after the calls of its scope. A read whose key is string(b) of a []byte is
// evaluated in its place among the calls instead, as gc does (walk/order.go: OINDEXMAP with
// mapKeyReplaceStrConv copies it there), unless m[k] is the target of op= or ++ (markStores).
func (fg *funcGen) collectMapIndex(sc *orderScope, x *ast.IndexExpr, root int, up *orderStep) {
	if !fg.stores[x] && fg.bytesKey(x.Index) {
		in := fg.analyze(false, []ast.Expr{x.X, x.Index})
		sc.items = append(sc.items, &orderItem{e: x, unit: true, eff: in.effect().or(effect{read: true}), root: root, up: up, inner: []*orderScope{in}})
		return
	}
	below := &orderStep{node: x, up: up}
	fg.collect(sc, x.X, root, below)
	fg.collect(sc, x.Index, root, below)
	sc.items = append(sc.items, &orderItem{e: x, root: root, up: up, eff: effect{read: true}})
}

// bytesKey reports whether the key e is string(b), a conversion of a []byte.
func (fg *funcGen) bytesKey(e ast.Expr) bool {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 1 || !fg.g.info.Types[c.Fun].IsType() || !isString(fg.g.info.TypeOf(c)) {
		return false
	}
	return elemKind(fg.g.info.TypeOf(c.Args[0])) == types.Uint8
}

// markStores marks the map entry that the simple statement s changes with op= or ++ (fg.stores):
// gc's Assigned, which collectMapIndex looks at.
func (fg *funcGen) markStores(s ast.Stmt) {
	var x ast.Expr
	switch s := s.(type) {
	case *ast.IncDecStmt:
		x = s.X
	case *ast.AssignStmt:
		if s.Tok != token.ASSIGN && s.Tok != token.DEFINE && len(s.Lhs) == 1 {
			x = s.Lhs[0]
		}
	}
	if m := fg.mapIndex(x); m != nil {
		fg.stores[m] = true
	}
}

// mapBuiltin returns the C++ of a call of make, delete or clear on a map. make's size hint
// only reserves room: Go ignores a negative one, and one too big to allocate (gp::make_map
// does the same), so make never panics for a map.
func (fg *funcGen) mapBuiltin(c *ast.CallExpr, name string) cppir.Expr {
	arg := c.Args[0]
	switch name {
	case "make":
		mt := under(fg.g.info.TypeOf(arg)).(*types.Map)
		targs := []cppir.Node{fg.g.cxxType(mt.Key(), arg.Pos()), fg.g.cxxType(mt.Elem(), arg.Pos())}
		if len(c.Args) == 1 {
			return templateCall("gp::make_map", targs)
		}
		return templateCall("gp::make_map", targs, fg.intExpr(c.Args[1]))
	case "delete":
		return call("gp::erase", fg.expr(arg), fg.expr(c.Args[1]))
	case "clear":
		return call("gp::clear", fg.expr(arg))
	}
	fg.g.unsupported(c.Pos(), name+" of "+types.TypeString(fg.g.info.TypeOf(arg), types.RelativeTo(fg.g.pkg.Types))+" (Phase 03)")
	return placeholder
}

// mapConversion returns the C++ of a conversion to a map type: map[K]V(nil) is gp::map<K, V>(),
// and a map of the same C++ type is itself (identical types: a named map type converts as its
// underlying type, namedConversion).
func (fg *funcGen) mapConversion(c *ast.CallExpr, to, from types.Type, x cppir.Expr) cppir.Expr {
	arg := c.Args[0]
	typ := fg.g.cxxType(to, c.Pos())
	switch {
	case fg.g.info.Types[arg].IsNil():
		return construct(typ)
	case isMap(from) && cppir.TypeString(typ) == cppir.TypeString(fg.g.cxxType(from, arg.Pos())):
		return x
	}
	qual := types.RelativeTo(fg.g.pkg.Types)
	fg.g.unsupported(c.Pos(), "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual))
	return placeholder
}

// mapAppend returns the append call of the assignment s if it is m[k] = append(m[k], ...), with
// the same m and k on both sides (gc's ir.SameSafeExpr: variables, constants, *p, indexes,
// operators and simple conversions of them, such as (*p)[k], ss[i][k] and m[k+1]), or nil. gc
// takes the entry once for it (walk/assign.go reuses the mapassign): it evaluates m and k first
// (the calls in them, then m and k themselves into temporaries unless they are names or
// constants, so a nil *p or an index out of range panics there), then the other arguments, then
// it takes the entry (inserting k) and appends to the slice that is there. The C++ does the same
// (mapPre, mapSimple), with the slice moved into gp::append.
func (fg *funcGen) mapAppend(s ast.Stmt) *ast.CallExpr {
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.ASSIGN || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		return nil
	}
	l := fg.mapIndex(a.Lhs[0])
	c, ok := ast.Unparen(a.Rhs[0]).(*ast.CallExpr)
	if l == nil || !ok || !fg.isBuiltin(c, "append") {
		return nil
	}
	r := fg.mapIndex(c.Args[0])
	if r == nil || !fg.sameSafe(l, r) {
		return nil
	}
	return c
}

// wrappedKey reports whether gc wraps a map key of type k in a conversion before a map call
// (walk/order.go: mapKeyTemp): for the fast map calls (walk/walk.go: mapfast), a key whose type
// is not the call's own key type, uint32, uint64, unsafe.Pointer or string, is converted to it:
// int, int32 (rune), int64, uint and uintptr keys, pointer keys, and named types over uint32,
// uint64 and string. In m[k] = append(m[k], ...), cheapExpr then copies the conversion, so such a
// key is read before the other arguments, while a key of another type (bool, int8, int16, uint8,
// uint16, floats, uint32, uint64, string) is read where the entry is taken, after them. The fast
// calls also need a value of at most 128 bytes, which a slice always is here. Arrays and structs
// (step 3.8): see the code. tests/diff/maps_append_key.gp checks this rule against the Go in use.
func wrappedKey(k types.Type) bool {
	if types.Identical(k, types.Typ[types.Uint32]) || types.Identical(k, types.Typ[types.Uint64]) ||
		types.Identical(k, types.Typ[types.String]) || types.Identical(k, types.Typ[types.UnsafePointer]) {
		return false
	}
	if isArray(k) {
		// An array goes through memory into a temporary (mapKeyTemp's unsafe cast) when its
		// algorithm is a fast one (reflectdata.AlgType): plain memory of 4 or 8 bytes, or one
		// element whose own algorithm is (a [1]string is a string).
		mem, size := goMemKey(k)
		return mem && (size == 4 || size == 8) || singleFast(k)
	}
	if isStruct(k) {
		// A struct is wrapped like an array when gc's algorithm for it is a fast one: a struct
		// of one field (not _) has that field's (types.CalcStructSize), and any other one is
		// plain memory of 4 or 8 bytes without blank fields and padding (goMemKey).
		if f := soleField(k); f != nil {
			return isString(f.Type()) || wrappedKey(f.Type()) || fastMem(f.Type())
		}
		return fastMem(k)
	}
	switch u := under(k).(type) {
	case *types.Pointer, *types.Chan:
		return true
	case *types.Basic:
		switch u.Kind() {
		case types.Int, types.Int32, types.Int64, types.Uint, types.Uintptr, types.Uint32, types.Uint64,
			types.String, types.UnsafePointer:
			return true
		}
	}
	return false
}

// goMemKey reports whether gc compares a key of type t as plain memory (types.AMEM: integers,
// bools, pointers, channels and arrays of them; not floats, strings or interfaces), and its size
// with Go's sizes (8-byte pointers and ints).
func goMemKey(t types.Type) (bool, int64) {
	switch u := under(t).(type) {
	case *types.Pointer, *types.Chan:
		return true, 8
	case *types.Basic:
		switch u.Kind() {
		case types.Bool, types.Int8, types.Uint8:
			return true, 1
		case types.Int16, types.Uint16:
			return true, 2
		case types.Int32, types.Uint32:
			return true, 4
		case types.Int, types.Int64, types.Uint, types.Uint64, types.Uintptr, types.UnsafePointer:
			return true, 8
		}
	case *types.Array:
		mem, size := goMemKey(u.Elem())
		return mem, size * u.Len()
	case *types.Struct:
		// gc's rule (types.CalcStructSize): one field (not _) is that field; more are plain
		// memory if every field is, and none is _ or followed by padding.
		if f := soleField(t); f != nil {
			return goMemKey(f.Type())
		}
		fields := make([]*types.Var, u.NumFields())
		for i := range fields {
			fields[i] = u.Field(i)
		}
		offsets := goSizes.Offsetsof(fields)
		size := goSizes.Sizeof(t)
		for i, f := range fields {
			mem, fsize := goMemKey(f.Type())
			end := size
			if i+1 < len(fields) {
				end = offsets[i+1]
			}
			if !mem || f.Name() == "_" || offsets[i]+fsize != end {
				return false, 0
			}
		}
		return true, size
	}
	return false, 0
}

// soleField returns the field of the struct type t if it has one field that is not _, or nil.
func soleField(t types.Type) *types.Var {
	if st, ok := under(t).(*types.Struct); ok && st.NumFields() == 1 && st.Field(0).Name() != "_" {
		return st.Field(0)
	}
	return nil
}

// fastMem reports whether gc hashes a key of type t as plain memory of 4 or 8 bytes (AMEM32,
// AMEM64), which its fast map calls take.
func fastMem(t types.Type) bool {
	mem, size := goMemKey(t)
	return mem && (size == 4 || size == 8)
}

// singleFast reports whether t is an array of one element (also nested: [1][1]T) whose
// element gc hashes with a fast algorithm of its own: a string, or plain memory of 4 or 8 bytes.
func singleFast(t types.Type) bool {
	a, ok := under(t).(*types.Array)
	if !ok || a.Len() != 1 {
		return false
	}
	if _, ok := under(a.Elem()).(*types.Array); ok {
		return singleFast(a.Elem())
	}
	if isString(a.Elem()) {
		return true
	}
	mem, size := goMemKey(a.Elem())
	return mem && (size == 4 || size == 8)
}

// sameSafe reports whether a and b are the same expression that can be evaluated twice with the
// same result and no effect but a panic (gc's ir.SameSafeExpr): the same variable or constant
// (or nil), and *x, x[i], an operator or a conversion to a basic type of such expressions.
func (fg *funcGen) sameSafe(a, b ast.Expr) bool {
	a, b = ast.Unparen(a), ast.Unparen(b)
	ta, tb := fg.g.info.Types[a], fg.g.info.Types[b]
	if ta.Type == nil || tb.Type == nil || !types.Identical(ta.Type, tb.Type) {
		return false
	}
	if ta.Value != nil || tb.Value != nil {
		return ta.Value != nil && tb.Value != nil && constant.Compare(ta.Value, token.EQL, tb.Value)
	}
	if ta.IsNil() || tb.IsNil() {
		return ta.IsNil() && tb.IsNil()
	}
	switch x := a.(type) {
	case *ast.Ident:
		y, ok := b.(*ast.Ident)
		v, isVar := fg.g.info.Uses[x].(*types.Var)
		return ok && isVar && v == fg.g.info.Uses[y]
	case *ast.StarExpr:
		y, ok := b.(*ast.StarExpr)
		return ok && fg.sameSafe(x.X, y.X)
	case *ast.IndexExpr:
		y, ok := b.(*ast.IndexExpr)
		return ok && fg.sameSafe(x.X, y.X) && fg.sameSafe(x.Index, y.Index)
	case *ast.UnaryExpr:
		y, ok := b.(*ast.UnaryExpr)
		switch x.Op {
		case token.NOT, token.XOR, token.ADD, token.SUB:
			return ok && x.Op == y.Op && fg.sameSafe(x.X, y.X)
		}
	case *ast.BinaryExpr:
		y, ok := b.(*ast.BinaryExpr)
		switch x.Op {
		case token.ADD, token.SUB, token.OR, token.XOR, token.MUL, token.SHL, token.SHR, token.AND,
			token.AND_NOT, token.QUO, token.REM:
			return ok && x.Op == y.Op && fg.sameSafe(x.X, y.X) && fg.sameSafe(x.Y, y.Y)
		}
	case *ast.CallExpr:
		y, ok := b.(*ast.CallExpr)
		if !ok || len(x.Args) != 1 || len(y.Args) != 1 || !fg.g.info.Types[x.Fun].IsType() ||
			!fg.g.info.Types[y.Fun].IsType() || underBasic(ta.Type) == nil || isString(ta.Type) {
			return false // only conversions to numbers and bools (gc: types.IsSimple)
		}
		return fg.sameSafe(x.Args[0], y.Args[0])
	}
	return false
}

// mapShift reports whether the simple statement s is m[k] <<= n or m[k] >>= n whose count is not
// a constant below the width of the value: the runtime's gp::shl or gp::shr (mapSimple).
func (fg *funcGen) mapShift(s ast.Stmt) (*ast.AssignStmt, bool) {
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.SHL_ASSIGN && a.Tok != token.SHR_ASSIGN || fg.mapIndex(a.Lhs[0]) == nil {
		return nil, false
	}
	if v := fg.constValue(a.Rhs[0]); v != nil {
		n, exact := constant.Uint64Val(constant.ToInt(v))
		if b := underBasic(fg.g.info.TypeOf(a.Lhs[0])); b != nil && exact && n < bitSize(b) {
			return a, false
		}
	}
	return a, true
}

// mapNeedsPre reports whether mapPre(s) may return statements (before s is ordered).
func (fg *funcGen) mapNeedsPre(s ast.Stmt) bool {
	if fg.mapAppend(s) != nil {
		return true
	}
	a, ok := fg.mapShift(s)
	return ok && !fg.inert(a.Rhs[0])
}

// mapPre returns the statements that the simple statement s needs after its order (order.go) and
// before its C++ expression (mapSimple), for the map entries that it changes in two steps:
//   - m[k] = append(m[k], x, ...): the statement is ordered here, as gc does (mapAppend): m and k,
//     then each argument x, which goes into a temporary if it is not inert, since gc evaluates
//     them before it takes the entry; and the entry is taken once, by reference:
//     gp::slice<T>& gp_r0 = gp::ref(m, k);. Nothing can delete k while gp_r0 is held: the
//     arguments are temporaries or inert (no call, no panic);
//   - m[k] <<= n with a count that gp::shl takes (mapShift): n goes into a temporary if it is
//     not inert, since gc evaluates it before it takes the entry (which then panics for a
//     negative n, after inserting k), and C++ doesn't order the arguments of gp::shl.
func (fg *funcGen) mapPre(s ast.Stmt) []cppir.Stmt {
	var pre []cppir.Stmt
	temp := func(e ast.Expr) {
		if _, ok := fg.temps[ast.Unparen(e)]; ok || fg.inert(e) {
			return
		}
		t := fg.names.temp("t")
		pre = append(pre, varDecl(cppir.Pos{}, typeName("auto"), t, fg.exactExpr(e)))
		fg.temps[ast.Unparen(e)] = t
	}
	if c := fg.mapAppend(s); c != nil {
		// gc's order (mapAppend): m and k, each into a temporary unless it is a name or a
		// constant (cheapExpr), then the other arguments, then the entry. A key that is a
		// variable goes into a temporary too when gc wraps it in a conversion first
		// (wrappedKey), which cheapExpr then copies.
		lhs := fg.mapIndex(s.(*ast.AssignStmt).Lhs[0])
		pre = append(pre, fg.order(false, lhs.X, lhs.Index)...)
		for _, e := range []ast.Expr{lhs.X, lhs.Index} {
			if fg.constValue(e) != nil || fg.g.info.Types[e].IsNil() {
				continue
			}
			if _, name := ast.Unparen(e).(*ast.Ident); !name || e == lhs.Index && wrappedKey(under(fg.g.info.TypeOf(lhs.X)).(*types.Map).Key()) {
				temp(e)
			}
		}
		pre = append(pre, fg.order(false, c.Args[1:]...)...)
		fg.ordered[c] = true // its arguments were ordered here, not as a unit (simpleScope)
		for _, x := range c.Args[1:] {
			temp(x)
		}
		r := fg.names.temp("r")
		typ := fg.g.cxxType(fg.g.info.TypeOf(lhs), lhs.Pos())
		pre = append(pre, varDecl(cppir.Pos{}, &cppir.Ref{Elem: typ}, r, fg.mapRef(lhs)))
		fg.temps[lhs], fg.temps[ast.Unparen(c.Args[0])] = r, r
		fg.moves[c.Args[0]] = true // gp::append(static_cast<gp::slice<T>&&>(gp_r0), ...)
		return pre
	}
	if a, ok := fg.mapShift(s); ok {
		temp(a.Rhs[0])
	}
	return pre
}

// mapSimple returns the C++ expression of the simple statement s if it changes a map entry with
// op=, ++ or -- (or is m[k] = append(m[k], ...), mapAppend), or nil. It runs after mapPre.
//   - m[k]++, m[k] += y, and -=, *=, &=, |=, ^=, &^=: C++'s compound assignment on the entry,
//     gp::ref(m, k) += y, which evaluates y first (C++17), as gc does, and takes the entry once.
//     It gives Go's results: the value converts back to the small integer types with Go's
//     wrapping (C++20), signed overflow wraps (-fwrapv), and gp::string has +=;
//   - /= and %= of integers: gc rewrites them to m[k] = m[k] / y, so a division by zero panics
//     before k is inserted: gp::set(m, k, gp::div(gp::get(m, k), y)) (gp::set's arguments come
//     first). A constant divisor other than 0 and -1 can't panic: gp::ref(m, k) /= 3;
//   - <<= and >>=: a constant count below the width is C++'s <<= (defined in C++20); any other
//     count gp::ref(m, k) = gp::shl(gp::ref(m, k), n), which takes the entry before the count's
//     check (a negative count panics after inserting k, as in Go), with n in a temporary
//     (mapPre).
//
// The calls in m and k went into temporaries (forceTarget), so the forms that name the entry
// twice evaluate m and k once, like x op= y does (stmt.go).
func (fg *funcGen) mapSimple(s ast.Stmt) cppir.Expr {
	if c := fg.mapAppend(s); c != nil {
		lhs := s.(*ast.AssignStmt).Lhs[0]
		return &cppir.Assign{Op: "=", X: fg.expr(lhs), Y: fg.expr(c)}
	}
	switch s := s.(type) {
	case *ast.IncDecStmt:
		if x := fg.mapIndex(s.X); x != nil {
			return &cppir.Postfix{X: fg.mapRef(x), Op: s.Tok.String()}
		}
	case *ast.AssignStmt:
		if s.Tok == token.ASSIGN || s.Tok == token.DEFINE || len(s.Lhs) != 1 {
			return nil
		}
		x := fg.mapIndex(s.Lhs[0])
		if x == nil {
			return nil
		}
		y := s.Rhs[0]
		op := s.Tok + (token.ADD - token.ADD_ASSIGN)
		b := underBasic(fg.g.info.TypeOf(x))
		switch op {
		case token.AND_NOT:
			return &cppir.Assign{Op: "&=", X: fg.mapRef(x), Y: &cppir.Unary{Op: "~", X: at(fg.expr(y), precUnary)}}
		case token.QUO, token.REM:
			v := fg.constValue(y)
			safe := v != nil && constant.Sign(v) != 0 && constant.Compare(v, token.NEQ, constant.MakeInt64(-1))
			if b != nil && b.Info()&types.IsInteger != 0 && !safe {
				return call("gp::set", fg.expr(x.X), fg.expr(x.Index), fg.binaryOp(s.TokPos, op, x, y))
			}
			// A float, or a divisor that can't fail: compound.
		case token.SHL, token.SHR:
			if _, rt := fg.mapShift(s); rt {
				helper := "gp::shl"
				if op == token.SHR {
					helper = "gp::shr"
				}
				return &cppir.Assign{Op: "=", X: fg.mapRef(x), Y: call(helper, fg.mapRef(x), fg.shiftCount(y))}
			}
		}
		return &cppir.Assign{Op: s.Tok.String(), X: fg.mapRef(x), Y: fg.expr(y)}
	}
	return nil
}

// mapTarget makes t, the target of an assignment to the map entry x, store through gp::set
// (target.assignExpr).
func (fg *funcGen) mapTarget(t *target, x *ast.IndexExpr) {
	t.mapX, t.mapK = fg.expr(x.X), fg.expr(x.Index)
}

// rangeMapVar changes *value, the key or the value of a range over a map with the iterator it
// (for s[f()], v = range m), when the targets' operands ops are not inert: gp_key() and
// gp_value() are references into the current entry, which a call in the operands can delete
// (or clear the map), so they must not be read after the operands. gc reads both after them
// from its own iterator. The key of an entry can't change: it is copied before the operands
// (the statement it returns, auto gp_t0 = gp_it0.gp_key();). The value is read after them with
// gp::range_value(gp_it0): its current value, as in Go, or the zero value if the entry is gone
// (Go reads its cleared slot; it may clear the key too, which gocpp doesn't). The other range
// loops can't dangle there: a string's iterator holds its rune by value, and a slice's iterator
// holds its own copy of the slice, which keeps the array alive (gp_v() reads the element after
// the operands, as gc does).
func (fg *funcGen) rangeMapVar(ops []ast.Expr, key bool, it cppir.Expr, value *cppir.Expr) []cppir.Stmt {
	inert := true
	for _, op := range ops {
		inert = inert && fg.inert(op)
	}
	if inert {
		return nil
	}
	// The calls in the operands go into temporaries first (fg.force), so that a store
	// gp::set(m, f(), gp::range_value(gp_it0)), whose arguments C++ doesn't order, can't read
	// the value before f runs.
	for _, op := range ops {
		ast.Inspect(op, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && !fg.g.info.Types[c.Fun].IsType() {
				fg.force[c] = true
			}
			return true
		})
	}
	switch {
	case key:
		t := fg.names.temp("t")
		stmt := varDecl(cppir.Pos{}, typeName("auto"), t, *value)
		*value = cxxName(t)
		return []cppir.Stmt{stmt}
	}
	*value = call("gp::range_value", it)
	return nil
}

// rangeKey returns the key of a range loop whose iterator it was made by start (control.go:
// rangeIter): gp_it0.gp_key() for a map (gp::range_map), the index gp_it0.gp_i for the others.
func rangeKey(start string, it cppir.Expr) cppir.Expr {
	if start == "gp::range_map" {
		return &cppir.Call{Fun: &cppir.Member{X: it, Name: "gp_key"}}
	}
	return &cppir.Member{X: it, Name: "gp_i"}
}
