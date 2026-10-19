package lower

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// Untyped constants at C targets follow Go's rules, as in cgo (Phase 04 step 4.9): an untyped
// constant that goes to a variable, a parameter, a result or a field declared with one of C's
// built-in types (C.char, C.int, C.double, ...) must be representable in it, and one that goes
// to a C pointer type must be nil, else it is Go's error at the constant:
//
//	var c C.char = 200      cannot use 200 (untyped int constant) as C.char value in variable declaration (overflows)
//	var n C.int = 2.5       cannot use 2.5 (untyped float constant) as C.int value in variable declaration (truncated)
//	var s *C.char = "hi"    cannot use "hi" (untyped string constant) as *C.char value in variable declaration
//	C.int(2.5)              cannot convert 2.5 (untyped float constant) to type C.int
//	C.char(200)             constant 200 overflows C.char
//
// C++ would convert such a constant silently (200 becomes -56; 2.5 becomes 2). The types are
// cgo's on the targets of gocpp (Linux and Windows, x86-64): char is signed. C.long and C.ulong
// are 64 bits on Linux and 32 bits on Windows (LLP64, cConstType; step 14.4). A constant for any other C/C++ type (a typedef of a header, std.string) is the C++
// compiler's to check, and so is one passed to a C/C++ function, whose parameter types gocpp
// doesn't know in opaque mode.

// cConstTypes are the Go types with the ranges of C's built-in types (cBuiltins).
var cConstTypes = map[string]types.BasicKind{
	"char": types.Int8, "schar": types.Int8, "uchar": types.Uint8,
	"short": types.Int16, "ushort": types.Uint16, "int": types.Int32, "uint": types.Uint32,
	"long": types.Int64, "ulong": types.Uint64, "longlong": types.Int64, "ulonglong": types.Uint64,
	"float": types.Float32, "double": types.Float64, "size_t": types.Uint64, "wchar_t": types.Int32,
}

// cConstType returns the Go type with the range of the C built-in type name for the target of the
// package (cConstTypes are Linux's; Windows is LLP64: long and unsigned long have 32 bits, and
// wchar_t is the unsigned 16-bit UTF-16 unit, step 14.4).
func (g *gen) cConstType(name string) (types.BasicKind, bool) {
	if g.pkg.Load != nil && g.pkg.Load.GOOS == "windows" {
		switch name {
		case "long":
			return types.Int32, true
		case "ulong":
			return types.Uint32, true
		case "wchar_t":
			return types.Uint16, true
		}
	}
	kind, ok := cConstTypes[name]
	return kind, ok
}

// cConstOK reports whether the untyped constant e (tv is its type and value) may go where it
// goes, in its place parent, and reports Go's error if it may not (see above).
func (w *cWalk) cConstOK(e ast.Expr, tv types.TypeAndValue, parent ast.Node) bool {
	g := w.g
	switch p := parent.(type) {
	case *ast.BinaryExpr:
		// c + 300, c == 300 with var c C.char: the constant gets the type of c, as in Go.
		switch p.Op {
		case token.SHL, token.SHR, token.LAND, token.LOR:
			return true
		}
		other := p.X
		if ast.Unparen(p.X) == e {
			other = p.Y
		}
		return g.cOperandConstOK(e, tv, other, p.X, p.Op.String(), p.Y)
	case *ast.AssignStmt:
		// x += 1 << 40 with var x C.int.
		switch p.Tok {
		case token.ASSIGN, token.DEFINE, token.SHL_ASSIGN, token.SHR_ASSIGN:
		default:
			if len(p.Rhs) == 1 && ast.Unparen(p.Rhs[0]) == e {
				return g.cOperandConstOK(e, tv, p.Lhs[0], p.Lhs[0], p.Tok.String(), p.Rhs[0])
			}
		}
	}
	t := w.targetOf(e, parent)
	if t.multi || t.to != nil && complete(t.to) {
		return true
	}
	te := t.texpr
	if te == nil && t.v != nil {
		te = g.typeExprOf(t.v)
	}
	if te == nil {
		return true
	}
	ts := g.srcText(te)
	switch x := ast.Unparen(te).(type) {
	case *ast.StarExpr:
		g.userError(e.Pos(), "cannot use %s as %s value in %s", constDesc(g.srcText(e), tv, tv.Value), ts, t.context)
		return false
	case *ast.SelectorExpr:
		kind, ok := g.cConstType(x.Sel.Name)
		if !ok || !g.isC(x.X) {
			return true
		}
		if why, ok := g.cRepresentable(e, tv, kind); !ok {
			g.userError(e.Pos(), "cannot use %s as %s value in %s%s", constDesc(g.srcText(e), tv, tv.Value), ts, t.context, why)
			return false
		}
	}
	return true
}

// cOperandConstOK reports whether the untyped constant e (tv is its type and value), an operand
// of the operator op (x op y, also x op= y) next to the operand other, fits the type of other if
// other was declared with one of C's built-in types or a C pointer type, and reports go/types'
// error if it doesn't: 300 (untyped int constant) overflows C.char.
func (g *gen) cOperandConstOK(e ast.Expr, tv types.TypeAndValue, other, x ast.Expr, op string, y ast.Expr) bool {
	kind, ts := g.cKind(other)
	if kind == "" {
		return true
	}
	mismatched := func() bool {
		left, right := ts, tv.Type.String()
		if ast.Unparen(x) == e {
			left, right = right, left
		}
		g.userError(x.Pos(), "invalid operation: %s %s %s (mismatched types %s and %s)", g.srcText(x), op, g.srcText(y), left, right)
		return false
	}
	if kind == "pointer" {
		return mismatched() // p == 0: only nil goes with a C pointer
	}
	b, ok := g.cConstType(ts[len("C."):])
	if !ok {
		return true
	}
	why, ok := g.cRepresentable(e, tv, b)
	switch {
	case ok:
		return true
	case why == "":
		return mismatched() // c + "x", c == true
	case why == " (truncated)":
		g.userError(e.Pos(), "%s truncated to %s", constDesc(g.srcText(e), tv, tv.Value), ts)
	default:
		g.userError(e.Pos(), "%s overflows %s", constDesc(g.srcText(e), tv, tv.Value), ts)
	}
	return false
}

// cConstConversion checks an explicit conversion to a C built-in type. Unlike assignment,
// conversion accepts typed constants, but their values must still be representable in Go.
func (g *gen) cConstConversion(e ast.Expr, tv types.TypeAndValue, parent ast.Node) (bool, bool) {
	c, ok := parent.(*ast.CallExpr)
	if !ok || len(c.Args) != 1 || ast.Unparen(c.Args[0]) != e {
		return false, true
	}
	if r, ok := g.cRefOf(c.Fun); ok && r.kind == interop.Type {
		if n, ok := r.x.(*cppir.Name); ok && n.Args == nil && isBuiltinC(n.Name) {
			return true, g.cConvConstOK(e, tv, r.src)
		}
	}
	return false, true
}

// cRepresentable records a known numeric target when checking succeeds. Lowering uses it to
// emit an exact integer, not a double rounded on the way to an integer (9007199254740993+0i).
func (g *gen) cRepresentable(e ast.Expr, tv types.TypeAndValue, kind types.BasicKind) (string, bool) {
	why, ok := representable(tv, kind)
	if ok {
		if g.cConstKinds == nil {
			g.cConstKinds = make(map[ast.Expr]types.BasicKind)
		}
		g.cConstKinds[ast.Unparen(e)] = kind
	}
	return why, ok
}

// cConvConstOK reports whether the conversion to the C type to (C.int) of the constant
// e is one that Go allows (a constant conversion: the value must be representable), and reports
// Go's error if it isn't.
func (g *gen) cConvConstOK(e ast.Expr, tv types.TypeAndValue, to string) bool {
	kind, _ := g.cConstType(to[len("C."):])
	if _, ok := g.cRepresentable(e, tv, kind); ok {
		return true
	}
	src := underBasic(tv.Type)
	if src != nil && src.Info()&types.IsInteger != 0 && types.Typ[kind].Info()&types.IsInteger != 0 {
		g.userError(e.Pos(), "constant %s overflows %s", tv.Value.String(), to)
		return false
	}
	// go/types shows the value rounded to an integer when it is one (1e10 is 10000000000).
	v := tv.Value
	if i := constant.ToInt(v); i.Kind() == constant.Int && types.Typ[kind].Info()&types.IsInteger != 0 {
		v = i
	}
	g.userError(e.Pos(), "cannot convert %s to type %s", constDesc(g.srcText(e), tv, v), to)
	return false
}

// representable reports whether the constant tv is representable in a value of the
// basic type kind, and why not as go/types says it: " (overflows)" or " (truncated)" for a
// number (truncated: a float or complex constant for an integer type, or a complex one with an
// imaginary part), or "" for a constant of another kind (a string, a bool).
func representable(tv types.TypeAndValue, kind types.BasicKind) (string, bool) {
	v := tv.Value
	src := underBasic(tv.Type)
	info := types.Typ[kind].Info()
	if src == nil || src.Info()&types.IsNumeric == 0 {
		return "", false
	}
	why := " (overflows)"
	if src.Info()&types.IsInteger == 0 && info&types.IsInteger != 0 {
		why = " (truncated)"
	}
	if v.Kind() == constant.Complex {
		if constant.Sign(constant.Imag(v)) != 0 {
			return " (truncated)", false
		}
		v = constant.Real(v) // 1 + 0i
	}
	switch {
	case info&types.IsInteger != 0:
		i := constant.ToInt(v)
		if i.Kind() != constant.Int {
			return " (truncated)", false
		}
		size := map[types.BasicKind]uint{types.Int8: 8, types.Uint8: 8, types.Int16: 16, types.Uint16: 16,
			types.Int32: 32, types.Uint32: 32, types.Int64: 64, types.Uint64: 64}[kind]
		lo, hi := constant.MakeInt64(0), constant.Shift(constant.MakeInt64(1), token.SHL, size)
		if info&types.IsUnsigned == 0 {
			hi = constant.Shift(constant.MakeInt64(1), token.SHL, size-1)
			lo = constant.UnaryOp(token.SUB, hi, 0)
		}
		if constant.Compare(i, token.LSS, lo) || constant.Compare(i, token.GEQ, hi) {
			return why, false
		}
	default:
		f := constant.ToFloat(v)
		if f.Kind() != constant.Float && f.Kind() != constant.Int {
			return why, false
		}
		if kind == types.Float32 {
			if x, _ := constant.Float32Val(f); math.IsInf(float64(x), 0) {
				return why, false
			}
		} else if x, _ := constant.Float64Val(f); math.IsInf(x, 0) {
			return why, false
		}
	}
	return "", true
}

// constDesc returns the untyped constant src (tv is its type, v its value) as go/types describes
// an operand: 200 (untyped int constant), 1e300 (untyped float constant 1e+300).
func constDesc(src string, tv types.TypeAndValue, v constant.Value) string {
	if b := underBasic(tv.Type); b != nil && b.Info()&types.IsUntyped == 0 {
		return fmt.Sprintf("%s (constant %s of type %s)", src, v.String(), tv.Type.String())
	}
	name := "untyped constant"
	if b, ok := tv.Type.(*types.Basic); ok {
		name = b.Name() + " constant" // untyped int
	}
	if s := v.String(); s != src {
		name += " " + s
	}
	return fmt.Sprintf("%s (%s)", src, name)
}

// valueTypeExpr recovers only a type that gocpp declarations establish. go/types loses the
// C part of a container's element or a pointer, so inspect its type expression recursively.
// C++ operator results, and calls or members with opaque declarations, supply no type here:
// promotions and overloads can change the result type. C++ checks those results.
func (g *gen) valueTypeExpr(e ast.Expr) ast.Expr {
	return g.valueTypeExprSeen(e, make(map[*types.Var]bool))
}

func (g *gen) valueTypeExprSeen(e ast.Expr, seen map[*types.Var]bool) ast.Expr {
	var te ast.Expr
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident, *ast.SelectorExpr:
		v := g.varOf(e)
		if v == nil || seen[v] {
			return nil
		}
		seen[v] = true
		te = g.typeExprOf(v.Origin())
		if te == nil {
			te = g.valueTypeExprSeen(g.initialValue(v), seen)
		}
	case *ast.IndexExpr:
		container := g.valueTypeExprSeen(x.X, seen)
		if p, ok := ast.Unparen(container).(*ast.StarExpr); ok {
			container = p.X
		}
		switch t := ast.Unparen(container).(type) {
		case *ast.ArrayType:
			te = t.Elt
		case *ast.MapType:
			te = t.Value
		}
	case *ast.StarExpr:
		if p, ok := ast.Unparen(g.valueTypeExprSeen(x.X, seen)).(*ast.StarExpr); ok {
			te = p.X
		}
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			if elem := g.valueTypeExprSeen(x.X, seen); elem != nil {
				te = &ast.StarExpr{Star: x.OpPos, X: elem}
			}
		}
	case *ast.SliceExpr:
		container := g.valueTypeExprSeen(x.X, seen)
		if p, ok := ast.Unparen(container).(*ast.StarExpr); ok {
			container = p.X
		}
		if a, ok := ast.Unparen(container).(*ast.ArrayType); ok {
			te = &ast.ArrayType{Lbrack: a.Lbrack, Elt: a.Elt}
		}
	case *ast.CompositeLit:
		te, _ = g.litTypeExpr(x)
	case *ast.CallExpr:
		if r, ok := g.cRefOf(x.Fun); ok && r.kind == interop.Type {
			te = x.Fun
		} else if sig := g.calledSig(x); sig != nil && sig.Results().Len() == 1 {
			te = g.typeExprOf(sig.Results().At(0))
		} else if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok {
			if b, ok := g.info.Uses[id].(*types.Builtin); ok && len(x.Args) > 0 {
				switch b.Name() {
				case "make":
					te = x.Args[0]
				case "new":
					te = &ast.StarExpr{Star: x.Pos(), X: x.Args[0]}
				case "append":
					te = g.valueTypeExprSeen(x.Args[0], seen)
				}
			}
		}
	}

	// Named gocpp containers can carry C elements too (type Chars [1]C.char).
	for {
		id, ok := ast.Unparen(te).(*ast.Ident)
		if !ok {
			break
		}
		obj, ok := g.info.Uses[id].(*types.TypeName)
		if !ok {
			break
		}
		next := g.pkg.Opaque.TypeExpr(obj)
		if next == nil || next == te {
			break
		}
		te = next
	}
	return te
}

// initialValue indexes declarations once, also inferred pointers (p := &n), containers made
// with make, and aliases of known C values. Only a one-to-one declaration supplies a value.
func (g *gen) initialValue(v *types.Var) ast.Expr {
	if g.initialValues == nil {
		g.initialValues = make(map[*types.Var]ast.Expr)
		add := func(names, values []ast.Expr) {
			if len(names) != len(values) {
				return
			}
			for i, n := range names {
				if id, ok := n.(*ast.Ident); ok {
					if obj, ok := g.info.Defs[id].(*types.Var); ok {
						g.initialValues[obj] = values[i]
					}
				}
			}
		}
		for _, f := range g.pkg.Load.Files {
			ast.Inspect(f.AST, func(n ast.Node) bool {
				switch d := n.(type) {
				case *ast.AssignStmt:
					if d.Tok == token.DEFINE {
						add(d.Lhs, d.Rhs)
					}
				case *ast.ValueSpec:
					if d.Type == nil {
						names := make([]ast.Expr, len(d.Names))
						for i, n := range d.Names {
							names[i] = n
						}
						add(names, d.Values)
					}
				}
				return true
			})
		}
	}
	return g.initialValues[v]
}
