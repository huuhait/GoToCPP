package lower

import (
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// ---------------------------------------------------------------- strings

// pointerToChar reports whether m is a pointer to the character type name (const or not).
func pointerToChar(m ftype.Mapped, names ...string) (name string, isConst, ok bool) {
	if m.Kind != ftype.Pointer || m.Elem == nil || m.Elem.Kind != ftype.Scalar {
		return "", false, false
	}
	if m.Elem.C == nil || m.Elem.C.Kind != ctype.Builtin {
		return "", false, false
	}
	for _, n := range names {
		if m.Elem.C.Name == n {
			return n, m.Elem.Const || m.Elem.C.Const, true
		}
	}
	return "", false, false
}

// stringNeverTakes reports whether a Go string can never convert to a parameter of the mapped
// type m: a number, a pointer of any kind, an array (a class might have a constructor, so those are
// the C++ compiler's).
func stringNeverTakes(m ftype.Mapped) bool {
	switch m.Kind {
	case ftype.Scalar, ftype.Pointer, ftype.FuncPtr, ftype.Array:
		return true
	}
	return false
}

func cstrParam(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict) {
	m := g.mapParam(p, scope)
	if _, isConst, ok := pointerToChar(m, "char"); ok {
		if isConst {
			return adaptKey{row: "cstr"}, vAdapt
		}
		return adaptKey{}, vIgnore // a char*: the callee may write; the user passes C.CString
	}
	if stringNeverTakes(m) {
		return adaptKey{}, vIgnore
	}
	return adaptKey{}, vOther
}

func stdStringParam(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict) {
	switch paramConv(p, scope) {
	case convStdString:
		return adaptKey{row: "stdstring"}, vAdapt
	case convNoStr:
		return adaptKey{}, vIgnore
	}
	return adaptKey{}, vOther
}

// wideParam: a const wchar_t*, char16_t* or char32_t* parameter takes the wide copy of a string. A
// string constant is a C literal already, so for it only the wide parameters are adapted, and a
// const char* is left alone (vOther).
func wideParam(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict) {
	m := g.mapParam(p, scope)
	if name, isConst, ok := pointerToChar(m, "wchar_t", "char16_t", "char32_t"); ok {
		if isConst {
			return adaptKey{row: "wide", name: name}, vAdapt
		}
		return adaptKey{}, vIgnore
	}
	if ac == acStringConst && stringNeverTakes(m) {
		if _, _, ok := pointerToChar(m, "char"); !ok {
			return adaptKey{}, vIgnore // a literal reaches a const char* by itself: C++'s
		}
	}
	return adaptKey{}, vOther
}

func lowerCStr(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	if !fg.stringKeepsOK(st) || fg.formatGuard(st) {
		return nil, false
	}
	return &cppir.Call{Fun: &cppir.Member{X: call("gp::cstr", st.x), Name: "gp_ptr"}}, true
}

func lowerStdString(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	return &cppir.Call{Fun: cxxName("::std::string"), Args: []cppir.Expr{st.x}}, true
}

func lowerWide(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	if !fg.stringKeepsOK(st) {
		return nil, false
	}
	fun := &cppir.Name{Name: "gp::wide_cstr", Args: []cppir.Node{typeName(st.key.name)}}
	holder := &cppir.Call{Fun: fun, Args: []cppir.Expr{st.x}}
	return &cppir.Call{Fun: &cppir.Member{X: holder, Name: "gp_ptr"}}, true
}

// stringKeepsOK reports whether the callee is not known to keep the pointer of the string argument
// or to hand back a pointer into it, for a narrow or a wide string alike (keeps.go): a function on
// the list, a method of a class that has a pointer member of the character type, a char pointer
// that the call stores into (&end), a function whose result points into the string. It reports the
// user error when the callee does.
func (fg *funcGen) stringKeepsOK(st *adaptSite) bool {
	g := fg.g
	c := st.call
	a := st.arg
	text := g.srcText(a)
	chars := narrowChars
	char := "char"
	retain := "use C.CString(" + text + ")"
	holder := "C.cstr"
	if st.key.row == "wide" {
		chars = wideChars(st.key.name)
		char = st.key.name
		retain, holder = retainWide(char, text)
	}
	argText := func(n int) string { return g.srcText(c.Args[n]) }
	if why, keeps := keepsPointer(st.name, len(c.Args), argText); keeps {
		g.userError(a.Pos(), "%s keeps the pointer that it is given (%s), and a Go string's bytes "+
			"live only until the end of the statement: %s and C.free it when the library "+
			"is done with it", st.name, why, retain)
		return false
	}
	if why, keeps := g.methodKeepsCharPtr(c, chars); keeps {
		g.userError(a.Pos(), "%s may keep the pointer that it is given (%s), and a Go string's "+
			"bytes live only until the end of the statement: %s and C.free it when the "+
			"object is done with it", st.name, why, retain)
		return false
	}
	if out := g.charPtrOutParam(c, st.i, chars); out != nil {
		g.userError(out.Pos(), "%s is given the address of a %s pointer (%s) next to the "+
			"string %s: the pointer that it stores can point into the string, whose C string "+
			"lives only until the end of the statement: %s and C.free it when you are "+
			"done with the pointer", st.name, char, g.srcText(out), text, retain)
		return false
	}
	if aliasFuncs[lastName(st.name)] && !g.aliasUseOK(c) {
		g.userError(c.Pos(), "%s returns a pointer into its argument %s, which as a C string "+
			"lives only until the end of the statement: use it in the statement, or keep the "+
			"string alive with a variable, h := %s(%s)", st.name, text, holder, text)
		return false
	}
	return true
}

// retainWide returns the advice for a wide string that a callee keeps, and the holder that keeps
// it for the statements after: C.WString and C.wstr are for wchar_t; there is no helper for
// char16_t and char32_t (the string would have to be copied into memory from C.malloc).
func retainWide(char, text string) (advice, holder string) {
	if char == "wchar_t" {
		return "use C.WString(" + text + ")", "C.wstr"
	}
	return "copy the code units of " + text + " into memory from C.malloc", "C.cstr"
}

// formatGuard reports (and returns true) the use of a runtime string as the format of a printf-like
// function: the parameter is a format string by the compiler's own rule, so a % in the text is a
// conversion, and nothing in the Go types says whether it should be (the format-string bug).
func (fg *funcGen) formatGuard(st *adaptSite) bool {
	g := fg.g
	reached := 0
	for _, s := range st.sigs {
		if st.i >= len(s.t.Params) {
			continue // a variadic overload: the argument is not its format
		}
		if s.fmtKind != "printf" || s.fmtParam != st.i+1 {
			return false // an overload with no format there: C++ would choose among them
		}
		reached++
	}
	if reached == 0 {
		return false
	}
	text := g.srcText(st.arg)
	g.userError(st.arg.Pos(), "%s is printf-like (its parameter %d is a format string), and %s "+
		"would be read as a format: a %% in its text is a conversion (the format-string bug); "+
		"pass it as an argument, %s, or write C.cstr(%s) when it is a format on purpose",
		st.name, st.i+1, text, g.formatHint(st), text)
	return true
}

// formatHint returns the call of st with the runtime string passed as an argument of "%s": the
// callee as written, the format argument replaced by "%s" and the string next to it.
func (g *gen) formatHint(st *adaptSite) string {
	args := make([]string, 0, len(st.call.Args)+1)
	for i, a := range st.call.Args {
		if i == st.i {
			args = append(args, `"%s"`, g.srcText(a))
			continue
		}
		args = append(args, g.srcText(a))
	}
	return g.srcText(st.call.Fun) + "(" + strings.Join(args, ", ") + ")"
}
