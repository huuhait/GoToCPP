package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Optional concepts (Phase 07 step 7.9, Options.Concepts, `gocpp -gp:concepts`,
// docs/spec/cpp-mapping.md "Generics").
//
// pkg.hpp gets C++20 concepts that say, for a C++ type, whether it satisfies a Go constraint, so that
// C++ code that works with a package (a cpp-lib user, step 6.10) can check its types early,
// with a static_assert or a requires-clause, instead of in an error somewhere in a template:
//
//   - gp_concept_<Name><T> for each named interface type of the package that has no type
//     parameters (a constraint like `type Number interface{ ~int | ~float64 }`, or an ordinary
//     interface that is used as a constraint: its methods);
//   - gp_sat_<Func><T, U, ...> for each generic function (also an unexported one), the
//     conjunction of the constraints of all its type parameters, in the order of the type
//     parameters (a constraint that names another type parameter, `S ~[]E`, uses the template
//     parameter of that one).
//
// The pieces are those of gocpp/concepts.hpp: tilde<T, U> for ~U, exact<T, U> for U, comparable<T>,
// and the methods as requires-expressions. Under ADR-030 a generic function has no C++
// declaration that could carry a requires-clause: it exists for the instances that the Go code
// names. The concepts are therefore not attached to anything the generated code uses; a term that
// C++ cannot spell (a struct type, a C/C++ type) is left out, which makes the concept accept more,
// never fewer, types than Go's constraint.

// conceptDecls returns the concept definitions of the package's header, in source order.
func (g *gen) conceptDecls() []cppir.Decl {
	if !g.opts.Concepts || g.pkg.Load.Std {
		return nil
	}
	var out []cppir.Decl
	for _, f := range g.pkg.Load.Files {
		for _, d := range f.AST.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					if c := g.namedConcept(spec.(*ast.TypeSpec)); c != nil {
						out = append(out, c)
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil && d.Type.TypeParams != nil && d.Name.Name != "_" {
					if c := g.funcConcept(d); c != nil {
						out = append(out, c)
					}
				}
			}
		}
	}
	return out
}

// conceptName returns the name of the concept of the named interface type name.
func conceptName(name string) string { return "gp_concept_" + name }

// namedConcept returns the concept of the interface type that spec declares, or nil.
func (g *gen) namedConcept(spec *ast.TypeSpec) cppir.Decl {
	if spec.TypeParams != nil || spec.Assign.IsValid() || spec.Name.Name == "_" {
		return nil
	}
	tn, _ := g.info.Defs[spec.Name].(*types.TypeName)
	if tn == nil {
		return nil
	}
	named, _ := tn.Type().(*types.Named)
	if named == nil {
		return nil
	}
	it, ok := named.Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	cg := &conceptGen{g: g}
	subject := typeName("gp_T") // (a Go name never starts with gp_ in C++: escape appends a _)
	return &cppir.Concept{
		Template: &cppir.Template{Params: []cppir.Param{{Type: typeName("typename"), Name: "gp_T"}}},
		Name:     conceptName(tn.Name()),
		Op:       "&&",
		Terms:    cg.interfaceTerms(it, subject),
	}
}

// funcConcept returns gp_sat_<name> for the generic function d, or nil.
func (g *gen) funcConcept(d *ast.FuncDecl) cppir.Decl {
	fn, _ := g.info.Defs[d.Name].(*types.Func)
	if fn == nil {
		return nil
	}
	tparams := fn.Signature().TypeParams()
	cg := &conceptGen{g: g}
	// Each type parameter is a template parameter: a placeholder type that the lowering spells as
	// its name (the mechanism of C++ type arguments, cxxNamed), so a constraint that mentions it
	// is spelled with the template parameter.
	targs := make([]types.Type, tparams.Len())
	var params []cppir.Param
	used := make(map[string]bool)
	for i := range tparams.Len() {
		name := g.escape(tparams.At(i).Obj().Name())
		// (std and gp would hide the namespaces that the concept's text names)
		for name == "_" || name == "std" || name == "gp" || g.pkg.Types.Scope().Lookup(name) != nil || used[name] {
			name += "_"
			if name == "__" {
				name = "T" + strconv.Itoa(i)
			}
		}
		used[name] = true
		params = append(params, cppir.Param{Type: typeName("typename"), Name: name})
		tn := types.NewTypeName(token.NoPos, g.pkg.Types, "gp_param_"+name, nil)
		ph := types.NewNamed(tn, types.NewStruct(nil, nil), nil)
		if g.cxxNamed == nil {
			g.cxxNamed = make(map[*types.Named]cxxInfo)
		}
		g.cxxNamed[ph] = cxxInfo{typ: typeName(name)}
		targs[i] = ph
		cg.placeholders = append(cg.placeholders, ph)
	}
	defer func() {
		for _, ph := range cg.placeholders {
			delete(g.cxxNamed, ph)
		}
	}()
	s := newSubster(tparams, targs, g.context())
	var terms []cppir.Expr
	for i := range tparams.Len() {
		c := tparams.At(i).Constraint()
		subject := typeName(params[i].Name)
		if n, ok := types.Unalias(c).(*types.Named); ok && g.ownConstraint(n) {
			terms = append(terms, &cppir.Name{Name: conceptName(n.Obj().Name()), Args: []cppir.Node{subject}})
			continue
		}
		terms = append(terms, cg.element(s.typ(c), subject)...)
	}
	return &cppir.Concept{
		Template: &cppir.Template{Params: params},
		Name:     "gp_sat_" + fn.Name(),
		Op:       "&&",
		Terms:    terms,
	}
}

// ownConstraint reports whether n is a named interface type of the package that has a concept
// (gp_concept_<name>): declared at package level, without type parameters.
func (g *gen) ownConstraint(n *types.Named) bool {
	obj := n.Obj()
	if obj.Pkg() != g.pkg.Types || obj.Parent() != g.pkg.Types.Scope() || n.TypeParams().Len() > 0 || n.TypeArgs().Len() > 0 {
		return false
	}
	_, ok := n.Underlying().(*types.Interface)
	return ok
}

// A conceptGen writes the terms of one constraint.
type conceptGen struct {
	g            *gen
	placeholders []*types.Named
}

// interfaceTerms returns the conjuncts that say that subject is in the type set and has the methods
// of the interface type it: its embedded elements, then its own methods.
func (cg *conceptGen) interfaceTerms(it *types.Interface, subject cppir.Type) []cppir.Expr {
	var terms []cppir.Expr
	for i := range it.NumEmbeddeds() {
		terms = append(terms, cg.element(it.EmbeddedType(i), subject)...)
	}
	for i := range it.NumExplicitMethods() {
		if e := cg.method(it.ExplicitMethod(i), subject); e != nil {
			terms = append(terms, e)
		}
	}
	return terms
}

// element returns the conjuncts of an embedded element of an interface: a union of terms, an
// interface (its terms and methods), comparable, or one type.
func (cg *conceptGen) element(t types.Type, subject cppir.Type) []cppir.Expr {
	switch t := types.Unalias(t).(type) {
	case *types.Union:
		var alts []cppir.Expr
		for i := range t.Len() {
			term := t.Term(i)
			if _, ok := term.Type().Underlying().(*types.Interface); ok {
				// A term that is an interface (Integer | Float): its own type set.
				all := cg.element(term.Type(), subject)
				if len(all) == 0 {
					return nil // any, or a type set that C++ cannot tell: the union accepts everything
				}
				alts = appendUnique(alts, andOf(all))
				continue
			}
			e := cg.term(term.Tilde(), term.Type(), subject)
			if e == nil {
				return nil // a term that cannot be spelled: the whole union is left out
			}
			alts = appendUnique(alts, e)
		}
		return []cppir.Expr{orOf(alts)}
	case *types.Named:
		if t.Obj().Pkg() == nil && t.Obj().Name() == "comparable" {
			return []cppir.Expr{&cppir.Name{Name: "::gp::concepts::comparable", Args: []cppir.Node{subject}}}
		}
		if it, ok := t.Underlying().(*types.Interface); ok {
			return cg.interfaceTerms(it, subject)
		}
	case *types.Interface:
		return cg.interfaceTerms(t, subject)
	}
	if e := cg.term(false, t, subject); e != nil {
		return []cppir.Expr{e}
	}
	return nil
}

// term returns tilde<subject, U> or exact<subject, U> for the type t, or nil when C++ cannot spell it.
func (cg *conceptGen) term(tilde bool, t types.Type, subject cppir.Type) cppir.Expr {
	if !cg.spellable(t) {
		return nil
	}
	name := "::gp::concepts::exact"
	if tilde {
		name = "::gp::concepts::tilde"
	}
	return &cppir.Name{Name: name, Args: []cppir.Node{subject, cg.spell(t)}}
}

// method returns the requirement that subject has the method m: a value method (x.M(a)), a
// value method through a pointer (x->M(a)) or a pointer method of the pointee (X::M(x, a)); nil
// when its signature cannot be spelled.
func (cg *conceptGen) method(m *types.Func, subject cppir.Type) cppir.Expr {
	sig := m.Signature()
	var params []cppir.Param
	params = append(params, cppir.Param{Type: &cppir.Ref{Elem: subject}, Name: "gp_x"})
	var args []cppir.Expr
	for i := range sig.Params().Len() {
		p := sig.Params().At(i)
		if !cg.spellable(p.Type()) {
			return nil
		}
		name := "gp_a" + strconv.Itoa(i)
		params = append(params, cppir.Param{Type: cg.spell(p.Type()), Name: name})
		args = append(args, &cppir.Name{Name: name})
	}
	var result cppir.Type
	switch n := sig.Results().Len(); {
	case n == 0:
		result = typeName("void")
	default:
		var rs []cppir.Node
		for i := range n {
			t := sig.Results().At(i).Type()
			if !cg.spellable(t) {
				return nil
			}
			rs = append(rs, cg.spell(t))
		}
		if n == 1 {
			result = rs[0].(cppir.Type)
		} else {
			result = &cppir.TypeName{Name: "::std::tuple", Args: rs}
		}
	}
	same := &cppir.TypeName{Name: "::std::same_as", Args: []cppir.Node{result}}
	req := func(call cppir.Expr) cppir.Expr {
		return &cppir.Requires{Params: params, Reqs: []cppir.Requirement{{X: call, Result: same}}}
	}
	// The member is named as the struct that has it names it: a method named like its struct is
	// written with a _ at the end (constructor clash: methodName). The type is not known here, so
	// both names are tried.
	names := []string{cg.g.escape(m.Name())}
	if alt := cg.g.methodName(m.Name(), m.Name()); alt != names[0] {
		names = append(names, alt)
	}
	x := &cppir.Name{Name: "gp_x"}
	var alts []cppir.Expr
	for _, name := range names {
		value := &cppir.Call{Fun: &cppir.Member{X: x, Name: name}, Args: args}
		viaPtr := &cppir.Call{Fun: &cppir.Member{X: x, Arrow: true, Name: name}, Args: args}
		static := &cppir.Call{Fun: &cppir.Name{Name: "::gp::concepts::ptr_elem_t<" + cppir.TypeString(subject) + ">::" + name},
			Args: append([]cppir.Expr{x}, args...)}
		// (a defined pointer type, `type NPS *S`, has no methods in Go but a gp::newtype's -> works)
		onlyPtr := &cppir.Binary{Op: "&&", X: &cppir.Name{Name: "::gp::concepts::is_ptr", Args: []cppir.Node{subject}}, Y: req(viaPtr)}
		alts = append(alts, req(value), onlyPtr, req(static))
	}
	return orOf(alts)
}

// appendUnique appends e to alts unless the same C++ is there (~int and ~int64 are both int64_t).
func appendUnique(alts []cppir.Expr, e cppir.Expr) []cppir.Expr {
	text := cppir.ExprString(e)
	for _, a := range alts {
		if cppir.ExprString(a) == text {
			return alts
		}
	}
	return append(alts, e)
}

// andOf returns the conjuncts joined with &&.
func andOf(all []cppir.Expr) cppir.Expr {
	e := all[0]
	for _, a := range all[1:] {
		e = &cppir.Binary{Op: "&&", X: e, Y: a}
	}
	return e
}

// orOf returns the alternatives joined with ||.
func orOf(alts []cppir.Expr) cppir.Expr {
	e := alts[0]
	for _, a := range alts[1:] {
		e = &cppir.Binary{Op: "||", X: e, Y: a}
	}
	return e
}

// spell returns the C++ type of t, which spellable has checked.
func (cg *conceptGen) spell(t types.Type) cppir.Type {
	g := cg.g
	old := g.err
	g.err = nil
	r := g.cxxType(t, token.NoPos)
	g.err = old
	return r
}

// spellable reports whether t can be written without making a declaration: basic types, the
// type constructors over them, named types of the program (not generic, not a standard
// package's, but error and fmt's interfaces) and the placeholders of the type parameters. An
// unnamed struct or interface type, a C/C++ type, or a type of the standard library is not.
func (cg *conceptGen) spellable(t types.Type) bool {
	g := cg.g
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		return basicOf(t) != nil
	case *types.Pointer:
		return cg.spellable(t.Elem())
	case *types.Slice:
		return cg.spellable(t.Elem())
	case *types.Array:
		return cg.spellable(t.Elem())
	case *types.Chan:
		return cg.spellable(t.Elem())
	case *types.Map:
		return cg.spellable(t.Key()) && cg.spellable(t.Elem())
	case *types.Signature:
		for v := range t.Params().Variables() {
			if !cg.spellable(v.Type()) {
				return false
			}
		}
		for v := range t.Results().Variables() {
			if !cg.spellable(v.Type()) {
				return false
			}
		}
		return true
	case *types.Interface:
		return isAnyType(t)
	case *types.Named:
		if _, ok := g.cxxNamed[t]; ok {
			return true
		}
		if _, ok := runtimeIfaceOf(t); ok {
			return true
		}
		if t.TypeArgs().Len() > 0 || t.TypeParams().Len() > 0 || !complete(t) || hasC(t) {
			return false
		}
		pkg := t.Obj().Pkg()
		if pkg == nil || pkg.Path() == "unsafe" || pkg.Path() == "weak" {
			return false
		}
		if pkg == g.pkg.Types {
			return t.Obj().Parent() == pkg.Scope() && g.namedLowered(t)
		}
		o := g.prog.byPkg[pkg]
		return o != nil && !o.pkg.Load.Std && t.Obj().Parent() == pkg.Scope() && o.namedLowered(t)
	}
	return false
}

// namedLowered reports whether the named type n of the package has a C++ type of its own: a struct,
// a gp::newtype or an interface (not a weak pointer, not a type over C/C++ types).
func (g *gen) namedLowered(n *types.Named) bool {
	if isWeakPointer(n) {
		return false
	}
	if it, ok := n.Underlying().(*types.Interface); ok {
		return it.IsMethodSet() // a constraint (a type set) has no struct
	}
	return isStruct(n) || isNewtype(n)
}
