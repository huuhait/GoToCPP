package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// Go strings as arguments of C/C++ functions (Phase 12 step 12.6, README §2.5).
//
//	cstdio.puts(s)          ::puts(gp::cstr(s).gp_ptr())      a const char* parameter
//	cstdio.printf("%s", s)  ::printf("%s", gp::cstr(s).gp_ptr())   a variadic argument
//	takes(s)                takes(::std::string(s))           a std::string parameter
//
// In opaque mode a gocpp string goes to a std::string_view parameter by itself (gp::string
// converts to it), and anywhere else the user writes C.cstr(s) or std.string(s). Typed mode knows
// the parameters. The conversion is made only when **every** overload that has a parameter at
// that place has the same kind of parameter (a const char* or a std::string): then the opaque
// build would not compile the call, and the typed one cannot change what it means. A set that
// mixes kinds (f(const char*) next to f(std::string_view)) is left to C++'s overload
// resolution on the gocpp string, which prefers the string_view as in opaque mode. A function
// template, a function pointer variable and a name without declarations are left alone.
//
// The C string is gp::cstr's: a copy with a NUL at the end when the string is not
// NUL-terminated already, alive until the end of the statement that contains the call. A
// function that keeps the pointer longer (keeps.go) or returns a pointer into it, whose result is
// then stored, is a user error with the hint to use C.CString, as the opaque lowering asks for
// anyway. A NUL byte inside the string ends the C string there, as with C.CString and C.cstr.

type argConv int

const (
	convNone      argConv = iota
	convCStr              // gp::cstr(s).gp_ptr(): a const char* parameter, or a C variadic argument
	convStdString         // ::std::string(s)

	// convNoStr is a parameter that a gocpp string can never go to (a number, a pointer to
	// anything but const char, a wide string): an overload with it is no candidate for the string.
	convNoStr
)

// typedStringArgs returns the C++ of the arguments args of the call c of a C/C++ function with
// the conversions of Go strings made (see above); args is cArgs' result for c.Args. It returns args
// unchanged when typed mode is off or has nothing to convert.
func (fg *funcGen) typedStringArgs(c *ast.CallExpr, args []cppir.Expr) []cppir.Expr {
	g := fg.g
	if !g.typedOn() || c.Ellipsis.IsValid() {
		return args
	}
	var convs []argConv
	name := ""
	for i, a := range c.Args {
		if !g.goStringValue(a) {
			continue
		}
		if convs == nil {
			var ok bool
			if convs, name, ok = g.paramConvs(c); !ok {
				return args
			}
		}
		if i >= len(convs) || convs[i] == convNone {
			continue
		}
		if convs[i] == convCStr {
			if why, keeps := keepsPointer(name, len(c.Args), func(n int) string { return g.srcText(c.Args[n]) }); keeps {
				g.userError(a.Pos(), "%s keeps the pointer that it is given (%s), and a Go string's bytes live only until the end of the statement: use C.CString(%s) and C.free it when the library is done with it", name, why, g.srcText(a))
				return args
			}
			if why, keeps := g.methodKeepsCharPtr(c); keeps {
				g.userError(a.Pos(), "%s may keep the pointer that it is given (%s), and a Go string's bytes live only until the end of the statement: use C.CString(%s) and C.free it when the object is done with it", name, why, g.srcText(a))
				return args
			}
			if out := g.charPtrOutParam(c, i); out != nil {
				g.userError(out.Pos(), "%s is given the address of a char pointer (%s) next to the string %s: the pointer that it stores can point into the string, whose C string lives only until the end of the statement: use C.CString(%s) and C.free it when you are done with the pointer", name, g.srcText(out), g.srcText(a), g.srcText(a))
				return args
			}
			if aliasFuncs[lastName(name)] && !g.aliasUseOK(c) {
				g.userError(c.Pos(), "%s returns a pointer into its argument %s, which as a C string lives only until the end of the statement: use it in the statement, or keep the string alive with a variable, h := C.cstr(%s)", name, g.srcText(a), g.srcText(a))
				return args
			}
			args[i] = &cppir.Call{Fun: &cppir.Member{X: call("gp::cstr", args[i]), Name: "gp_ptr"}}
			continue
		}
		args[i] = &cppir.Call{Fun: cxxName("::std::string"), Args: []cppir.Expr{args[i]}}
	}
	return args
}

// lastName returns the last part of a qualified C++ name.
func lastName(name string) string {
	if j := strings.LastIndex(name, "::"); j >= 0 {
		return name[j+2:]
	}
	return name
}

// goStringValue reports whether e is a gocpp string that is not a constant (a constant is a C
// literal already, cArgs): a variable, a call, an operator's result of type string.
func (g *gen) goStringValue(e ast.Expr) bool {
	tv, ok := g.info.Types[e]
	if !ok || tv.Value != nil || tv.Type == nil || !complete(tv.Type) || g.pkg.Opaque.IsOpaque(e) || g.isHolder(e) {
		return false
	}
	return types.Identical(tv.Type, types.Typ[types.String])
}

// aliasUseOK reports whether the call c, of a function that returns a pointer into its argument,
// is used where the result is gone by the end of the statement: compared, passed on, or the
// statement itself. Assigned to a variable, stored or returned, it would outlive the string.
func (g *gen) aliasUseOK(c *ast.CallExpr) bool {
	parent := g.parentOf(c)
	for {
		switch p := parent.(type) {
		case *ast.ParenExpr:
			parent = g.parentOf(p)
			continue
		case *ast.BinaryExpr, *ast.CallExpr, *ast.ExprStmt, *ast.IfStmt, *ast.SwitchStmt, *ast.ForStmt, *ast.IndexExpr, *ast.StarExpr, *ast.SelectorExpr, *ast.UnaryExpr:
			return true
		}
		return false
	}
}

// parentOf returns the node whose direct child is n (the package's syntax tree), or nil.
func (g *gen) parentOf(n ast.Node) ast.Node {
	if g.parents == nil {
		g.parents = make(map[ast.Node]ast.Node)
		for _, f := range g.pkg.Load.Files {
			var stack []ast.Node
			ast.Inspect(f.AST, func(x ast.Node) bool {
				if x == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				if len(stack) > 0 {
					g.parents[x] = stack[len(stack)-1]
				}
				stack = append(stack, x)
				return true
			})
		}
	}
	return g.parents[n]
}

// paramConvs returns, for each argument of the call c of a C/C++ function, the conversion of a Go
// string there (convNone: leave it), with the name of the function. false: the function is not
// known.
func (g *gen) paramConvs(c *ast.CallExpr) ([]argConv, string, bool) {
	sigs, name, ok := g.calleeSigs(c)
	if !ok || len(sigs) == 0 {
		return nil, "", false
	}
	return sigConvs(sigs, len(c.Args)), name, true
}

// sigConvs returns the conversion of a Go string for each of the n arguments of a call of one of
// the overloads sigs.
func sigConvs(sigs []calleeSig, n int) []argConv {
	out := make([]argConv, n)
	for i := 0; i < n; i++ {
		var class argConv
		seen := false
		agree := true
		for _, s := range sigs {
			var pc argConv
			switch {
			case i < len(s.t.Params):
				pc = paramConv(s.t.Params[i], s.scope)
			case s.t.Variadic:
				pc = convCStr // a char* for %s
			default:
				continue // fewer parameters: not viable for this call
			}
			if pc == convNoStr {
				continue
			}
			if !seen {
				class, seen = pc, true
			} else if pc != class {
				agree = false
			}
		}
		if seen && agree {
			out[i] = class
		}
	}
	return out
}

// paramConv returns the conversion for a parameter of type t: const char* is convCStr,
// std::string (by value, const& or &&) convStdString.
func paramConv(t *ctype.Type, scope string) argConv {
	if t == nil {
		return convNone
	}
	switch t.Kind {
	case ctype.Pointer:
		if t.Elem != nil && t.Elem.Kind == ctype.Builtin && t.Elem.Name == "char" && t.Elem.Const {
			return convCStr
		}
		return convNoStr // a char*, an int*, a pointer to a class: no conversion from a gocpp string
	case ctype.Builtin, ctype.Array, ctype.MemberPtr:
		return convNoStr
	}
	switch t.Kind {
	case ctype.LRef:
		if t.Elem == nil || !t.Elem.Const {
			return convNone // std::string& takes an lvalue string: not a temporary
		}
		t = t.Elem
	case ctype.RRef:
		t = t.Elem
	}
	if t != nil && t.Kind == ctype.Named {
		inStd := scope == "std" || strings.HasPrefix(scope, "std::")
		switch {
		case isStdString(t, inStd):
			return convStdString
		case isWideString(t, inStd):
			return convNoStr
		}
	}
	return convNone
}

// isStdString reports whether the named type t is std::string (std::basic_string<char>, with or
// without the std:: written).
func isStdString(t *ctype.Type, inStd bool) bool {
	p := t.Path
	if len(p) > 0 && p[0].Name == "std" {
		p = p[1:]
	} else if !inStd {
		return false // a bare string is std's only inside std
	}
	if len(p) != 1 {
		return false
	}
	switch p[0].Name {
	case "string":
		return !p[0].HasArgs
	case "basic_string":
		return len(p[0].Args) >= 1 && p[0].Args[0].Type != nil && p[0].Args[0].Type.Kind == ctype.Builtin && p[0].Args[0].Type.Name == "char"
	}
	return false
}

// A calleeSig is the function type of one overload, with the scope its names are written in.
type calleeSig struct {
	t     *ctype.Type
	scope string
}

// isWideString reports whether t is std::wstring, u16string, u32string or basic_string of a type
// other than char: a gocpp string converts to none of them.
func isWideString(t *ctype.Type, inStd bool) bool {
	p := t.Path
	if len(p) > 0 && p[0].Name == "std" {
		p = p[1:]
	} else if !inStd {
		return false
	}
	if len(p) != 1 {
		return false
	}
	switch p[0].Name {
	case "wstring", "u16string", "u32string", "u8string":
		return true
	case "basic_string":
		return len(p[0].Args) >= 1 && p[0].Args[0].Type != nil && !(p[0].Args[0].Type.Kind == ctype.Builtin && p[0].Args[0].Type.Name == "char")
	}
	return false
}

// calleeSigs returns the function types that the callee of c can be (all the overloads) and its
// name: for a free function, the overload set of its name; for a member of a C++ object, the
// methods of that name of its class and bases. false when a candidate has no signature (a
// template), or the callee is something else (a function pointer, a constructor).
func (g *gen) calleeSigs(c *ast.CallExpr) ([]calleeSig, string, bool) {
	rep := g.interop.typed.rep
	if r, ok := g.cRefOf(c.Fun); ok {
		n, ok := r.x.(*cppir.Name)
		if !ok || r.kind != interop.Value {
			return nil, "", false
		}
		name := strings.TrimPrefix(n.Name, "::")
		fs := rep.Decls.Funcs[name]
		if fs == nil || len(fs.Overloads) == 0 {
			return nil, "", false
		}
		var out []calleeSig
		for _, o := range fs.Overloads {
			if o.Kind != "function" || o.Sig.T == nil || o.Sig.T.Kind != ctype.Func {
				return nil, "", false
			}
			out = append(out, calleeSig{o.Sig.T, fs.Scope})
		}
		return out, name, true
	}
	// x.m(args): the methods m of the class of x.
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	out, ok := g.methodSigs(sel.X, sel.Sel.Name)
	return out, sel.Sel.Name, ok
}

// methodSigs returns the function types of the methods called name of the class that xe is an
// object (or a pointer to one) of, with the methods of its bases; false when there is none, or
// one is a template.
func (g *gen) methodSigs(xe ast.Expr, name string) ([]calleeSig, bool) {
	rep := g.interop.typed.rep
	v := g.tvalOf(xe)
	if v == nil {
		return nil, false
	}
	t := v.Type
	if v.Kind == ftype.Pointer && v.Elem != nil {
		t = v.Elem.Type
	} else if v.Kind != ftype.Record {
		return nil, false
	}
	ms := rep.Mapper.MembersOf(t)
	mem := ms.Lookup(name)
	if mem == nil || mem.Kind != "method" {
		return nil, false
	}
	scope := rep.Mapper.Scope(t)
	var out []calleeSig
	for _, m := range mem.Methods {
		if m.Template || m.Sig.C == nil || m.Sig.C.Kind != ctype.Func {
			return nil, false
		}
		out = append(out, calleeSig{m.Sig.C, scope})
	}
	return out, len(out) > 0
}

// charPtrOutParam returns the argument of the call c, other than the string argument number skip,
// that is the address of a char pointer (&end of a *C.char variable: char **), or nil: the
// function can store a pointer into the string there (strtol, sqlite3_prepare_v2's tail).
func (g *gen) charPtrOutParam(c *ast.CallExpr, skip int) ast.Expr {
	for j, a := range c.Args {
		if j == skip {
			continue
		}
		if v := g.tvalOf(a); v != nil && v.Kind == ftype.Pointer && v.Elem != nil && v.Elem.Kind == ftype.Pointer &&
			v.Elem.Elem != nil && charLike(v.Elem.Elem.C) {
			return a
		}
		if u, ok := ast.Unparen(a).(*ast.UnaryExpr); ok && u.Op == token.AND {
			if te := g.valueTypeExpr(u.X); te != nil {
				if st, ok := ast.Unparen(te).(*ast.StarExpr); ok && g.isCharType(st.X) {
					return a
				}
			}
		}
	}
	return nil
}

// charLike reports whether t is char, signed char or unsigned char (const or not).
func charLike(t *ctype.Type) bool {
	if t == nil || t.Kind != ctype.Builtin {
		return false
	}
	switch t.Name {
	case "char", "signed char", "unsigned char":
		return true
	}
	return false
}

// isCharType reports whether the type expression te is C.char, C.schar or C.uchar.
func (g *gen) isCharType(te ast.Expr) bool {
	sel, ok := ast.Unparen(te).(*ast.SelectorExpr)
	if !ok || !g.isC(sel.X) {
		return false
	}
	switch sel.Sel.Name {
	case "char", "schar", "uchar":
		return true
	}
	return false
}

// methodKeepsCharPtr reports whether the call c is a method of a class of the program (not the
// library's) that has a data member that is a char pointer: a setter such as set(const char*) is
// then likely to store the pointer, and nothing in its declaration says otherwise.
func (g *gen) methodKeepsCharPtr(c *ast.CallExpr) (string, bool) {
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	v := g.tvalOf(sel.X)
	if v == nil {
		return "", false
	}
	t := v.Type
	if v.Kind == ftype.Pointer && v.Elem != nil {
		t = v.Elem.Type
	} else if v.Kind != ftype.Record {
		return "", false
	}
	rep := g.interop.typed.rep
	if inStdScope(rep.Mapper.Scope(t)) {
		return "", false
	}
	rec := rep.Decls.Records[rep.Mapper.Canonical(t)]
	if rec == nil {
		return "", false
	}
	for _, f := range rec.Fields {
		if ft := f.Type.T; ft != nil && ft.Kind == ctype.Pointer && charLike(ft.Elem) {
			return "the class has a char pointer member, " + f.Name, true
		}
	}
	return "", false
}

// inStdScope reports whether the namespace scope is std or inside it.
func inStdScope(scope string) bool { return scope == "std" || strings.HasPrefix(scope, "std::") }
