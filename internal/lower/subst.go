package lower

import (
	"go/token"
	"go/types"
)

// A subster replaces the type parameters of a generic function by the type arguments of one of
// its instances (generics.go). go/types has this only inside its checker; the lowering needs it
// for the types of every expression of the body, which go/types recorded in terms of the type
// parameters.
//
// A type that mentions no type parameter is returned as it is (the same pointer), so that the
// identity of named types, signatures and structs is kept wherever it can be.
type subster struct {
	env  map[*types.TypeParam]types.Type
	ctxt *types.Context

	// vars are the variables whose type changed, by the variable of the generic function: the
	// parameters, the locals and the variables of the signatures of func literals. The instance
	// has a new variable for each of them (with the same name and place), because the lowering
	// keys its tables by the variable, and the type is part of the variable.
	vars map[*types.Var]*types.Var

	// names are the objects that stand for the type parameters in the code, T in make([]T, n),
	// by type parameter: a type name whose type is the type argument.
	names map[*types.TypeParam]*types.TypeName

	err error
}

func newSubster(tparams *types.TypeParamList, targs []types.Type, ctxt *types.Context) *subster {
	s := &subster{
		env:   make(map[*types.TypeParam]types.Type, len(targs)),
		ctxt:  ctxt,
		vars:  make(map[*types.Var]*types.Var),
		names: make(map[*types.TypeParam]*types.TypeName),
	}
	for i := range tparams.Len() {
		s.env[tparams.At(i)] = targs[i]
	}
	return s
}

// typ returns t with the type parameters replaced.
func (s *subster) typ(t types.Type) types.Type {
	switch t := t.(type) {
	case nil:
		return nil
	case *types.Basic:
		return t
	case *types.Alias:
		u := types.Unalias(t)
		if r := s.typ(u); r != u {
			return r
		}
		return t
	case *types.TypeParam:
		if r, ok := s.env[t]; ok {
			return r
		}
		return t
	case *types.Named:
		targs := t.TypeArgs()
		if targs.Len() == 0 {
			return t
		}
		args := make([]types.Type, targs.Len())
		changed := false
		for i := range args {
			args[i] = s.typ(targs.At(i))
			changed = changed || args[i] != targs.At(i)
		}
		if !changed {
			return t
		}
		r, err := types.Instantiate(s.ctxt, t.Origin(), args, false)
		if err != nil {
			s.err = err
			return t
		}
		return r
	case *types.Pointer:
		if e := s.typ(t.Elem()); e != t.Elem() {
			return types.NewPointer(e)
		}
	case *types.Slice:
		if e := s.typ(t.Elem()); e != t.Elem() {
			return types.NewSlice(e)
		}
	case *types.Array:
		if e := s.typ(t.Elem()); e != t.Elem() {
			return types.NewArray(e, t.Len())
		}
	case *types.Map:
		k, e := s.typ(t.Key()), s.typ(t.Elem())
		if k != t.Key() || e != t.Elem() {
			return types.NewMap(k, e)
		}
	case *types.Chan:
		if e := s.typ(t.Elem()); e != t.Elem() {
			return types.NewChan(t.Dir(), e)
		}
	case *types.Signature:
		params, results := s.tuple(t.Params()), s.tuple(t.Results())
		if params != t.Params() || results != t.Results() {
			return types.NewSignatureType(nil, nil, nil, params, results, t.Variadic())
		}
	case *types.Tuple:
		return s.tuple(t)
	case *types.Struct:
		fields := make([]*types.Var, t.NumFields())
		tags := make([]string, t.NumFields())
		changed := false
		for i := range fields {
			fields[i] = s.varFor(t.Field(i))
			tags[i] = t.Tag(i)
			changed = changed || fields[i] != t.Field(i)
		}
		if changed {
			return types.NewStruct(fields, tags)
		}
	case *types.Interface:
		return s.iface(t)
	case *types.Union:
		terms := make([]*types.Term, t.Len())
		changed := false
		for i := range terms {
			tt := t.Term(i)
			if ty := s.typ(tt.Type()); ty != tt.Type() {
				terms[i] = types.NewTerm(tt.Tilde(), ty)
				changed = true
			} else {
				terms[i] = tt
			}
		}
		if changed {
			return types.NewUnion(terms)
		}
	}
	return t
}

// tuple returns the parameters or results t with the type parameters replaced.
func (s *subster) tuple(t *types.Tuple) *types.Tuple {
	if t == nil {
		return nil
	}
	vars := make([]*types.Var, t.Len())
	changed := false
	for i := range vars {
		vars[i] = s.varFor(t.At(i))
		changed = changed || vars[i] != t.At(i)
	}
	if !changed {
		return t
	}
	return types.NewTuple(vars...)
}

// iface returns the interface type t with the type parameters replaced.
func (s *subster) iface(t *types.Interface) types.Type {
	methods := make([]*types.Func, t.NumExplicitMethods())
	embeddeds := make([]types.Type, t.NumEmbeddeds())
	changed := false
	for i := range methods {
		m := t.ExplicitMethod(i)
		sig := m.Signature()
		if ns := s.typ(sig); ns != sig {
			methods[i] = types.NewFunc(m.Pos(), m.Pkg(), m.Name(), ns.(*types.Signature))
			changed = true
		} else {
			methods[i] = m
		}
	}
	for i := range embeddeds {
		embeddeds[i] = s.typ(t.EmbeddedType(i))
		changed = changed || embeddeds[i] != t.EmbeddedType(i)
	}
	if !changed {
		return t
	}
	r := types.NewInterfaceType(methods, embeddeds)
	r.Complete()
	return r
}

// varFor returns the variable of the instance that stands for v, which is v itself when its type
// has no type parameter.
func (s *subster) varFor(v *types.Var) *types.Var {
	if v == nil {
		return nil
	}
	if n, ok := s.vars[v]; ok {
		return n
	}
	nt := s.typ(v.Type())
	if nt == v.Type() {
		return v
	}
	var n *types.Var
	if v.IsField() {
		n = types.NewField(v.Pos(), v.Pkg(), v.Name(), nt, v.Embedded())
	} else {
		n = types.NewVar(v.Pos(), v.Pkg(), v.Name(), nt)
	}
	s.vars[v] = n
	return n
}

// object returns the object of the instance for obj, which an identifier of the generic function
// declares or uses: a variable of a changed type, the name of a type parameter (T, whose type is
// the type argument now), or obj itself.
func (s *subster) object(obj types.Object) types.Object {
	switch obj := obj.(type) {
	case *types.Var:
		if obj.IsField() || obj.Pkg() != nil && obj.Parent() == obj.Pkg().Scope() {
			return obj
		}
		return s.varFor(obj)
	case *types.TypeName:
		if tp, ok := obj.Type().(*types.TypeParam); ok {
			if r, ok := s.env[tp]; ok {
				if n, ok := s.names[tp]; ok {
					return n
				}
				n := types.NewTypeName(token.NoPos, obj.Pkg(), obj.Name(), r)
				s.names[tp] = n
				return n
			}
		}
	}
	return obj
}
