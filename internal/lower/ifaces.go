package lower

import (
	"crypto/sha256"
	"encoding/hex"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Interface types (Phase 05 step 5.4; the runtime is gocpp/iface.hpp, docs/spec/cpp-mapping.md
// "Interfaces"):
//
//	type Shape interface{ Area() float64; Scale(k float64) }
//
//	struct Shape : gp::iface {                       (before the structs: no data members)
//	    using gp::iface::iface;
//	    static const gp::type_desc* gp_desc();
//	    double Area() const;
//	    void Scale(double a0) const;
//	};
//	...
//	inline const gp::imethod gp_im_Shape[] = {gp::imethod("Area", nullptr, "func() float64"), ...};
//	inline const gp::type_desc gp_td_Shape = gp::named<Shape>("main.Shape", "main", gp::shape_interface(gp_im_Shape));
//	inline const gp::type_desc* Shape::gp_desc() { return &gp_td_Shape; }
//	inline double Shape::Area() const { return gp::icall<double()>(*this, 0); }
//	inline void Shape::Scale(double a0) const { gp::icall<void(double)>(*this, 1, static_cast<double&&>(a0)); }
//
// One struct per interface type: a named one, and an unnamed one with methods
// (interface{ M() } is gp_anon_i followed by 16 hex digits of the SHA-256 of its identity text, so
// identical types are one struct). The empty interface is gp::any and error is gp::error (the
// runtime's). The slot of a method is its position in the interface's methods, sorted as the
// runtime's tables are (canon.go: sortedMethods). The wrappers are defined after all the
// structs and the descriptors, so an interface may return a struct that holds it.
//
// A call x.M(args) of an interface value is the wrapper's, x.M(args) (methods.go: methodCall).
// A conversion to an interface is gp::to_iface or gp::iface_cast (iconv.go).

// wrapTag is the attribute of the wrapper methods of the interface structs: it puts B7gp_wrap in
// the mangled name, by which the stack trace of a panic leaves the wrapper out (Go has no frame for
// it: runtime/src/trace.cpp, func_name).
const wrapTag = `[[gnu::abi_tag("gp_wrap")]]`

// An ifaceInfo is the C++ struct of one interface type.
type ifaceInfo struct {
	t       types.Type // the *types.Named, or the *types.Interface of an unnamed type
	it      *types.Interface
	name    string        // the C++ name
	goName  string        // Go's name of the type, as %T prints it
	methods []*types.Func // in the order of the slots
	named   *types.Named  // nil for an unnamed interface
	mnames  []string      // the C++ names of the methods
}

// ifaceTable holds the interface structs of the package.
type ifaceTable struct {
	list  []*ifaceInfo
	named map[*types.TypeName]*ifaceInfo
	anon  map[string]*ifaceInfo // by identity text
}

func (g *gen) itable() *ifaceTable {
	if g.ifaces == nil {
		g.ifaces = &ifaceTable{named: make(map[*types.TypeName]*ifaceInfo), anon: make(map[string]*ifaceInfo)}
	}
	return g.ifaces
}

// isInterface reports whether t is an interface type (through names and aliases), error and the
// empty interface included, not a type parameter.
func isInterface(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// isErrorType reports whether t is the predeclared type error.
func isErrorType(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() == nil && n.Obj().Name() == "error"
}

// isAnyType reports whether t is the unnamed empty interface (any, interface{}).
func isAnyType(t types.Type) bool {
	it, ok := types.Unalias(t).(*types.Interface)
	return ok && it.NumMethods() == 0 && it.IsMethodSet()
}

// isNilType reports whether t is the type of the untyped nil.
func isNilType(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.UntypedNil
}

// addIface adds the struct of the named interface type t (typeSpec), which the C++ calls name.
func (g *gen) addIface(t *types.Named, name string) *ifaceInfo {
	it := t.Underlying().(*types.Interface)
	info := g.newIface(t, it, name)
	info.named = t
	info.goName = g.canonName(t)
	g.itable().named[t.Obj()] = info
	return info
}

// newIface returns the struct of the interface type it (t is it, or the named type over it).
func (g *gen) newIface(t types.Type, it *types.Interface, name string) *ifaceInfo {
	info := &ifaceInfo{t: t, it: it, name: name, methods: sortedMethods(it)}
	for _, m := range info.methods {
		if !complete(m.Signature()) {
			g.unsupported(m.Pos(), "the interface method "+m.Name()+" has a C/C++ type in its signature (typed interop: Phase 12)")
		}
	}
	for _, m := range info.methods {
		info.mnames = append(info.mnames, g.methodName(name, m.Name()))
	}
	tab := g.itable()
	tab.list = append(tab.list, info)
	return info
}

// ifaceOf returns the struct of the interface type t (named, error and any excepted), or nil
// with an error recorded at pos.
func (g *gen) ifaceOf(t types.Type, pos token.Pos) *ifaceInfo {
	switch u := types.Unalias(t).(type) {
	case *types.Named:
		if info := g.itable().named[u.Obj()]; info != nil {
			return info
		}
		g.unsupported(pos, "the interface type "+g.canonName(u)+" (interface types of other packages come with Phase 06)")
		return nil
	case *types.Interface:
		if !u.IsMethodSet() {
			g.unsupported(pos, "constraint interfaces (Phase 07)")
			return nil
		}
		key := g.canonText(u)
		tab := g.itable()
		if info := tab.anon[key]; info != nil {
			return info
		}
		sum := sha256.Sum256([]byte(key))
		info := g.newIface(u, u, "gp_anon_i"+hex.EncodeToString(sum[:8]))
		info.goName = g.canonName(u)
		tab.anon[key] = info
		return info
	}
	g.unsupported(pos, "the interface type "+types.TypeString(t, nil))
	return nil
}

// ifaceType returns the C++ type of the interface type t: gp::any, gp::error or its struct.
func (g *gen) ifaceType(t types.Type, pos token.Pos) cppir.Type {
	switch {
	case isErrorType(t):
		return typeName("gp::error")
	case isAnyType(t):
		return typeName("gp::any")
	}
	info := g.ifaceOf(t, pos)
	if info == nil {
		return typeName("gp_unsupported")
	}
	if g.inTemplate > 0 {
		return typeName("::" + g.ns + "::" + info.name)
	}
	if td := g.tdecls(); td.hidden[info.name] || td.members[info.name] {
		return typeName(g.ns + "::" + info.name)
	}
	return typeName(info.name)
}

// ifaceMethods returns the number of methods of the interface type t (0 for an empty one).
func (g *gen) ifaceMethods(t types.Type) int {
	if it, ok := t.Underlying().(*types.Interface); ok {
		return it.NumMethods()
	}
	return 0
}

// ifaceMethodName returns the C++ name of the method fn of the interface type t.
func (g *gen) ifaceMethodName(t types.Type, fn *types.Func, pos token.Pos) string {
	if isErrorType(t) {
		return "Error"
	}
	info := g.ifaceOf(t, pos)
	if info == nil {
		return "gp_unsupported"
	}
	for i, m := range info.methods {
		if m.Name() == fn.Name() && (exportedName(m.Name()) || pkgPath(m) == pkgPath(fn)) {
			return info.mnames[i]
		}
	}
	g.errorf(pos, "internal error: %s is no method of %s", fn.Name(), info.goName)
	return "gp_unsupported"
}

// sigCxx returns the C++ parameter types and the result type of the signature sig, as a
// function or a func value of it has them (closures.go: funcType).
func (g *gen) sigCxx(sig *types.Signature, pos token.Pos) ([]cppir.Type, cppir.Type) {
	ft := g.funcType(sig, pos).(*cppir.TypeName).Args[0].(*cppir.FuncType)
	return ft.Params, ft.Result
}

// ifaceDefs returns the definitions of the interface structs (after the forward declarations of
// the structs, before their definitions). The empty and the unnamed ones are made while the
// functions are lowered, so this runs last.
func (g *gen) ifaceDefs() []cppir.Decl {
	tab := g.itable()
	var decls []cppir.Decl
	// Every interface struct is declared first: the methods of one may mention another (a
	// visitor and its nodes).
	for _, info := range tab.list {
		decls = append(decls, &cppir.Struct{Name: info.name, Forward: true})
	}
	for _, info := range tab.list {
		s := &cppir.Struct{Name: info.name, Bases: []cppir.Type{typeName("gp::iface")}}
		s.Members = append(s.Members, &cppir.Using{Name: "gp::iface::iface"})
		s.Members = append(s.Members, &cppir.Func{
			Specs:  []string{"static"},
			Result: &cppir.Ptr{Elem: &cppir.Const{Elem: typeName("gp::type_desc")}},
			Name:   "gp_desc",
		})
		for i, m := range info.methods {
			params, result := g.sigCxx(m.Signature(), m.Pos())
			d := &cppir.Func{Specs: []string{wrapTag}, Result: result, Name: info.mnames[i], Quals: []string{"const"}}
			for k, p := range params {
				d.Params = append(d.Params, cppir.Param{Type: p, Name: "gp_a" + strconv.Itoa(k)})
			}
			s.Members = append(s.Members, d)
		}
		decls = append(decls, s, &cppir.StaticAssert{Cond: &cppir.Binary{Op: "==", X: call("sizeof", cxxName(info.name)), Y: lit("32")}}, &cppir.Blank{})
	}
	return decls
}

// ifaceImpls returns the definitions that follow the descriptors of the types: the method
// arrays and descriptors of the interfaces, gp_desc and the wrappers.
func (g *gen) ifaceImpls() []cppir.Decl {
	var decls []cppir.Decl
	inline := []string{"inline"}
	descPtr := &cppir.Ptr{Elem: &cppir.Const{Elem: typeName("gp::type_desc")}}
	for _, info := range g.itable().list {
		// The methods of the descriptor (and the name of an unnamed interface type).
		var ims []cppir.Expr
		for _, m := range info.methods {
			args := []cppir.Expr{lit(cppir.CString(m.Name()))}
			if exportedName(m.Name()) {
				args = append(args, lit("nullptr"), lit(cppir.CString(g.canonText(m.Signature()))))
			} else {
				args = append(args, lit(cppir.CString(pkgPath(m))), lit(cppir.CString(g.canonText(m.Signature()))), lit(cppir.CString(m.Pkg().Name())))
			}
			ims = append(ims, call("gp::imethod", args...))
		}
		if info.named != nil {
			var shape cppir.Expr = call("gp::shape_interface", lit("nullptr"), lit("0"))
			if len(ims) > 0 {
				decls = append(decls, &cppir.VarDecl{Specs: []string{"inline", "const"}, Type: typeName("gp::imethod"),
					Vars: []cppir.Declarator{{Name: "gp_im_" + info.name + "[]", Init: &cppir.InitList{Elems: ims}}}})
				shape = call("gp::shape_interface", cxxName("gp_im_"+info.name))
			}
			obj := info.named.Obj()
			pkg := ""
			if obj.Pkg() != nil {
				pkg = importPath(obj.Pkg())
			}
			decls = append(decls, &cppir.VarDecl{Specs: []string{"inline", "const"}, Type: typeName("gp::type_desc"),
				Vars: []cppir.Declarator{{Name: "gp_td_" + info.name, Init: templateCall("gp::named", []cppir.Node{typeName(info.name)},
					lit(cppir.CString(info.goName)), lit(cppir.CString(pkg)), shape)}}})
			decls = append(decls, &cppir.Func{Specs: inline, Result: descPtr, Name: info.name + "::gp_desc",
				Body: &cppir.Block{Stmts: []cppir.Stmt{&cppir.Return{X: &cppir.Unary{Op: "&", X: cxxName("gp_td_" + info.name)}}}}})
		} else {
			// An unnamed type: interned from its methods, once.
			td := varDecl(cppir.Pos{}, &cppir.Const{Elem: descPtr}, "gp_td", templateInterface(ims, info.goName))
			td.Specs = []string{"static"}
			decls = append(decls, &cppir.Func{Specs: inline, Result: descPtr, Name: info.name + "::gp_desc",
				Body: &cppir.Block{Stmts: []cppir.Stmt{td, &cppir.Return{X: cxxName("gp_td")}}}})
		}
		// The wrappers.
		for i, m := range info.methods {
			params, result := g.sigCxx(m.Signature(), m.Pos())
			sig := &cppir.FuncType{Result: result, Params: params}
			args := []cppir.Expr{deref(lit("this")), lit(strconv.Itoa(i))}
			fn := &cppir.Func{Specs: []string{wrapTag, "inline"}, Result: result, Name: info.name + "::" + info.mnames[i], Quals: []string{"const"}, Body: &cppir.Block{}}
			for k, p := range params {
				name := "gp_a" + strconv.Itoa(k)
				fn.Params = append(fn.Params, cppir.Param{Type: p, Name: name})
				if _, isRef := p.(*cppir.Ref); isRef {
					args = append(args, cxxName(name))
				} else {
					args = append(args, moved(p, cxxName(name)))
				}
			}
			icall := &cppir.Call{Fun: &cppir.Name{Name: "gp::icall", Args: []cppir.Node{sig}}, Args: args}
			if tn, ok := result.(*cppir.TypeName); ok && tn.Name == "void" && len(tn.Args) == 0 {
				fn.Body.Stmts = []cppir.Stmt{&cppir.ExprStmt{X: icall}}
			} else {
				fn.Body.Stmts = []cppir.Stmt{&cppir.Return{X: icall}}
			}
			decls = append(decls, fn)
		}
		decls = append(decls, &cppir.Blank{})
	}
	return decls
}

// templateInterface returns gp::td_interface({methods}, "name"), the descriptor of an unnamed
// interface type.
func templateInterface(ims []cppir.Expr, name string) cppir.Expr {
	return call("gp::td_interface", &cppir.InitList{Elems: ims}, lit(cppir.CString(name)))
}

// holdsIface reports whether a value of type t is, or holds in a field, an element or behind a
// pointer, an interface value (which the fmt shim can't print: step 5.11).
func holdsIface(t types.Type) bool {
	seen := make(map[types.Type]bool)
	var walk func(t types.Type) bool
	walk = func(t types.Type) bool {
		if seen[t] {
			return false
		}
		seen[t] = true
		if isInterface(t) {
			return true
		}
		switch u := types.Unalias(t).Underlying().(type) {
		case *types.Pointer:
			return walk(u.Elem())
		case *types.Slice:
			return walk(u.Elem())
		case *types.Array:
			return walk(u.Elem())
		case *types.Map:
			return walk(u.Key()) || walk(u.Elem())
		case *types.Struct:
			for i := range u.NumFields() {
				if walk(u.Field(i).Type()) {
					return true
				}
			}
		}
		return false
	}
	return walk(t)
}

// hashMayPanic reports whether hashing a value of the comparable type t can panic, because the
// value is, or holds BY VALUE (in a field that is not blank, or an array element), an interface
// whose dynamic type may be unhashable. Such a key type gets gp_hash_panics (gocpp/hash.hpp):
// Go checks the key of a read or delete in a nil or empty map for it without hashing
// (mapKeyError), and gp::check_empty_key does the same.
func hashMayPanic(t types.Type) bool {
	seen := make(map[types.Type]bool)
	var walk func(t types.Type) bool
	walk = func(t types.Type) bool {
		if seen[t] {
			return false
		}
		seen[t] = true
		if isInterface(t) {
			return true
		}
		switch u := types.Unalias(t).Underlying().(type) {
		case *types.Array:
			return walk(u.Elem())
		case *types.Struct:
			for i := range u.NumFields() {
				if f := u.Field(i); f.Name() != "_" && walk(f.Type()) {
					return true
				}
			}
		}
		return false
	}
	return walk(t)
}
