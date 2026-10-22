package adapt

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/types"
	"math/big"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The decisions of Decide with made-up declarations (no compiler): which argument a row adapts,
// which it refuses, and with which words at which place. The lowering, `gocpp check` and the editor
// all take these answers, so a rule is pinned here and not only by the programs that clang builds.

// testMapper is a mapper over declarations with a plain enum Color (0, 1, 3), a scoped enum
// Small whose underlying type is unsigned char, and a scoped enum Neg with a negative value.
func testMapper() *ftype.Mapper {
	small := &probe.Enum{Name: "Small", Scoped: true, Underlying: probe.TypeText{
		Text: "unsigned char", T: &ctype.Type{Kind: ctype.Builtin, Name: "unsigned char"}}}
	color := &probe.Enum{Name: "Color", Values: []probe.Enumerator{
		{Name: "Red", Value: big.NewInt(0)}, {Name: "Green", Value: big.NewInt(1)},
		{Name: "Blue", Value: big.NewInt(3)},
	}}
	signed := &probe.Enum{Name: "Signed", Values: []probe.Enumerator{
		{Name: "Low", Value: big.NewInt(-5)}, {Name: "High", Value: big.NewInt(3)},
	}}
	decls := &probe.DeclTable{
		Enums: map[string]*probe.Enum{"Color": color, "Small": small, "Signed": signed},
		Canon: map[string]string{"Color": "Color", "Small": "Small", "Signed": "Signed"},
	}
	return ftype.New(decls, ftype.LP64)
}

// callOf makes a call of name whose overloads are the function types, and the arguments with the
// texts as the program wrote them.
func callOf(t *testing.T, name string, texts []string, args []Arg, sigs ...string) *Call {
	t.Helper()
	c := &Call{Name: name, Args: args}
	c.Source = func(i int) string {
		if i == AtCall {
			return name
		}
		return texts[i]
	}
	for _, text := range sigs {
		ty, err := ctype.Parse(text)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		// No parameter is written as an array; a test that needs one says so.
		c.Sigs = append(c.Sigs, Sig{T: ty, Arrays: make([]bool, len(ty.Params))})
	}
	return c
}

func intConst(n int64) Arg {
	return Arg{Class: ClassNumConst, Const: types.TypeAndValue{
		Type: types.Typ[types.UntypedInt], Value: constant.MakeInt64(n)}}
}

func floatConst(x float64) Arg {
	return Arg{Class: ClassNumConst, Const: types.TypeAndValue{
		Type: types.Typ[types.UntypedFloat], Value: constant.MakeFloat64(x)}}
}

var str = Arg{Class: ClassString}

func TestDecideFormatGuard(t *testing.T) {
	m := testMapper()
	call := callOf(t, "printf", []string{"msg"}, []Arg{str}, "int (const char *, ...)")
	call.Sigs[0].FmtKind, call.Sigs[0].FmtParam = "printf", 1
	got := Decide(m, call, 0)
	if got.Outcome != Refuse || got.At != 0 || !strings.Contains(got.Msg, "printf is printf-like") ||
		!strings.Contains(got.Msg, `printf("%s", msg)`) {
		t.Errorf("a runtime string as the format = %+v", got)
	}

	// The string is an argument of the format, not the format.
	call = callOf(t, "printf", []string{`"%s"`, "msg"}, []Arg{{Class: ClassStringConst}, str},
		"int (const char *, ...)")
	call.Sigs[0].FmtKind, call.Sigs[0].FmtParam = "printf", 1
	if got := Decide(m, call, 1); got.Outcome != Convert || got.Key.Row != RowCStr {
		t.Errorf("a string after the format = %+v, want the cstr conversion", got)
	}

	// A function that is not printf-like takes the string.
	call = callOf(t, "puts", []string{"msg"}, []Arg{str}, "int (const char *)")
	if got := Decide(m, call, 0); got.Outcome != Convert || got.Key.Row != RowCStr {
		t.Errorf("puts = %+v, want the cstr conversion", got)
	}

	// An overload that has no format at that place leaves the choice to C++.
	call = callOf(t, "log", []string{"msg"}, []Arg{str}, "void (const char *)", "void (const char *)")
	call.Sigs[0].FmtKind, call.Sigs[0].FmtParam = "printf", 1
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("overloads that disagree on the format = %+v, want a conversion", got)
	}

	// A scanf-like format is not a runtime string to refuse: the guard is for printf.
	call = callOf(t, "scanf", []string{"fmt", "&n"}, []Arg{str}, "int (const char *, ...)")
	call.Sigs[0].FmtKind, call.Sigs[0].FmtParam = "scanf", 1
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("a runtime string as a scanf format = %+v, want a conversion", got)
	}

	// An overload with too few parameters stands for no format there: the others decide.
	call = callOf(t, "log", []string{"level", "msg"}, []Arg{str, str},
		"void (const char *, const char *)", "void (const char *)")
	call.Sigs[0].FmtKind, call.Sigs[0].FmtParam = "printf", 2
	if got := Decide(m, call, 1); got.Outcome != Refuse || got.At != 1 {
		t.Errorf("a format that a shorter overload lacks = %+v, want the refusal", got)
	}

	// The format of another parameter number does not guard this one.
	call = callOf(t, "logf", []string{"msg", "fmt"}, []Arg{str, str}, "void (const char *, const char *, ...)")
	call.Sigs[0].FmtKind, call.Sigs[0].FmtParam = "printf", 2
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("a string before the format = %+v, want a conversion", got)
	}
	if got := Decide(m, call, 1); got.Outcome != Refuse || got.At != 1 || !strings.Contains(got.Msg, "parameter 2") {
		t.Errorf("the format at parameter 2 = %+v", got)
	}
}

func TestDecideStringKeeps(t *testing.T) {
	m := testMapper()
	call := callOf(t, "InitWindow", []string{"800", "600", "title"},
		[]Arg{intConst(800), intConst(600), str}, "void (int, int, const char *)")
	got := Decide(m, call, 2)
	if got.Outcome != Refuse || got.At != 2 || !strings.Contains(got.Msg, "keeps the pointer") ||
		!strings.Contains(got.Msg, "use C.CString(title)") {
		t.Errorf("a function that keeps the title = %+v", got)
	}

	call = callOf(t, "lib.set_static_name", []string{"s"}, []Arg{str}, "void (const char *)")
	if got := Decide(m, call, 0); got.Outcome != Refuse || !strings.Contains(got.Msg, "its name says") {
		t.Errorf("a name with static in it = %+v", got)
	}

	call = callOf(t, "lib.Window::SetTitle", []string{"s"}, []Arg{str}, "void (const char *)")
	call.MethodKeepsCharPtr = func(cs CharSet) (string, bool) {
		if !cs.HasName("char") {
			t.Errorf("the character set of a narrow string lacks char")
		}
		return "the class has a char pointer member", true
	}
	if got := Decide(m, call, 0); got.Outcome != Refuse || !strings.Contains(got.Msg, "may keep the pointer") ||
		!strings.Contains(got.Msg, "the class has a char pointer member") {
		t.Errorf("a method of a class with a char pointer = %+v", got)
	}
	call.MethodKeepsCharPtr = func(CharSet) (string, bool) { return "", false }
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("a method of a class without one = %+v, want a conversion", got)
	}

	// The address of a char pointer next to the string: the function can store a pointer into it.
	call = callOf(t, "strtol", []string{"s", "&end", "10"}, []Arg{str, {CharPtrAddr: "char"}, intConst(10)},
		"long (const char *, char **, int)")
	got = Decide(m, call, 0)
	if got.Outcome != Refuse || got.At != 1 || !strings.Contains(got.Msg, "(&end) next to the string s") {
		t.Errorf("strtol(s, &end, 10) = %+v, want the refusal at &end", got)
	}
	call.Args[1].CharPtrAddr = "wchar_t"
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("the address of a wide pointer next to a narrow string = %+v, want a conversion", got)
	}
	// The string argument itself is not the pointer that can point into it.
	call.Args[1].CharPtrAddr = ""
	call.Args[0].CharPtrAddr = "char"
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("a string that is the only char pointer address = %+v, want a conversion", got)
	}

	// A function whose result points into the string.
	call = callOf(t, "strstr", []string{"s", `"x"`}, []Arg{str, {Class: ClassStringConst}},
		"const char *(const char *, const char *)")
	call.AliasUseOK = func() bool { return false }
	got = Decide(m, call, 0)
	if got.Outcome != Refuse || got.At != AtCall || !strings.Contains(got.Msg, "h := C.cstr(s)") {
		t.Errorf("strstr whose result outlives the statement = %+v", got)
	}
	call.AliasUseOK = func() bool { return true }
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("strstr used in the statement = %+v, want a conversion", got)
	}
	call.AliasUseOK = nil
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("strstr with no fact about the use = %+v, want a conversion", got)
	}
}

func TestDecideWideStringKeeps(t *testing.T) {
	m := testMapper()
	for _, c := range []struct{ char, advice, holder string }{
		{"wchar_t", "use C.WString(s)", "C.wstr"},
		{"char16_t", "copy the code units of s into memory from C.malloc", "C.cstr"},
	} {
		call := callOf(t, "openlog", []string{"s"}, []Arg{str}, "void (const "+c.char+" *)")
		got := Decide(m, call, 0)
		if got.Outcome != Refuse || !strings.Contains(got.Msg, c.advice) {
			t.Errorf("%s: a kept wide string = %+v, want the advice %q", c.char, got, c.advice)
		}
		call = callOf(t, "strchr", []string{"s"}, []Arg{str}, "const "+c.char+" *(const "+c.char+" *)")
		call.AliasUseOK = func() bool { return false }
		got = Decide(m, call, 0)
		if got.Outcome != Refuse || !strings.Contains(got.Msg, "h := "+c.holder+"(s)") {
			t.Errorf("%s: a wide alias = %+v, want the holder %s", c.char, got, c.holder)
		}
		call = callOf(t, "lib.f", []string{"s"}, []Arg{str}, "void (const "+c.char+" *)")
		if got := Decide(m, call, 0); got.Outcome != Convert || got.Key != (Key{Row: RowWide, Name: c.char}) {
			t.Errorf("%s: a wide string for a plain function = %+v", c.char, got)
		}
		// A narrow char pointer address next to a wide string is not the wide character's.
		call = callOf(t, "wcstol", []string{"s", "&end"}, []Arg{str, {CharPtrAddr: "char"}},
			"long (const "+c.char+" *, "+c.char+" **)")
		if got := Decide(m, call, 0); got.Outcome != Convert {
			t.Errorf("%s: a narrow pointer's address next to a wide string = %+v", c.char, got)
		}
		call.Args[1].CharPtrAddr = c.char
		if got := Decide(m, call, 0); got.Outcome != Refuse || got.At != 1 {
			t.Errorf("%s: the address of the wide pointer = %+v, want the refusal at it", c.char, got)
		}
	}
}

func TestDecideEnumRange(t *testing.T) {
	m := testMapper()
	for _, c := range []struct {
		enum   string
		n      int64
		range_ string
		want   Outcome
	}{
		{"Color", 0, "", Convert},
		{"Color", 3, "", Convert},
		{"Color", 2, "", Convert}, // inside the bit-field of the enumerators
		{"Color", 4, "0 to 3", Refuse},
		{"Color", -1, "0 to 3", Refuse},
		{"Small", 255, "", Convert},
		{"Small", 256, "0 to 255", Refuse},
		{"Small", -1, "0 to 255", Refuse},
		{"Signed", -8, "", Convert},
		{"Signed", 7, "", Convert},
		{"Signed", 8, "-8 to 7", Refuse},
		{"Signed", -9, "-8 to 7", Refuse},
	} {
		call := callOf(t, "paint", []string{"n"}, []Arg{intConst(c.n)}, "void ("+c.enum+")")
		got := Decide(m, call, 0)
		if got.Outcome != c.want {
			t.Errorf("%s %d = %+v, want outcome %v", c.enum, c.n, got, c.want)
			continue
		}
		if c.want == Refuse && (got.At != 0 || !strings.Contains(got.Msg, c.range_) ||
			!strings.Contains(got.Msg, "enum "+c.enum)) {
			t.Errorf("%s %d: message %q, want the range %q", c.enum, c.n, got.Msg, c.range_)
		}
		if c.want == Convert && got.Key != (Key{Row: RowEnum, Name: c.enum}) {
			t.Errorf("%s %d: key %+v", c.enum, c.n, got.Key)
		}
	}
}

func TestDecideNumberConstants(t *testing.T) {
	m := testMapper()
	for _, c := range []struct {
		param string
		arg   Arg
		text  string
		want  Outcome
		msg   string
	}{
		{"char", intConst(100), "100", Leave, ""},
		{"char", intConst(200), "200", Refuse, "cannot use 200 (untyped int constant) as C.char value"},
		{"unsigned int", intConst(-1), "-1", Refuse, "as C.uint value"},
		{"unsigned int", intConst(1 << 31), "1<<31", Leave, ""},
		{"int", floatConst(2.5), "2.5", Refuse, "as C.int value"},
		{"int", floatConst(2), "2.0", Leave, ""},
		{"float", floatConst(1e60), "1e60", Refuse, "as C.float value"},
		{"double", floatConst(1e60), "1e60", Leave, ""},
	} {
		call := callOf(t, "lib.f", []string{c.text}, []Arg{c.arg}, "void ("+c.param+")")
		got := Decide(m, call, 0)
		if got.Outcome != c.want || (c.want == Refuse && (got.At != 0 || !strings.Contains(got.Msg, c.msg))) {
			t.Errorf("%s for %s = %+v, want outcome %v with %q", c.text, c.param, got, c.want, c.msg)
		}
	}
	// A constant for a parameter that is not a number is C++'s.
	call := callOf(t, "lib.f", []string{"300"}, []Arg{intConst(300)}, "void (const char *)")
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("a constant for a pointer = %+v", got)
	}
}

func TestDecideVoidOutDeferred(t *testing.T) {
	m := testMapper()
	call := callOf(t, "make_object", []string{"&obj"}, []Arg{{Class: ClassAddrCPtr}}, "int (void **)")
	got := Decide(m, call, 0)
	if got.Outcome != Convert || got.Key.Row != RowVoidOut {
		t.Errorf("&obj for a void** = %+v, want the voidout conversion", got)
	}
	call.Deferred = true
	got = Decide(m, call, 0)
	if got.Outcome != Refuse || got.At != 0 || !strings.Contains(got.Msg, "deferred or spawned call") {
		t.Errorf("&obj in a deferred call = %+v", got)
	}
	// A const void** is not written by the callee: the row does not apply.
	call = callOf(t, "use", []string{"&obj"}, []Arg{{Class: ClassAddrCPtr}}, "int (void *const *)")
	call.Deferred = true
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("&obj for a void *const * = %+v, want it left to C++", got)
	}
}

func TestDecideScalarPointers(t *testing.T) {
	m := testMapper()
	addr := func(kind types.BasicKind) Arg {
		return Arg{Class: ClassAddrScalar, Var: "n", VarKind: kind}
	}
	for _, c := range []struct {
		name  string
		param string
		arg   Arg
		want  Outcome
		msg   string
	}{
		{"the same type", "int *", addr(types.Int32), Leave, ""},
		{"wider Go variable for a written int", "int *", addr(types.Int64), Convert, ""},
		{"narrower Go variable for a written int", "int *", addr(types.Int8), Refuse,
			"takes a pointer to int that it may write, and &n points to int8"},
		{"narrower Go variable for a const int", "const int *", addr(types.Int8), Convert, ""},
		{"integer for a float", "float *", addr(types.Int32), Refuse, "C converts neither way through a pointer"},
		{"float for an integer", "int *", addr(types.Float64), Refuse, "C converts neither way through a pointer"},
		{"wider float for a const float", "const float *", addr(types.Float64), Refuse,
			"is wider than float: the copy would round it"},
		{"unsigned for a signed that is written", "int *", addr(types.Uint32), Refuse, "that it may write"},
	} {
		call := callOf(t, "lib.f", []string{"&n"}, []Arg{c.arg}, "void ("+c.param+")")
		got := Decide(m, call, 0)
		if got.Outcome != c.want || (c.want == Refuse && (got.At != 0 || !strings.Contains(got.Msg, c.msg))) {
			t.Errorf("%s: %+v, want outcome %v with %q", c.name, got, c.want, c.msg)
		}
	}
	// An array parameter takes several elements: the address of one variable is not for it.
	call := callOf(t, "lib.f", []string{"&n"}, []Arg{addr(types.Int64)}, "void (int *)")
	call.Sigs[0].Arrays = []bool{true}
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("&n for an array parameter = %+v, want it left to C++", got)
	}
	call.Sigs[0].Arrays = []bool{false}
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("&n for a pointer parameter = %+v, want a conversion", got)
	}
	call.Sigs[0].Arrays = nil // not known: the same as an array
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("&n where the declaration is not known = %+v, want it left to C++", got)
	}
}

func TestDecideOverloadsMustAgree(t *testing.T) {
	m := testMapper()
	// Two overloads that adapt alike.
	call := callOf(t, "lib.f", []string{"s"}, []Arg{str}, "void (const char *)", "void (const char *, int)")
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("overloads that adapt alike = %+v", got)
	}
	// A char* is no candidate for a Go string, nor is a number: the others decide.
	call = callOf(t, "lib.f", []string{"s"}, []Arg{str}, "void (const char *)", "void (char *)", "void (int)")
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("overloads that cannot take a string = %+v", got)
	}
	// Two wide overloads of other characters are the same row with another detail.
	call = callOf(t, "lib.f", []string{"s"}, []Arg{str}, "void (const wchar_t *)", "void (const char16_t *)")
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("wchar_t and char16_t overloads = %+v, want it left to C++", got)
	}
	// One of them is an unknown class: nothing is adapted.
	call = callOf(t, "lib.f", []string{"s"}, []Arg{str}, "void (const char *)", "void (Widget)")
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("an overload with a class = %+v, want it left to C++", got)
	}
	// A narrow and a wide overload adapt differently.
	call = callOf(t, "lib.f", []string{"s"}, []Arg{str}, "void (const char *)", "void (const wchar_t *)")
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("a narrow and a wide overload = %+v, want it left to C++", got)
	}
	// An overload that takes no argument there is no candidate.
	call = callOf(t, "lib.f", []string{"s"}, []Arg{str}, "void (const char *)", "void ()")
	if got := Decide(m, call, 0); got.Outcome != Convert {
		t.Errorf("an overload with too few parameters = %+v", got)
	}
	// The refusal needs every overload to refuse the same way: the keeper overload counts.
	call = callOf(t, "getopt", []string{"s"}, []Arg{str}, "int (const char *)", "int (const char *)")
	if got := Decide(m, call, 0); got.Outcome != Refuse {
		t.Errorf("two overloads of a keeper = %+v", got)
	}
	// No overloads, no argument, a class that no row handles.
	call = callOf(t, "lib.f", []string{"s"}, []Arg{str})
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("no declaration = %+v", got)
	}
	call = callOf(t, "lib.f", []string{"s"}, []Arg{{}}, "void (const char *)")
	if got := Decide(m, call, 0); got.Outcome != Leave {
		t.Errorf("an argument of no class = %+v", got)
	}
	if got := Decide(m, call, 5); got.Outcome != Leave {
		t.Errorf("an argument that does not exist = %+v", got)
	}
}

func TestCPtrDeclared(t *testing.T) {
	isC := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		return ok && sel.X.(*ast.Ident).Name == "lib"
	}
	for _, c := range []struct {
		decl string
		want bool
	}{
		{"*lib.T", true},
		{"(*lib.T)", true},
		{"**lib.T", false},
		{"*int", false},
		{"lib.T", false},
		{"[]lib.T", false},
	} {
		te, err := parser.ParseExpr(c.decl)
		if err != nil {
			t.Fatalf("%s: %v", c.decl, err)
		}
		var isCType func(ast.Expr) bool
		isCType = func(e ast.Expr) bool {
			if star, ok := e.(*ast.StarExpr); ok {
				return isCType(star.X) // as check.IsCTypeExpr: a pointer to a C type is one
			}
			return isC(e)
		}
		if got := CPtrDeclared(te, isCType); got != c.want {
			t.Errorf("CPtrDeclared(%s) = %v, want %v", c.decl, got, c.want)
		}
	}
	if CPtrDeclared(nil, func(ast.Expr) bool { return true }) {
		t.Error("CPtrDeclared(nil) = true")
	}
}

func TestCharPtrAddr(t *testing.T) {
	isC := func(e ast.Expr) bool {
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "C"
	}
	for _, c := range []struct {
		arg, decl, want string
	}{
		{"&end", "*C.char", "char"},
		{"&end", "*C.schar", "signed char"},
		{"&end", "*C.uchar", "unsigned char"},
		{"&end", "*C.wchar_t", "wchar_t"},
		{"&end", "*C.int", ""},
		{"&end", "C.char", ""},
		{"&end", "*lib.char", ""},
		{"end", "*C.char", ""},
		{"&end", "", ""},
	} {
		arg, err := parser.ParseExpr(c.arg)
		if err != nil {
			t.Fatal(err)
		}
		declared := func(ast.Expr) ast.Expr {
			if c.decl == "" {
				return nil
			}
			te, err := parser.ParseExpr(c.decl)
			if err != nil {
				t.Fatal(err)
			}
			return te
		}
		if got := CharPtrAddr(nil, arg, declared, isC); got != c.want {
			t.Errorf("CharPtrAddr(%s declared %s) = %q, want %q", c.arg, c.decl, got, c.want)
		}
	}
	// What the probe says wins over the declaration: a char ** value.
	charPtrPtr := &ftype.Value{Mapped: ftype.Mapped{Kind: ftype.Pointer, Elem: &ftype.Mapped{
		Kind: ftype.Pointer, Elem: &ftype.Mapped{
			C: &ctype.Type{Kind: ctype.Builtin, Name: "wchar_t"}}}}}
	arg, _ := parser.ParseExpr("&end")
	if got := CharPtrAddr(charPtrPtr, arg, func(ast.Expr) ast.Expr { return nil }, func(ast.Expr) bool { return false }); got != "wchar_t" {
		t.Errorf("CharPtrAddr of a probed wchar_t ** = %q", got)
	}
}
