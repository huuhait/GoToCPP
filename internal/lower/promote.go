package lower

import (
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Forwarding members for promoted methods (Phase 05 README §2.6, step 5.8).
//
//	type Base struct{ ID int }
//	func (b Base) Get() int  { return b.ID }
//	func (b *Base) Touch()   { b.ID++ }
//	type User struct{ Base; Name string }
//
//	struct User {
//	    ...
//	    int64_t Get() const;                  // in User's method set: a const member
//	    static void Touch(gp::ptr<User> gp_p); // only in *User's: static, takes the pointer
//	};
//	int64_t User::Get() const { return Base.Get(); }
//	void User::Touch(gp::ptr<User> gp_p) { Base::Touch(gp::field_ptr(gp_p, &User::Base)); }
//
// Every method of the method set of *T that T gets from an embedded field (go/types' selection has
// a path of more than one index) is a member of T's struct with the same form as a method that T
// declares: a const member function when the method is in the method set of T (a value method of
// the embedded type, or any method of an embedded pointer's type), else a static member whose first
// parameter is the owning pointer. So the thunks of the method tables (step 5.4) call
// unbox<T>(self).M(args), p->M(args) and T::PM(p, args) for a promoted method as for a declared
// one, a C++ user (and generic code, Phase 07) writes u.Touch() without knowing the path, and
// T's method tables and itabs have the promoted methods, with the receiver adjusted by the path
// (the fields, an embedded pointer's nil check, gp::field_ptr for an address inside the owner).
//
// A call in the program that Go resolves through a path is still written with the path (u.Base.Get(),
// methods.go): the forwarders are for C++ callers and the method tables, never for the generated
// calls, so they can't change what a call evaluates or panics on.
//
// So they are written only for the structs that need them (code size: a type that promotes 100
// methods into 20 structs would be 1000 members, 10 times the lines of the program, and 3 times the
// compile time): the structs whose descriptors have method tables (fwdNeed, set by
// the descriptors of step 5.4/5.5's emitter), and every struct when Options.AllForwarders is set (the
// tests that call the members from C++). The names of all the forwarders still hide the package's
// names in the struct (forwarderNames), whether they are written or not: which structs need them is
// found while the functions are lowered, after the first member function has been written.
//
// The members are declared in the struct (methodDecls) and defined out of line after all the structs
// (forwardDefs), like the fmt hooks, since the embedded type may be defined after the outer struct
// (through an embedded pointer).

// A forwarder is the member of a struct for a method promoted into it.
type forwarder struct {
	sel    *selection // the selection in T's method set, or in *T's when static
	name   string     // the member's C++ name
	static bool       // a static member with a pointer: the method is only in *T's method set
}

// Options are the choices of Package that the driver does not make.
type Options struct {
	// AllForwarders writes the forwarding members of every struct (see the top), not only those of the
	// structs that need them.
	AllForwarders bool
}

// forwarders returns the forwarders of the struct si that are written: all of them for a struct
// that needs them (Options.AllForwarders, or fwdNeed: the method tables of step 5.4/5.5 set it), else none.
// The request is on the owner of the struct (needForwarders), also when g is another package: the
// declarations that such a g makes for a shared struct are not used (makeShared runs in the owner),
// so no program tells g from the owner here; the owner is what is right.
func (g *gen) forwarders(si *structInfo) []forwarder {
	if o := si.owner; o.opts.AllForwarders || o.fwdNeed[si] {
		return g.allForwarders(si)
	}
	return nil
}

// allForwarders returns the forwarders of the struct si, in the order of go/types' method sets (by
// name), none for a named type that isn't a struct or for a struct without promoted methods.
func (g *gen) allForwarders(si *structInfo) []forwarder {
	if si.st == nil || !hasEmbedded(si.st) {
		return nil
	}
	if si.owner != g {
		return si.owner.allForwarders(si) // (the names are the owner's: macros of its headers)
	}
	if fs, ok := g.fwds[si]; ok {
		return fs
	}
	var fs []forwarder
	vset := types.NewMethodSet(si.t)
	pset := types.NewMethodSet(types.NewPointer(si.t))
	for i := range pset.Len() {
		s := pset.At(i)
		fn, _ := s.Obj().(*types.Func)
		if fn == nil || len(s.Index()) < 2 || fn.Name() == "_" || s.Kind() != types.MethodVal {
			continue
		}
		f := forwarder{sel: wrapSelection(s), name: g.memberFor(si, fn), static: true}
		if v := vset.Lookup(fn.Pkg(), fn.Name()); v != nil {
			f.sel, f.static = wrapSelection(v), false
		}
		fs = append(fs, f)
	}
	if g.fwds == nil {
		g.fwds = make(map[*structInfo][]forwarder)
	}
	g.fwds[si] = fs
	return fs
}

// hasEmbedded reports whether the struct st has an embedded field.
func hasEmbedded(st *types.Struct) bool {
	for i := range st.NumFields() {
		if st.Field(i).Embedded() {
			return true
		}
	}
	return false
}

// forwarderNames returns the C++ names of all the forwarders of si, written or not (see the top).
func (g *gen) forwarderNames(si *structInfo) []string {
	var names []string
	for _, f := range g.allForwarders(si) {
		names = append(names, f.name)
	}
	return names
}

// fwdSig returns the C++ parameters (named gp_a0, gp_a1, ...) and result type of the method fn
// as a forwarder declares it, like prepare's signature for a method of the struct itself.
func (g *gen) fwdSig(fn *types.Func, pos token.Pos) ([]cppir.Param, cppir.Type) {
	sig := fn.Signature()
	params := make([]cppir.Param, sig.Params().Len())
	for i := range params {
		p := sig.Params().At(i)
		params[i] = cppir.Param{Type: g.varType(p, pos), Name: "gp_a" + strconv.Itoa(i)}
		if largeVar(p.Type()) {
			params[i].Type = &cppir.Ref{Elem: &cppir.Const{Elem: params[i].Type}} // as prepare does
		}
	}
	return params, g.resultType(sig, pos)
}

// fwdDecls returns the declarations in the struct si of its forwarders (methodDecls).
func (g *gen) fwdDecls(si *structInfo) []cppir.Decl {
	var decls []cppir.Decl
	for _, f := range g.forwarders(si) {
		fn := f.sel.Obj().(*types.Func)
		params, result := g.fwdSig(fn, fn.Pos())
		d := &cppir.Func{Result: result, Name: f.name, Params: params}
		if f.static {
			d.Specs = []string{"static"}
			d.Params = append([]cppir.Param{g.fwdRecv(si)}, params...)
		} else {
			d.Quals = []string{"const"}
		}
		decls = append(decls, d)
	}
	return decls
}

// fwdRecv returns the pointer parameter of a static forwarder: gp::ptr<T> gp_p.
func (g *gen) fwdRecv(si *structInfo) cppir.Param {
	return cppir.Param{Type: ptrType(g.structType(si.t, token.NoPos)), Name: "gp_p"}
}

// forwardDefs returns the definitions of the forwarders of si, out of line after all the structs
// (see the top): each calls the promoted method through the path of its selection, as a call
// u.M(args) in Go does.
func (g *gen) forwardDefs(si *structInfo) []cppir.Decl {
	var decls []cppir.Decl
	fs := g.forwarders(si)
	if len(fs) == 0 {
		return nil
	}
	defer g.hideMembers(g.memberNames(si))() // they are member functions
	for _, f := range fs {
		fn := f.sel.Obj().(*types.Func)
		r := pathOf(f.sel)
		params, result := g.fwdSig(fn, fn.Pos())
		d := &cppir.Func{Result: result, Name: si.name + "::" + f.name, Params: params}
		fg := newFuncGen(g, g.newFuncNames(), nil)
		var base cppir.Expr // where the path starts: the pointer gp_p (to *T), or *this (a T)
		if f.static {
			d.Params = append([]cppir.Param{g.fwdRecv(si)}, params...)
			base = cxxName("gp_p")
		} else {
			d.Quals = []string{"const"}
			base = deref(lit("this"))
		}
		args := make([]cppir.Expr, len(params))
		for i, p := range params {
			args[i] = cxxName(p.Name)
			if t := fn.Signature().Params().At(i).Type(); !largeVar(t) && !plainValue(t) {
				args[i] = movedName(p.Name)
			}
		}
		recv, ptr, ok := fg.recvFrom(base, r)
		if !ok {
			g.errorf(token.NoPos, "internal error: the forwarder %s of %s has no pointer for its receiver", f.name, si.name)
			return nil
		}
		// A value method through an embedded pointer: -> (gp::ptr's) panics for nil, and the method
		// copies its receiver at its entry, so no copy of the pointer is needed to hold the object
		// (a call in the program makes one for the arguments' evaluation: methodCall; the arguments
		// here are parameters).
		x := g.callMethod(recv, ptr, fn, r, args, token.NoPos)
		var body cppir.Stmt = &cppir.Return{X: x}
		if sig := fn.Signature(); sig.Results().Len() == 0 {
			body = &cppir.ExprStmt{X: x}
		}
		d.Body = &cppir.Block{Stmts: []cppir.Stmt{body}}
		decls = append(decls, d, &cppir.Blank{})
	}
	return decls
}

// plainValue reports whether a value of type t is copied as cheaply as it is moved: a number, a
// bool or a pointer-sized basic value (not a string or a complex number).
func plainValue(t types.Type) bool {
	b := basicOf(t)
	return b != nil && b.Info()&(types.IsString|types.IsComplex) == 0
}

// memberFor returns the C++ name of the member of the struct si that stands for the method fn of
// its method set: the method's name (escaped), or for an unexported method of another package
// that si promotes, the name with the package: `name__gpkg_base`'s tail. Two packages' unexported
// methods are different methods in Go (a.m and b.m), so a struct that declares its own m and
// promotes another package's m has two members, and the C++ names must differ.
func (g *gen) memberFor(si *structInfo, fn *types.Func) string {
	name := fn.Name()
	if !fn.Exported() && fn.Pkg() != nil && fn.Pkg() != si.owner.pkg.Types {
		name += "__" + NamespaceName(importPath(fn.Pkg()))[len("gpkg_"):]
	}
	return g.methodName(si.name, name)
}
