package adapt

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// GoFacts are the answers about the Go program that the classification of an argument needs. The
// lowering and the checker both have them (go/types' result and the opaque pass of
// internal/check), each in its own form.
type GoFacts interface {
	// Operand returns what go/types recorded for e (the zero value when nothing).
	Operand(e ast.Expr) types.TypeAndValue
	// Opaque reports whether e is a C/C++ value.
	Opaque(e ast.Expr) bool
	// Holder reports whether e is a holder of C.cstr or C.wstr: a call of one, or a variable that
	// holds one. A holder is passed as it is.
	Holder(e ast.Expr) bool
	// Variable reports whether e names a variable or a field of a struct: the forms of &x whose
	// pointee is exactly one value (never an element of an array, which a C array parameter would
	// take several of).
	Variable(e ast.Expr) bool
	// CPtrVariable reports whether e is a variable declared with a pointer to a C/C++ type
	// (var p *lib.T: a gp::cptr<T>). A side that cannot tell says false.
	CPtrVariable(e ast.Expr) bool
}

// Describe returns the argument a as the decision sees it: its class, and the constant or the
// variable of its class (text writes the variable as the program did). CharPtrAddr is the caller's
// to set.
func Describe(f GoFacts, a ast.Expr, text func(ast.Expr) string) Arg {
	class, tv, x, kind := classOf(f, a)
	out := Arg{Class: class}
	if class == ClassNone {
		return out
	}
	switch class {
	case ClassNumConst:
		out.Const = tv
	case ClassAddrScalar:
		out.Var = text(x)
		out.VarKind = kind
	}
	return out
}

// classOf returns the class of the Go argument a, with the type and value of a constant and, for
// &x, the variable x and its Go number kind.
func classOf(f GoFacts, a ast.Expr) (Class, types.TypeAndValue, ast.Expr, types.BasicKind) {
	tv := f.Operand(a)
	switch {
	case tv.Type == nil || tv.Type == types.Typ[types.Invalid]:
		// &p of a pointer to a C type has no Go type (the C part is invalid): only the form
		// decides.
	case tv.Value != nil && tv.Value.Kind() == constant.String:
		if f.Holder(a) {
			return ClassNone, tv, nil, 0
		}
		return ClassStringConst, tv, nil, 0
	case tv.Value != nil && (tv.Value.Kind() == constant.Int || tv.Value.Kind() == constant.Float):
		if b, ok := tv.Type.(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
			return ClassNumConst, tv, nil, 0
		}
		return ClassNone, tv, nil, 0
	case tv.Value != nil:
		return ClassNone, tv, nil, 0
	case GoStringValue(f, a):
		return ClassString, tv, nil, 0
	case goFloatValue(f, a) == types.Float32:
		return ClassFloat32, tv, nil, 0
	case goFloatValue(f, a) == types.Float64:
		return ClassFloat64, tv, nil, 0
	}
	u, ok := ast.Unparen(a).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return ClassNone, tv, nil, 0
	}
	if f.Variable(u.X) {
		if kind, ok := goNumberKind(f, u.X); ok {
			return ClassAddrScalar, tv, u.X, kind
		}
		if f.CPtrVariable(u.X) {
			return ClassAddrCPtr, tv, u.X, 0
		}
	}
	return ClassNone, tv, nil, 0
}

// GoStringValue reports whether e is a gocpp string that is not a constant (a constant is a C
// literal already): a variable, a call, an operator's result of type string.
func GoStringValue(f GoFacts, e ast.Expr) bool {
	tv := f.Operand(e)
	if tv.Type == nil || tv.Value != nil || f.Opaque(e) || f.Holder(e) {
		return false
	}
	return types.Identical(tv.Type, types.Typ[types.String])
}

// goFloatValue returns types.Float32 or types.Float64 if e is a value of exactly that Go type (not
// a constant, not a named type, not a C/C++ value), else types.Invalid.
func goFloatValue(f GoFacts, e ast.Expr) types.BasicKind {
	tv := f.Operand(e)
	if tv.Type == nil || tv.Value != nil || !tv.IsValue() || f.Opaque(e) {
		return types.Invalid
	}
	b, ok := tv.Type.(*types.Basic)
	if !ok || (b.Kind() != types.Float32 && b.Kind() != types.Float64) {
		return types.Invalid
	}
	return b.Kind()
}

// goNumberKind returns the basic kind of e if it is a variable of a Go number type (int, uint8,
// float32, ...: not a named type, not a C type, not a constant).
func goNumberKind(f GoFacts, e ast.Expr) (types.BasicKind, bool) {
	tv := f.Operand(e)
	if tv.Type == nil || tv.Value != nil || f.Opaque(e) {
		return 0, false
	}
	b, ok := tv.Type.(*types.Basic)
	if !ok || b.Info()&types.IsNumeric == 0 || b.Info()&(types.IsComplex|types.IsUntyped) != 0 {
		return 0, false
	}
	return b.Kind(), true
}

// FuncSigs returns the overloads of the free function name (the qualified name the declarations
// are keyed by: "snprintf", "ImGui::Text"), or false when there is none or one has no signature
// (a template).
func FuncSigs(decls *probe.DeclTable, name string) ([]Sig, bool) {
	if decls == nil {
		return nil, false
	}
	fs := decls.Funcs[name]
	if fs == nil || len(fs.Overloads) == 0 {
		return nil, false
	}
	var out []Sig
	for _, o := range fs.Overloads {
		if o.Kind != "function" || o.Sig.T == nil || o.Sig.T.Kind != ctype.Func {
			return nil, false
		}
		out = append(out, Sig{
			T: o.Sig.T, Scope: fs.Scope, FmtKind: o.FormatKind, FmtParam: o.FormatParam,
			Arrays: o.ArrayParams,
		})
	}
	return out, true
}

// MethodSigs returns the overloads of the method called name of the class that the typed value v
// is an object (or a pointer to one) of, with the methods of its bases; false when there is none,
// or one is a template.
func MethodSigs(m *ftype.Mapper, v *ftype.Value, name string) ([]Sig, bool) {
	if v == nil {
		return nil, false
	}
	t := v.Type
	if v.Kind == ftype.Pointer && v.Elem != nil {
		t = v.Elem.Type
	} else if v.Kind != ftype.Record {
		return nil, false
	}
	mem := m.MembersOf(t).Lookup(name)
	if mem == nil || mem.Kind != "method" {
		return nil, false
	}
	scope := m.Scope(t)
	var out []Sig
	for _, meth := range mem.Methods {
		if meth.Template || meth.Sig.C == nil || meth.Sig.C.Kind != ctype.Func {
			return nil, false
		}
		out = append(out, Sig{T: meth.Sig.C, Scope: scope, Arrays: methodArrays(meth)})
	}
	return out, len(out) > 0
}

// methodArrays returns which parameters of the method are written as arrays, or nil when the
// method's parameters are not all known.
func methodArrays(m *ftype.MethodInfo) []bool {
	if len(m.Params) != len(m.Sig.C.Params) {
		return nil
	}
	out := make([]bool, len(m.Params))
	for i, p := range m.Params {
		out[i] = p.Array
	}
	return out
}

// AliasUseOK reports whether the call c, of a function that returns a pointer into its argument,
// is used where the result is gone by the end of the statement: compared, passed on, or the
// statement itself. Assigned to a variable, stored or returned, it would outlive the string.
// parent returns the node whose direct child a node is.
func AliasUseOK(parent func(ast.Node) ast.Node, c *ast.CallExpr) bool {
	p := parent(c)
	for {
		switch x := p.(type) {
		case *ast.ParenExpr:
			p = parent(x)
			continue
		case *ast.BinaryExpr, *ast.CallExpr, *ast.ExprStmt, *ast.IfStmt, *ast.SwitchStmt, *ast.ForStmt,
			*ast.IndexExpr, *ast.StarExpr, *ast.SelectorExpr, *ast.UnaryExpr:
			return true
		}
		return false
	}
}

// MethodKeepsCharPtr reports whether the object v (a class value or a pointer to one) is of a class
// of the program (not the library's) that has a data member that is a char pointer: a setter such
// as set(const char*) is then likely to store the pointer, and nothing in its declaration says
// otherwise. The text is the reason.
func MethodKeepsCharPtr(m *ftype.Mapper, v *ftype.Value, cs CharSet) (string, bool) {
	t := v.Type
	if v.Kind == ftype.Pointer && v.Elem != nil {
		t = v.Elem.Type
	} else if v.Kind != ftype.Record {
		return "", false
	}
	if inStdScope(m.Scope(t)) {
		return "", false
	}
	rec := m.Decls().Records[m.Canonical(t)]
	if rec == nil {
		return "", false
	}
	for _, f := range rec.Fields {
		if ft := f.Type.T; ft != nil && ft.Kind == ctype.Pointer && cs.Has(ft.Elem) {
			return "the class has a char pointer member, " + f.Name, true
		}
	}
	return "", false
}

// inStdScope reports whether the namespace scope is std or inside it.
func inStdScope(scope string) bool { return scope == "std" || strings.HasPrefix(scope, "std::") }

// IsVariableName reports whether e names a variable or a field of a struct: the forms of &x whose
// pointee is exactly one value (never an element of an array, which a C array parameter would take
// several of).
func IsVariableName(info *types.Info, e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		v, ok := info.Uses[x].(*types.Var)
		return ok && !v.IsField()
	case *ast.SelectorExpr:
		if s := info.Selections[x]; s != nil {
			return s.Kind() == types.FieldVal
		}
		_, ok := info.Uses[x.Sel].(*types.Var) // pkg.v
		return ok
	}
	return false
}

// CPtrDeclared reports whether the declared type te of a variable is a pointer to a C/C++ type
// (var p *lib.T: a gp::cptr<T>), not a pointer to a pointer. isCType says whether a type
// expression names a C/C++ type. The lowering and the checker both ask this of the declared type
// they know (the lowering follows more: a parameter, a variable that gets its type from its value;
// the checker only a written declaration, so whatever it finds the lowering finds too).
func CPtrDeclared(te ast.Expr, isCType func(ast.Expr) bool) bool {
	star, ok := ast.Unparen(te).(*ast.StarExpr)
	if !ok || !isCType(star.X) {
		return false
	}
	_, nested := ast.Unparen(star.X).(*ast.StarExpr)
	return !nested
}

// CharPtrAddr returns the C++ name of the character type of the pointer whose address the argument
// arg is (&end of a *C.char variable: char **), or "". typed is what the probe says of arg (nil
// when nothing); declared returns the declared type of a variable or field expression, and isC
// says whether an expression is the package name C.
func CharPtrAddr(typed *ftype.Value, arg ast.Expr, declared func(ast.Expr) ast.Expr,
	isC func(ast.Expr) bool) string {
	if typed != nil && typed.Kind == ftype.Pointer && typed.Elem != nil &&
		typed.Elem.Kind == ftype.Pointer && typed.Elem.Elem != nil {
		if c := typed.Elem.Elem.C; c != nil && c.Kind == ctype.Builtin {
			return c.Name
		}
	}
	u, ok := ast.Unparen(arg).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return ""
	}
	te := declared(u.X)
	if te == nil {
		return ""
	}
	st, ok := ast.Unparen(te).(*ast.StarExpr)
	if !ok {
		return ""
	}
	sel, ok := ast.Unparen(st.X).(*ast.SelectorExpr)
	if !ok || !isC(sel.X) {
		return ""
	}
	return CharTypeName(sel.Sel.Name)
}
