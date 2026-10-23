package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
)

// PkgCVarMessage is the error of a package variable named name that is declared without a type from a
// C/C++ value whose C++ type the lowering cannot write (var s = std.to_string(5), var it = Seq(words)):
// a package variable is declared in the package's header, where C++ has no auto. A local is auto
// (x := lib.f()). generic is the name of the generic function whose call is the value, when the call
// leaves type arguments to C++ (Seq), and targs the type arguments to write (std.string), else empty:
// written out, they give the variable its type. The lowering reports the same words (cvalues.go).
func PkgCVarMessage(name, generic string, targs []string) string {
	msg := "package variables of C/C++ values without a type are not supported yet: write the type of " + name +
		" (var " + name + " T = ...)"
	if generic != "" {
		msg += " or the type arguments of " + generic + " (" + generic + "[" + strings.Join(targs, ", ") + "](...))"
	}
	return msg
}

// reportPkgCVars reports each package variable of p declared without a type from a call of a generic
// function with a type argument that C++ deduces (var it = Seq(words)): the lowering cannot write its
// C++ type (cvalues.go: varType), and the build refuses it with these words (PkgCVarMessage), at the
// variable's name. With all type arguments known, written or inferred as Go types (var it =
// Seq[std.string](words)), the variable has the instance's result type. (Any other C/C++ value without
// a type, var s = std.to_string(5), is refused by the build alone, as before: the opaque checks of
// check have such variables in packages that are only checked.) As reportGenericCArgs, it is quiet
// next to a type error of the program.
func (p *Package) reportPkgCVars(fset *token.FileSet) {
	if p.Info == nil || p.Errors == nil || p.typeErrors || p.Opaque == nil {
		return
	}
	for _, init := range p.Info.InitOrder {
		for _, v := range init.Lhs {
			if v.Name() == "_" || cxxComplete(v.Type()) || p.Opaque.TypeExpr(v) != nil {
				continue
			}
			rhs := ast.Unparen(init.Rhs)
			if len(init.Lhs) == 1 {
				if lit, ok := rhs.(*ast.CompositeLit); ok && lit.Type != nil && p.Opaque.IsOpaque(lit) {
					continue
				}
			}
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}
			fn, known := p.knownTypeArgs(call, -1)
			if fn == nil || !slices.Contains(known, false) {
				continue // not generic, or the instance's result types (lower's pkgVarInstResult)
			}
			p.Errors.Add(diag.Diagnostic{Pos: fset.Position(v.Pos()), Severity: diag.Error,
				Msg: PkgCVarMessage(v.Name(), fn.Name(), p.typeArgsToWrite(call, fn))})
		}
	}
}

// knownTypeArgs returns the generic function that call calls and, for each of its type parameters,
// whether its type argument is known to the lowering (lower's opaqueInstRef and shapeArgs): written,
// or inferred by go/types as a Go type without C/C++ parts, or given by a written slice or map type
// argument (F[[]std.string] gives T of S ~[]T); else only C++ can deduce it. With written >= 0 the
// type arguments are taken as if the first written of them were written (typeArgsToWrite). It returns
// nil when call calls no generic function.
func (p *Package) knownTypeArgs(call *ast.CallExpr, written int) (*types.Func, []bool) {
	id := instanceIdent(call.Fun)
	if id == nil {
		return nil, nil
	}
	fn, ok := p.Info.Uses[id].(*types.Func)
	if !ok || fn.Signature().TypeParams().Len() == 0 {
		return nil, nil
	}
	idx := typeArgExprs(ast.Unparen(call.Fun))
	tparams := fn.Signature().TypeParams()
	inst, hasInst := p.Info.Instances[id]
	known := make([]bool, tparams.Len())
	for i := range known {
		switch {
		case i < len(idx) || i < written:
			known[i] = true
		case hasInst && i < inst.TypeArgs.Len():
			t := inst.TypeArgs.At(i)
			known[i] = cxxComplete(t) && !typeHasC(t, nil)
		}
	}
	for i := range min(max(len(idx), written), tparams.Len()) {
		core := CoreType(tparams.At(i).Constraint())
		var parts []types.Type
		switch c := core.(type) {
		case *types.Slice:
			parts = []types.Type{c.Elem()}
		case *types.Map:
			parts = []types.Type{c.Key(), c.Elem()}
		}
		if i < len(idx) {
			// A written type argument gives its parts when it is a slice or map of that shape (in its
			// syntax, []std.string, or in its type, []Holder).
			shaped := false
			switch x := ast.Unparen(idx[i]).(type) {
			case *ast.ArrayType:
				_, shaped = core.(*types.Slice)
				shaped = shaped && x.Len == nil
			case *ast.MapType:
				_, shaped = core.(*types.Map)
			}
			switch types.Unalias(p.Info.TypeOf(idx[i])).(type) {
			case *types.Slice:
				_, isSlice := core.(*types.Slice)
				shaped = shaped || isSlice
			case *types.Map:
				_, isMap := core.(*types.Map)
				shaped = shaped || isMap
			}
			if !shaped {
				parts = nil
			}
		}
		for _, part := range parts {
			if tp, ok := types.Unalias(part).(*types.TypeParam); ok && tp.Index() < tparams.Len() && tparams.At(tp.Index()) == tp {
				known[tp.Index()] = true
			}
		}
	}
	return fn, known
}

// typeArgsToWrite returns the type arguments of the call of fn that a program must write so that every
// type argument is known (knownTypeArgs): the fewest first ones, each as the source text of the
// argument it comes from when the call has one (Seq(words) with var words []std.string: std.string;
// Keys(map[int]std.string{}): map[int]std.string), else the name of the type parameter.
func (p *Package) typeArgsToWrite(call *ast.CallExpr, fn *types.Func) []string {
	tparams := fn.Signature().TypeParams()
	n := tparams.Len()
	for k := 1; k < tparams.Len(); k++ {
		if _, known := p.knownTypeArgs(call, k); !slices.Contains(known, false) {
			n = k
			break
		}
	}
	idx := typeArgExprs(ast.Unparen(call.Fun))
	out := make([]string, n)
	for i := range n {
		if i < len(idx) {
			out[i] = p.SourceText(idx[i])
			continue
		}
		out[i] = tparams.At(i).Obj().Name()
		if text := p.argTypeText(call, fn, tparams.At(i)); text != "" {
			out[i] = text
		}
	}
	return out
}

// argTypeText returns the source text of the type that an argument of call gives the type parameter
// tp of fn: the written type of the argument of a parameter of type tp, or the element type of one of
// type []tp (a composite literal's type, or the declared type of a variable), or "".
func (p *Package) argTypeText(call *ast.CallExpr, fn *types.Func, tp *types.TypeParam) string {
	sig := fn.Signature()
	if call.Ellipsis.IsValid() || len(call.Args) != sig.Params().Len() {
		return ""
	}
	for k, arg := range call.Args {
		if sig.Variadic() && k == sig.Params().Len()-1 {
			break
		}
		var te ast.Expr // the argument's written type
		switch a := ast.Unparen(arg).(type) {
		case *ast.CompositeLit:
			te = a.Type
		case *ast.Ident:
			if obj := p.Info.Uses[a]; obj != nil {
				te = p.Opaque.TypeExpr(obj)
				if te == nil {
					te = p.pkgVarLitType(obj) // var words = []std.string{"a"}
				}
			}
		}
		if te == nil {
			continue
		}
		switch pt := types.Unalias(sig.Params().At(k).Type()).(type) {
		case *types.TypeParam:
			if pt == tp {
				return p.SourceText(te)
			}
		case *types.Slice:
			if at, ok := te.(*ast.ArrayType); ok && at.Len == nil && types.Unalias(pt.Elem()) == tp {
				return p.SourceText(at.Elt)
			}
		}
	}
	return ""
}

// pkgVarLitType returns the written type of the composite literal that the package variable obj is
// declared from without a type, or nil.
func (p *Package) pkgVarLitType(obj types.Object) ast.Expr {
	for _, init := range p.Info.InitOrder {
		if len(init.Lhs) == 1 && init.Lhs[0] == obj {
			if lit, ok := ast.Unparen(init.Rhs).(*ast.CompositeLit); ok {
				return lit.Type
			}
			return nil
		}
	}
	return nil
}

// cxxComplete reports whether the lowering writes the C++ type of t from t itself (lower's complete):
// no part of t is invalid, and no named type in it has an invalid underlying type (type H C.int).
func cxxComplete(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return t.Kind() != types.Invalid
	case *types.Alias:
		return cxxComplete(types.Unalias(t))
	case *types.Named:
		b, ok := t.Underlying().(*types.Basic)
		return !ok || b.Kind() != types.Invalid
	case *types.Pointer:
		return cxxComplete(t.Elem())
	case *types.Slice:
		return cxxComplete(t.Elem())
	case *types.Array:
		return cxxComplete(t.Elem())
	case *types.Chan:
		return cxxComplete(t.Elem())
	case *types.Map:
		return cxxComplete(t.Key()) && cxxComplete(t.Elem())
	case *types.Struct:
		for i := range t.NumFields() {
			if !cxxComplete(t.Field(i).Type()) {
				return false
			}
		}
	case *types.Signature:
		return cxxComplete(t.Params()) && cxxComplete(t.Results())
	case *types.Tuple:
		for i := range t.Len() {
			if !cxxComplete(t.At(i).Type()) {
				return false
			}
		}
	}
	return true
}
