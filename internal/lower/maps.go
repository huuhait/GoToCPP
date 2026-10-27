package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
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
//	                            (control.go: rangeIter, over the table, from a random slot)
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
	if isTuple(fg.g.info.TypeOf(e)) || fg.commaOKs[e] {
		fun = "gp::lookup"
	}
	fg.g.noteMap(e.Lbrack, "read "+fun) // why.go
	return call(fun, fg.expr(e.X), fg.expr(e.Index))
}

// commaOK records that the read m[k] gives two values (bind), where go/types has no tuple type
// for it: a map of C/C++ values (map[string]C.int).
func (fg *funcGen) commaOK(e *ast.IndexExpr) {
	if fg.commaOKs == nil {
		fg.commaOKs = make(map[*ast.IndexExpr]bool)
	}
	fg.commaOKs[e] = true
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
	fg.g.noteMap(x.Lbrack, "write gp::ref") // why.go
	return call("gp::ref", fg.expr(x.X), fg.expr(x.Index))
}

// mapLit returns the C++ of the map literal e: gp::map_of<K, V>({{k1, v1}, ...}), whose braced
// list C++ evaluates in order, entry by entry, in gc's order (mapEntries). Elided types of
// composite keys and values (map[string][]int{"a": {1, 2}}) are go/types' to fill in.
func (fg *funcGen) mapLit(e *ast.CompositeLit) cppir.Expr {
	t := fg.g.litType(e)
	// The map type first, with its own checks (a key with C/C++ parts is not supported yet), then
	// its parts (map[string]C.int: clits.go).
	fg.g.litPartType(e, t, nil)
	key, elem := fg.g.litPartType(e, t, keyOf), fg.g.litPartType(e, t, elemOf)
	if m, ok := under(t).(*types.Map); ok {
		key = cMapKey(m.Key(), key)
	}
	var pairs []cppir.Expr
	fg.g.noteMap(e.Lbrace, "literal gp::map_of") // why.go
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
		// gc stores each entry as m[k] = v (walk/order.go: OMAPLIT), which evaluates the
		// panicking parts of v before those of k (safeMapRHS); the braced list is in order.
		in := fg.analyzeWith(&orderScope{seq: true, rank: rootRank(1), copy: fg.entryKey(x)}, []ast.Expr{kv.Key, kv.Value})
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

// entryKey returns the copy function (orderScope) of an entry of the map literal e, whose key
// is the expression 0: gc stores the entry as m[k] = v, with a key that it may copy first
// (orderKey).
func (fg *funcGen) entryKey(e *ast.CompositeLit) func(int, ast.Expr) bool {
	return func(i int, k ast.Expr) bool { return i == 0 && fg.orderKey(e, k) }
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
	lt := fg.g.litType(e)
	typ := fg.g.litPartType(e, lt, nil) // (first: its own checks, as in mapLit)
	key, elem := fg.g.litPartType(e, lt, keyOf), fg.g.litPartType(e, lt, elemOf)
	if m, ok := under(lt).(*types.Map); ok {
		key = cMapKey(m.Key(), key)
	}
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
	*pre = append(*pre, varDecl(cppir.Pos{}, typ, t, fg.g.wrapNamed(lt, start, e.Pos())))
	for _, kv := range dynamics {
		// The arguments of gp::set are not in order: the entry's own scope, as a store.
		*pre = append(*pre, fg.orderWith(&orderScope{rank: rootRank(1), copy: fg.entryKey(e)}, kv.Key, kv.Value)...)
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
	if fg.orderKey(x.X, x.Index) {
		// gc copies the key into a temporary in its place among the calls (orderKey).
		in := fg.analyze(false, []ast.Expr{x.Index})
		if eff := in.effect(); eff.any() {
			sc.items = append(sc.items, &orderItem{e: ast.Unparen(x.Index), unit: true, eff: eff, root: root, up: below, inner: []*orderScope{in}})
		} else {
			sc.calls = append(sc.calls, in.calls...)
		}
	} else {
		fg.collect(sc, x.Index, root, below)
	}
	sc.items = append(sc.items, &orderItem{e: x, root: root, up: up, eff: effect{read: true}})
}

// orderKey reports whether gc's order pass copies the key k of the map m (a read or a store)
// into a temporary in its place among the calls, like a call (walk/order.go: mapKeyTemp, from
// Go 1.25's source):
//   - a slow map call (mapfast is mapslow: the value is larger than 128 bytes, or the key's
//     algorithm is not AMEM32, AMEM64 or ASTRING: gcMapFast) takes the key's address, and
//     addrTemp copies a key that isn't addressable (gcAddressable); an addressable one (a
//     variable, an element of a slice, *p, p.f) is read where the map is, except a local that
//     SSA can hold (addrTemp copies it too), which a closure that gc inlines can change: ssaKey,
//     inplace.go's rule for fmt (divergence D25 where gc's inliner decides otherwise).
//   - a fast call takes the key by value: one of the call's own kind (an integer, a pointer, a
//     string, also of a named type) is never copied; an array or a struct of 4 or 8 bytes of
//     plain memory (or one with the alg of a string) goes "through memory" into a temporary,
//     always.
//
// A constant key, or a static composite literal, is read-only data: its place doesn't matter.
func (fg *funcGen) orderKey(m, k ast.Expr) bool {
	mt, ok := under(fg.g.info.TypeOf(m)).(*types.Map)
	if !ok || fg.staticLit(k) {
		return false
	}
	if !gcMapFast(mt) {
		return !fg.gcAddressable(k) || fg.ssaKey(k)
	}
	switch under(mt.Key()).(type) {
	case *types.Basic, *types.Pointer, *types.Chan:
		return false
	}
	return true
}

// gcMapFast reports whether gc uses a fast map call for the map type m (walk/walk.go: mapfast):
// its value is at most 128 bytes (abi.OldMapMaxElemBytes) and its key's algorithm is AMEM32,
// AMEM64 or ASTRING (reflectdata.AlgType).
func gcMapFast(m *types.Map) bool {
	if goSizes.Sizeof(m.Elem()) > 128 {
		return false
	}
	switch a, size := gcAlg(m.Key()); {
	case a == algString:
		return true
	case a == algMem:
		return size == 4 || size == 8
	}
	return false
}

// The algorithm kinds of gc's types/alg.go that gcMapFast needs; the others are algOther.
const (
	algMem = iota
	algString
	algOther
)

// gcAlg returns the equality algorithm of t as gc's types.CalcSize computes it, and t's size:
// plain memory for integers, bools, pointers and channels, and for arrays and structs of them
// without blank fields and padding; a one-field struct and a one-element array have the alg of
// their element; strings have their own; floats, complex numbers, interfaces and the rest are
// other.
//
// (reflectdata.AlgType also keeps AMEM for a type aligned less than the target's alignment and
// than its size, [2]int16 on a strict 32-bit target: never on amd64 and arm64, whose
// Arch.Alignment is 1; a 32-bit target would need it.)
func gcAlg(t types.Type) (int, int64) {
	size := goSizes.Sizeof(t)
	switch u := under(t).(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			return algString, size
		case u.Info()&(types.IsFloat|types.IsComplex) != 0:
			return algOther, size
		}
		return algMem, size
	case *types.Pointer, *types.Chan:
		return algMem, size
	case *types.Array:
		a, _ := gcAlg(u.Elem())
		switch {
		case a == algMem, u.Len() == 0:
			return algMem, size
		case u.Len() == 1:
			return a, size
		}
		return algOther, size
	case *types.Struct:
		if u.NumFields() == 1 && u.Field(0).Name() != "_" {
			a, _ := gcAlg(u.Field(0).Type())
			return a, size
		}
		fields := make([]*types.Var, u.NumFields())
		for i := range fields {
			fields[i] = u.Field(i)
		}
		offsets := goSizes.Offsetsof(fields)
		for i, f := range fields {
			end := size
			if i+1 < len(fields) {
				end = offsets[i+1]
			}
			a, fs := gcAlg(f.Type())
			if a != algMem || f.Name() == "_" || offsets[i]+fs != end {
				return algOther, size
			}
		}
		return algMem, size
	}
	return algOther, size
}

// gcAddressable reports whether gc's ir.IsAddressable says e is addressable: a variable, *p, an
// element of a slice or through a pointer to an array, a field through a pointer, and an element
// or a field of an addressable array or struct (not of a string, a map read or a call).
func (fg *funcGen) gcAddressable(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		_, ok := fg.g.info.Uses[x].(*types.Var)
		return ok
	case *ast.StarExpr:
		return true
	case *ast.IndexExpr:
		switch t := fg.g.info.TypeOf(x.X); {
		case isArray(t):
			return fg.gcAddressable(x.X)
		case isSlice(t), arrayPtr(t) != nil:
			return true
		}
	case *ast.SelectorExpr:
		if s := fg.g.info.Selections[x]; s != nil {
			return s.Kind() == types.FieldVal && (s.Indirect() || fg.gcAddressable(x.X))
		}
		_, ok := fg.g.info.Uses[x.Sel].(*types.Var) // pkg.V
		return ok
	}
	return false
}

// orderKeys returns the copy function (orderScope) for the expressions list that starts with
// the operands of the targets lhs (targetOperands): the key of a map entry that gc copies
// first (orderKey).
func (fg *funcGen) orderKeys(lhs []ast.Expr) func(int, ast.Expr) bool {
	keys := make(map[ast.Expr]bool)
	for _, e := range lhs {
		if x := fg.mapIndex(e); x != nil && ast.Unparen(e) == x && fg.orderKey(x.X, x.Index) {
			keys[x.Index] = true
		}
	}
	return func(_ int, e ast.Expr) bool { return keys[e] }
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
		var targs []cppir.Node
		if at, ok := ast.Unparen(arg).(*ast.MapType); ok && !complete(mt) {
			// map[string]C.int: the C/C++ parts from the type expression (ctypes.go).
			targs = []cppir.Node{cMapKey(mt.Key(), fg.g.typeExpr(at.Key)), fg.g.typeExpr(at.Value)}
		} else {
			targs = []cppir.Node{fg.g.cxxType(mt.Key(), arg.Pos()), fg.g.cxxType(mt.Elem(), arg.Pos())}
		}
		fg.g.noteMap(c.Pos(), "make gp::make_map") // why.go
		if len(c.Args) == 1 {
			return templateCall("gp::make_map", targs)
		}
		return templateCall("gp::make_map", targs, fg.intExpr(c.Args[1]))
	case "delete":
		fg.g.noteMap(c.Pos(), "delete gp::erase") // why.go
		return call("gp::erase", fg.expr(arg), fg.expr(c.Args[1]))
	case "clear":
		return call("gp::clear", fg.expr(arg))
	}
	// go/types rejects append and copy on a map.
	fg.g.errorf(c.Pos(), "internal error: %s of %s (a construct the checker should have rejected)", name, types.TypeString(fg.g.info.TypeOf(arg), types.RelativeTo(fg.g.pkg.Types)))
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
// the same m and k on both sides (gc's ir.SameSafeExpr, sameSafe: variables, constants, *p,
// indexes, fields, operators and simple conversions of them, such as (*p)[k], ss[i][k],
// t.Inner.V for t.V and m[k+1] of a number, but not m[s+"x"], which gc's OADDSTR makes unsafe),
// or nil. gc takes the entry once for it (walk/assign.go reuses the mapassign): it evaluates m
// and k first (the calls in them, then m and k themselves into temporaries unless they are names
// or constants, so a nil *p or an index out of range panics there), then the other arguments,
// then it takes the entry (inserting k) and appends to the slice that is there. The C++ does the
// same (mapPre, mapSimple), with the slice moved into gp::append.
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

// plainName reports whether e is a name to gc (ONAME, which its cheapExpr doesn't copy): an
// identifier, or a variable of another package, pkg.V (a selector in Go's syntax).
func (fg *funcGen) plainName(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		_, isVar := fg.g.qualifiedObj(x).(*types.Var)
		return isVar
	}
	return false
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
// same result and no effect but a panic (gc's ir.SameSafeExpr, ir/expr.go). It compares what gc
// sees after typecheck, not the source, so:
//   - a conversion that gc makes an OCONVNOP is looked through at the top of both sides first, so
//     m[int(k)] is the same expression as m[k] (stripConvNop with convNopExpr, gc's convertOp and
//     assignOp from typecheck/subr.go after tcConv: the identical and identical-underlying
//     conversions, the rounding conversions of two floats or two complex numbers excepted, the
//     integers of one SimType, pointers to unsafe.Pointer and back, nil to a nillable type; a
//     conversion with an interface operand is OCONVIFACE in gc except where assignOp makes it an
//     OCONVNOP, so convNopExpr stops at assignOp for those);
//   - a string + is an AddStringExpr (OADDSTR) by then, and SameSafeExpr has no case for it, so
//     m[s+"x"] is not safe while m[i+1] of a number is;
//   - a conversion that survives as OCONV is safe only if its result is a simple type (gc's
//     types.IsSimple: the integers, floats, complex and bool), so a conversion to a string, an
//     interface or a pointer is not;
//   - nil is safe (ONIL), and two constants are safe when they are equal.
//
// Then: a variable (the same one; gc's ONAME, which also covers a variable of another package,
// pkg.V), *x, x[i] (an index of a map too, and of a pointer to an array, whose implicit deref
// makes parr[i] and (*parr)[i] one expression: indexBase), x.f (gc's ODOT and ODOTPTR: the same
// field, see fieldPath for the promoted ones), the operators !x, ^x, +x, -x and x+y, x-y, x|y,
// x^y, x*y, x<<y, x>>y, x&y, x&^y, x/y, x%y (gc's ONOT, OBITNOT, OPLUS, ONEG and OADD, OSUB, OOR,
// OXOR, OMUL, OLSH, ORSH, OAND, OANDNOT, ODIV, OMOD) of such expressions, and a conversion of one.
// gc requires the two expressions to have one type first, and the same operation.
func (fg *funcGen) sameSafe(a, b ast.Expr) bool {
	a, b = fg.stripConvNop(a), fg.stripConvNop(b)
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
	case *ast.SelectorExpr:
		y, ok := b.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		sx, sy := fg.g.info.Selections[x], fg.g.info.Selections[y]
		if sx == nil || sy == nil {
			// pkg.V: the same variable of another Go package (a name to gc: plainName).
			v, isVar := fg.g.qualifiedObj(x).(*types.Var)
			return sx == nil && sy == nil && isVar && v == fg.g.qualifiedObj(y)
		}
		if sx.Kind() != types.FieldVal || sy.Kind() != types.FieldVal {
			return false
		}
		if bx, px, okx := fg.fieldPath(x); okx {
			if by, py, oky := fg.fieldPath(y); oky {
				return slices.Equal(px, py) && fg.sameSafe(bx, by)
			}
			return false
		}
		return sx.Obj() == sy.Obj() && slices.Equal(sx.Index(), sy.Index()) && fg.sameSafe(x.X, y.X)
	case *ast.IndexExpr:
		y, ok := b.(*ast.IndexExpr)
		return ok && fg.sameSafe(fg.indexBase(x.X), fg.indexBase(y.X)) &&
			fg.sameSafe(x.Index, y.Index)
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
			// A string + is an OADDSTR in gc and has no case there, whatever the other side:
			// the key would be read after the appended arguments.
			if x.Op == token.ADD && isString(ta.Type) {
				return false
			}
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

// indexBase returns the base expression of the index e as gc's SameSafeExpr sees it. typecheck's
// implicitstar (typecheck/typecheck.go, from tcIndex) wraps a base of pointer-to-array type in an
// implicit ODEREF, so parr[i] and (*parr)[i] are the same ODEREF(ONAME) chain there while the
// source nodes differ. The explicit star over a pointer to an array is dropped here, which leaves
// both spellings as the pointer expression.
func (fg *funcGen) indexBase(e ast.Expr) ast.Expr {
	e = ast.Unparen(e)
	if s, ok := e.(*ast.StarExpr); ok && isPtrToArray(fg.g.info.TypeOf(s.X)) {
		return ast.Unparen(s.X)
	}
	return e
}

// isPtrToArray reports whether t is a pointer to an array, gc's implicitstar condition (the
// pointer's element type has array underlying).
func isPtrToArray(t types.Type) bool {
	p, ok := under(t).(*types.Pointer)
	if !ok {
		return false
	}
	_, ok = under(p.Elem()).(*types.Array)
	return ok
}

// fieldPath returns the base expression of the field selector e and the fields that it selects,
// the innermost first: gc's chain of ODOT and ODOTPTR nodes, in which a promoted field (t.f of a
// field of the embedded Inner) is the explicit t.Inner.f. gc's SameSafeExpr compares such a
// chain node by node (l.Sel == r.Sel), so the two forms are one expression to it. The last
// result is false for an expression that gc does not make such a chain for: a field of another
// package (pkg.V, a name there), and a field of a type parameter, whose struct is not known.
func (fg *funcGen) fieldPath(e ast.Expr) (ast.Expr, []*types.Var, bool) {
	e = ast.Unparen(e)
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return e, nil, true
	}
	s := fg.g.info.Selections[sel]
	if s == nil {
		return e, nil, false // pkg.V
	}
	if s.Kind() != types.FieldVal {
		return e, nil, false
	}
	base, fields, ok := fg.fieldPath(sel.X)
	if !ok {
		return e, nil, false
	}
	recv := fg.g.info.TypeOf(sel.X)
	for _, i := range s.Index() {
		st, ok := under(derefType(recv)).(*types.Struct)
		if !ok {
			return e, nil, false
		}
		f := st.Field(i)
		fields = append(fields, f)
		recv = f.Type()
	}
	return base, fields, true
}

// derefType returns the type that t points to if t is a pointer type, or t itself.
func derefType(t types.Type) types.Type {
	if p, ok := under(t).(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// stripConvNop returns e without the conversions that gc makes an OCONVNOP, which
// ir.SameSafeExpr looks through at the top of both of its sides: int64(k) is k.
//
// There is no depth limit, as in gc (ir/expr.go SameSafeExpr loops while l.Op() == OCONVNOP):
// the loop ends on the real condition, since it continues only while e is a one-argument call of
// a type name that convNopExpr accepts, and every turn replaces e with that call's argument, a
// strictly smaller subtree. A cap would make int(int(...(k)...)) a different key from k above
// that depth, and silently lose the order.
func (fg *funcGen) stripConvNop(e ast.Expr) ast.Expr {
	for {
		e = ast.Unparen(e)
		c, ok := e.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			return e
		}
		tv, ok := fg.g.info.Types[c.Fun]
		if !ok || !tv.IsType() || !fg.convNopExpr(fg.g.info.TypeOf(c.Args[0]), tv.Type) {
			return e
		}
		e = c.Args[0]
	}
}

// convNopExpr reports whether gc makes the explicit conversion of a value of type from to type
// to an OCONVNOP: typecheck's convertOp, and then tcConv, which turns a conversion of a float or
// a complex number of one kind back into an OCONV (it rounds). convNop (panicid.go) covers the
// numeric rule (convertOp's rule 4, for the integers: a same-kind float conversion never
// survives tcConv); these are the rest of gc's set for a conversion with no interface operand:
//   - identical underlying types, struct tags ignored (rule 2; a defined type with the same
//     underlying type as the other, which assignOp only covers when one of them is unnamed);
//   - unnamed pointer types over types of identical underlying types with the tags ignored
//     (rule 3);
//   - a bidirectional channel to a channel of the same element type, when one of the two is
//     unnamed (assignOp rule 4: a one-directional channel does not convert back);
//   - nil to a nillable type (assignOp rule 5);
//   - a pointer or a uintptr to unsafe.Pointer and back (rules 8 and 9).
//
// A conversion with an interface on either side is narrower: convertOp stops after assignOp
// there ("the rules for interfaces are no different in conversions than assignments"), so an
// interface to another interface, or a value to an interface, is an OCONVIFACE (rule 3) and not
// an OCONVNOP, except where assignOp's own rules make it one: rule 1, identical types, and rule 2
// when the source is an empty interface (a non-empty interface to another with the same methods
// recomputes the itab, so it is not one).
func (fg *funcGen) convNopExpr(from, to types.Type) bool {
	if from == nil || to == nil {
		return false
	}
	if fb, ok := under(from).(*types.Basic); ok && fb.Kind() == types.UntypedNil {
		return isNillable(to)
	}
	if isInterface(from) || isInterface(to) {
		// gc's assignOp, which is all convertOp does for an interface operand: rule 1, and rule
		// 2 when the source is an empty interface.
		return types.Identical(from, to) ||
			types.Identical(from.Underlying(), to.Underlying()) && isEmptyInterface(from)
	}
	if convNop(from, to) {
		return true
	}
	// tcConv turns a no-op conversion between two floats or two complex numbers of one kind back
	// into an OCONV, because it rounds; gc's SameSafeExpr then applies its OCONV case to it.
	if types.IdenticalIgnoreTags(under(from), under(to)) && !rounds(from) {
		return true
	}
	fp, fok := under(from).(*types.Pointer)
	tp, tok := under(to).(*types.Pointer)
	if fok && tok && isUnnamed(from) && isUnnamed(to) {
		return types.IdenticalIgnoreTags(fp.Elem().Underlying(), tp.Elem().Underlying())
	}
	// A bidirectional channel to a channel of the same element type, one way only (assignOp rule
	// 4: neither Go nor gc converts a one-directional channel back to a bidirectional one), when
	// one of the two is unnamed.
	fc, fok := under(from).(*types.Chan)
	tc, tok := under(to).(*types.Chan)
	if fok && tok && fc.Dir() == types.SendRecv && types.Identical(fc.Elem(), tc.Elem()) &&
		(isUnnamed(from) || isUnnamed(to)) {
		return true
	}
	if isUnsafePointer(from) {
		return isPointer(to) || isUintptr(to)
	}
	if isUnsafePointer(to) {
		return isPointer(from) || isUintptr(from)
	}
	return false
}

// rounds reports whether a conversion between two types of one kind rounds, which gc's tcConv
// keeps as an OCONV: two floats or two complex numbers of the same kind (float32 to float32,
// complex64 to complex64).
func rounds(t types.Type) bool {
	b, ok := under(t).(*types.Basic)
	return ok && b.Info()&(types.IsFloat|types.IsComplex) != 0
}

// isUnnamed reports whether t has no type name: a type literal (a pointer, a slice, a map, a
// channel, a func type, an unnamed struct or interface) or one of the predeclared basic types, as
// opposed to a defined type or a type parameter. gc's Sym() == nil test is not the same set: the
// predeclared basic types do carry a Sym (defBasic in types/universe.go sets typ.obj), so
// Sym() == nil holds for compound types built with no object only. The test appears in convertOp's
// rule 3, the unnamed-pointer rule, and in assignOp's rules 2 and 4, which convertOp reaches
// through its own rule 1. This predicate is used for the pointer and channel rules alone, where a
// basic type never arrives, so the difference never decides: the callers below check for a pointer
// or a channel first, and gc's Sym() test sits behind the same check (convertOp rule 3 wants
// src.IsPtr() && dst.IsPtr(), assignOp rule 4 wants src.IsChan() && dst.IsChan()).
func isUnnamed(t types.Type) bool {
	switch types.Unalias(t).(type) {
	case *types.Named, *types.TypeParam:
		return false
	}
	return true
}

// isNillable reports whether a nil value can be converted to t (assignOp rule 5).
func isNillable(t types.Type) bool {
	switch under(t).(type) {
	case *types.Pointer, *types.Signature, *types.Map, *types.Chan, *types.Interface, *types.Slice:
		return true
	}
	return false
}

// isEmptyInterface reports whether t is an interface type with no methods, gc's
// IsEmptyInterface (a named one too: type E interface{}). The method set is completed first, so
// an interface that only embeds a non-empty one (interface{ I }) is not empty, as in gc's
// AllMethods.
func isEmptyInterface(t types.Type) bool {
	it, ok := under(t).(*types.Interface)
	return ok && it.Complete().NumMethods() == 0
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
			if !fg.plainName(e) || e == lhs.Index && wrappedKey(under(fg.g.info.TypeOf(lhs.X)).(*types.Map).Key()) {
				temp(e)
			}
		}
		// gc's mapAppend copies make([]T, n) of m[k] = append(m[k], make([]T, n)...) into a
		// temporary too, so the make stays (not gp::append_make): "makeslice" for a bad n.
		fg.keepMake[c] = true
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
				fg.g.noteMap(x.Lbrack, map[token.Token]string{token.QUO: "divide gp::set", token.REM: "remainder gp::set"}[op]) // why.go
				return call("gp::set", fg.expr(x.X), fg.expr(x.Index), fg.binaryOp(s.TokPos, op, x, y))
			}
			// A float, or a divisor that can't fail: compound.
		case token.SHL, token.SHR:
			if _, rt := fg.mapShift(s); rt {
				fg.g.noteMap(x.Lbrack, "shift gp::ref") // why.go
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
	fg.g.noteMap(x.Lbrack, "store gp::set") // why.go
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
	switch start {
	case "gp::range_map":
		return &cppir.Call{Fun: &cppir.Member{X: it, Name: "gp_key"}}
	case "gp::range_chan", "gp::range_chan_drop":
		return &cppir.Call{Fun: &cppir.Member{X: it, Name: "gp_v"}} // the element: taken out of the iterator
	}
	return &cppir.Member{X: it, Name: "gp_i"}
}

// cMapKey returns the C++ key type of a gocpp map whose Go key type is t and C++ key type key:
// gp::cgo::map_key<K> for a key with C/C++ parts (map[std.string]int, map[[2]std.string]int, a struct
// with a std.string field), which is K and checks at the map's line that gp::map can hash and compare
// it (interop.hpp: key_ok), else key.
func cMapKey(t types.Type, key cppir.Type) cppir.Type {
	if complete(t) && !cmpHasC(t) {
		return key
	}
	return &cppir.TypeName{Name: "gp::cgo::map_key", Args: []cppir.Node{key}}
}
