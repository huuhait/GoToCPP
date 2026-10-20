package main

import (
	"fmt"
	"strings"
)

// Val is one value the generated programs can format. Op is the operand as written in a call
// (vN for a global, or the literal itself); a global is declared as `var vN Decl = Init`.
type Val struct {
	Op       string
	Decl     string // type of the global ("" for an inline literal)
	Init     string // initializer ("" for the zero value)
	Class    string // int, uint, float, complex, string, bool, bytes, rune, nil, comp, ptr, err, meth
	TOnly    bool   // only %T is deterministic (a pointer to a scalar: its address)
	PtrIn    bool   // a non-nil pointer inside: only %v %s %q %x %X %T (methods hide it) are deterministic, and no # flag
	Err      bool   // an error value (for %w)
	Comp     bool   // composite: %+v and %#v are likely
	Describe string
}

// catalog returns the values, in a fixed order.
func catalog() []Val {
	var vs []Val
	n := 0
	global := func(class, typ, init string) *Val {
		n++
		vs = append(vs, Val{Op: fmt.Sprintf("v%d", n), Decl: typ, Init: init, Class: class, Describe: typ + " = " + init})
		return &vs[len(vs)-1]
	}
	lit := func(class, l string) *Val {
		vs = append(vs, Val{Op: l, Class: class, Describe: l})
		return &vs[len(vs)-1]
	}

	// integers
	ints := []struct {
		t        string
		min, max string
	}{
		{"int8", "-128", "127"}, {"int16", "-32768", "32767"}, {"int32", "-2147483648", "2147483647"},
		{"int64", "-9223372036854775808", "9223372036854775807"}, {"int", "-9223372036854775808", "9223372036854775807"},
	}
	for _, it := range ints {
		for _, v := range []string{"0", "1", "-1", "42", "-100", it.min, it.max} {
			global("int", it.t, v)
		}
	}
	uints := []struct{ t, max string }{
		{"uint8", "255"}, {"uint16", "65535"}, {"uint32", "4294967295"},
		{"uint64", "18446744073709551615"}, {"uint", "18446744073709551615"}, {"uintptr", "18446744073709551615"},
	}
	for _, it := range uints {
		for _, v := range []string{"0", "1", "42", "200", it.max} {
			global("uint", it.t, v)
		}
	}
	for _, v := range []string{"0", "65", "'x'", "0x263a", "0x1F600", "-1", "0x110000", "0xD800", "10", "0", "127", "128", "0xFFFD"} {
		global("rune", "rune", v)
	}
	global("int", "MyInt", "7")
	global("int", "MyInt", "-300")
	global("uint", "MyU8", "250")
	global("int", "Color", "0")
	global("int", "Color", "1")
	global("int", "Color", "9")
	for _, l := range []string{"0", "5", "-5", "1000000", "'a'", "0x7fffffff", "'\\n'"} {
		lit("int", l)
	}

	// floats
	for _, v := range []string{"0", "1", "-1", "0.1", "1.5", "-2.5", "123456789.123", "1e20", "1e21", "1e-5", "1e-7", "100000", "1234567890123",
		"0.5", "3.0e8", "2.5e-10", "1<<53 + 1", "999999.9999", "0.000123456", "float64(1)/3"} {
		global("float", "float64", v)
	}
	for _, v := range []string{"math.Float64frombits(0x8000000000000000)", "math.NaN()", "math.Inf(1)", "math.Inf(-1)", "math.MaxFloat64",
		"math.SmallestNonzeroFloat64", "math.Float64frombits(1<<52)", "-math.MaxFloat64"} {
		global("float", "float64", v)
	}
	for _, v := range []string{"0", "0.1", "16777216", "3.4e38", "1e-45", "1.17549435e-38", "1.5", "-7.25", "float32(1)/3", "123456.789", "1e10", "math.Float32frombits(0x80000000)",
		"float32(math.NaN())", "float32(math.Inf(1))", "float32(math.Inf(-1))"} {
		global("float", "float32", v)
	}
	global("float", "MyF", "2.5")
	for _, l := range []string{"1.5", "0.0", "1e100", "-0.25", "3.14159265358979", "1e6", "1e-6"} {
		lit("float", l)
	}

	// complex
	for _, v := range []string{"0", "1+2i", "-1.5-2.5i", "complex(math.NaN(), math.Inf(1))", "complex(math.Inf(-1), 0)", "1e21+1e-7i", "complex(1, math.Float64frombits(0x8000000000000000))"} {
		global("complex", "complex128", v)
	}
	for _, v := range []string{"0", "1+2i", "complex(float32(0.1), -3)", "complex(float32(math.NaN()), 1)"} {
		global("complex", "complex64", v)
	}
	lit("complex", "2i")
	lit("complex", "(1+1i)")

	// bool, nil, string
	global("bool", "bool", "true")
	global("bool", "bool", "false")
	global("bool", "MyBool", "true")
	lit("bool", "true")
	lit("nil", "nil")
	strs := []string{`""`, `"hello"`, `"hello world"`, `"héllo, 世界"`, `"a\tb\nc"`, `"\x00\x01\x7f"`, `"\xff\xfe"`, `"ab\xffcd"`, `"日本語"`,
		`"emoji \U0001F600 end"`, "`back` + \"`\" + `quote`", `"say \"hi\""`, `"    \u200b"`, `"0123456789012345678901234567890123456789012345678901234567890123456789"`,
		`"%d %s %!"`, `"\a\b\f\r\v\\"`, `"é"`, `"\xc3"`, `"x\xe4\xb8y"`, `"ä"`, `"hello, 世界 with more text to cut"`}
	for _, s := range strs {
		global("string", "string", s)
	}
	global("string", "MyStr", `"named"`)
	lit("string", `"lit"`)
	lit("string", `"a b"`)
	lit("string", `""`)

	// bytes
	global("bytes", "[]byte", "nil")
	global("bytes", "[]byte", "[]byte{}")
	global("bytes", "[]byte", `[]byte("abc")`)
	global("bytes", "[]byte", `[]byte("hi\x00\xff")`)
	global("bytes", "[]byte", `[]byte("héllo, wörld 世界, a longer byte slice with text")`)
	global("bytes", "[]uint8", `[]uint8{1, 2, 3}`)
	global("bytes", "MyBytes", `MyBytes("xy")`)
	global("bytes", "[4]byte", `[4]byte{1, 2, 3, 250}`)
	global("bytes", "[0]byte", `[0]byte{}`)
	global("bytes", "[3]byte", `[3]byte{'a', 'b', 'c'}`)
	global("bytes", "[]int8", `[]int8{-1, 2}`)
	global("bytes", "[]rune", `[]rune("a世\xff")`)

	// slices, arrays
	comp := func(typ, init string) { global("comp", typ, init).Comp = true }
	comp("[]int", "nil")
	comp("[]int", "[]int{}")
	comp("[]int", "[]int{1, -2, 3}")
	comp("[]string", `[]string{"a", "b c", ""}`)
	comp("[]string", "nil")
	comp("[][]int", "[][]int{{1}, nil, {2, 3}}")
	comp("[3]int", "[3]int{1, 2, 3}")
	comp("[0]int", "[0]int{}")
	comp("[2][2]int", "[2][2]int{{1, 2}, {3, 4}}")
	comp("[]float64", "[]float64{1.5, 0, -2}")
	comp("[]bool", "[]bool{true, false}")
	comp("[]any", `[]any{1, "a", nil, 2.5, []int{1}}`)
	comp("[]*Pt", "[]*Pt{nil, nil}")
	comp("MyArr", "MyArr{4, 5, 6}")
	comp("MySlice", `MySlice{"x", "y"}`)
	comp("[]error", "[]error{nil, ErrV{1}}")
	comp("[]StrV", "[]StrV{{1}, {2}}")
	comp("[]Color", "[]Color{0, 1, 5}")
	comp("[2]string", `[2]string{"p q", "r"}`)
	comp("[]uint16", "[]uint16{1, 65535}")
	comp("[]complex128", "[]complex128{1 + 2i}")

	// maps
	comp("map[string]int", `map[string]int{"b": 2, "a": 1, "c": 3}`)
	comp("map[string]int", "nil")
	comp("map[string]int", "map[string]int{}")
	comp("map[int]string", `map[int]string{3: "c", -1: "a", 2: "b"}`)
	comp("map[float64]string", `map[float64]string{0.5: "a", 1.5: "b", -3: "c"}`)
	comp("map[float64]int", `map[float64]int{math.NaN(): 1, 2: 2, math.Inf(-1): 3}`)
	comp("map[bool]int", `map[bool]int{true: 1, false: 0}`)
	comp("map[Pt]string", `map[Pt]string{{2, 1}: "b", {1, 9}: "a", {1, 2}: "c"}`)
	comp("map[[2]int]bool", `map[[2]int]bool{{2, 1}: true, {1, 5}: false}`)
	comp("map[any]int", `map[any]int{3: 1, 1: 2, 2: 3}`)
	comp("map[any]int", `map[any]int{"b": 1, "a": 2}`)
	comp("map[string][]int", `map[string][]int{"x": {1, 2}, "y": nil}`)
	comp("map[rune]bool", `map[rune]bool{'b': true, 'a': false}`)
	comp("map[uint8]string", `map[uint8]string{200: "z", 3: "y"}`)
	comp("MyMap", `MyMap{"k": 1}`)
	comp("map[string]map[string]int", `map[string]map[string]int{"o": {"i": 1}}`)
	comp("map[StrV]int", `map[StrV]int{{2}: 1, {1}: 2}`)
	comp("map[string]any", `map[string]any{"a": 1, "b": "x", "c": nil, "d": []int{1}}`)
	comp("map[complex128]int", `map[complex128]int{1 + 2i: 1, 1 + 1i: 2}`)

	// structs
	comp("Pt", "Pt{1, 2}")
	comp("Pt", "Pt{-1, 1000000}")
	comp("Inner", `Inner{1, "x y"}`)
	comp("Outer", "Outer{}")
	comp("Outer", `Outer{I: Inner{2, "q"}, S: []int{1}, M: map[string]int{"a": 1}, E: ErrV{3}, Any: 4.5}`)
	comp("Outer", `Outer{Any: Pt{1, 2}, E: nil}`)
	comp("Emb", `Emb{Inner{1, "x"}, 2}`)
	comp("Unexp", `Unexp{1, "s", 2}`)
	comp("Tagged", `Tagged{1, "t"}`)
	comp("Empty", "Empty{}")
	comp("struct{}", "struct{}{}")
	comp("struct{ A int; B []string }", "struct{ A int; B []string }{1, nil}")
	comp("struct{ X, Y float64; Z bool }", "struct{ X, Y float64; Z bool }{1.5, -2, true}")
	comp("Pair[int, string]", `Pair[int, string]{1, "a"}`)
	comp("Pair[string, []int]", `Pair[string, []int]{"k", []int{1}}`)
	comp("Box[Pair[int, bool]]", `Box[Pair[int, bool]]{Pair[int, bool]{1, true}}`)
	comp("Box[[]int]", `Box[[]int]{[]int{1}}`)
	comp("Box[*Pt]", `Box[*Pt]{}`)
	comp("Box[map[string]int]", `Box[map[string]int]{map[string]int{"a": 1}}`)
	comp("[]Pair[int, int]", `[]Pair[int, int]{{1, 2}, {3, 4}}`)
	comp("HasStrField", "HasStrField{StrV{1}, StrV{2}, nil}")
	comp("struct{ F func(); C chan int; P *int; E error; I any }", "struct{ F func(); C chan int; P *int; E error; I any }{}")
	comp("struct{ A [2]uint8; B []byte; S string }", `struct{ A [2]uint8; B []byte; S string }{[2]uint8{1, 2}, []byte("b"), "s"}`)

	// pointers
	ptr := func(typ, init string, tonly bool) { g := global("ptr", typ, init); g.TOnly = tonly; g.Comp = true }
	ptr("*Pt", "nil", false)
	ptr("*int", "nil", false)
	ptr("*string", "nil", false)
	ptr("*[]int", "nil", false)
	ptr("*map[string]int", "nil", false)
	ptr("*Outer", "nil", false)
	ptr("*Pt", "&Pt{1, 2}", false)
	ptr("*Inner", `&Inner{3, "z"}`, false)
	ptr("*[3]int", "&[3]int{1, 2, 3}", false)
	ptr("*[]int", "&[]int{1, 2}", false)
	ptr("*map[string]int", `&map[string]int{"a": 1}`, false)
	ptr("*Emb", `&Emb{Inner{1, "x"}, 2}`, false)
	ptr("*int", "new(int)", true)
	ptr("*string", "new(string)", true)
	ptr("*float64", "new(float64)", true)
	ptr("*Empty", "&Empty{}", false)
	ptr("*Box[int]", "&Box[int]{3}", false)
	ptr("*MyInt", "new(MyInt)", true)

	// funcs, chans, interfaces
	global("nil", "func()", "")
	global("nil", "func(int) string", "")
	global("nil", "chan int", "")
	global("nil", "<-chan string", "")
	global("nil", "error", "")
	global("nil", "any", "")
	global("nil", "fmt.Stringer", "")
	global("nil", "map[string]int", "")
	global("nil", "[]string", "")
	global("nil", "*Pt", "")

	// interfaces holding values
	for _, v := range []string{"int8(5)", "uint(7)", "1.5", `"s"`, "true", "[]int{1}", "Pt{1, 2}", "&Pt{1, 2}", "(*Pt)(nil)", "([]int)(nil)", "(map[string]int)(nil)", "(func())(nil)", "Color(1)", "StrV{3}", "'x'", "complex64(1)", "[2]bool{true}", `MyStr("n")`, "error(nil)"} {
		global("iface", "any", "id("+v+")").Comp = true
	}
	global("iface", "any", "any(nil)")

	// methods
	meth := func(class, typ, init string) *Val { g := global(class, typ, init); g.Comp = true; return g }
	meth("meth", "StrV", "StrV{1}")
	meth("meth", "*StrV", "&StrV{2}")
	meth("meth", "StrP", "StrP{3}")
	meth("meth", "*StrP", "&StrP{4}")
	meth("meth", "*StrP", "nil")
	meth("meth", "any", "&StrP{5}")
	meth("meth", "any", "StrP{5}")
	meth("meth", "fmt.Stringer", "StrV{6}")
	meth("meth", "ErrV", "ErrV{1}")
	meth("meth", "*ErrV", "&ErrV{2}")
	meth("meth", "*ErrV", "nil")
	meth("meth", "error", "ErrV{3}")
	meth("meth", "GoS", "GoS{1}")
	meth("meth", "*GoS", "&GoS{1}")
	meth("meth", "[]GoS", "[]GoS{{1}, {2}}")
	meth("meth", "Fmtr", "Fmtr{1}")
	meth("meth", "*Fmtr", "&Fmtr{1}")
	meth("meth", "[]Fmtr", "[]Fmtr{{1}, {2}}")
	meth("meth", "map[string]Fmtr", `map[string]Fmtr{"a": {1}}`)
	meth("meth", "PanicS", "PanicS{}")
	meth("meth", "*NilS", "&NilS{}")
	meth("meth", "*NilS", "nil")
	meth("meth", "[]PanicS", "[]PanicS{{}}")
	meth("meth", "Color", "Color(0)")
	meth("meth", "[]Color", "[]Color{0, 1}")
	meth("meth", "map[Color]Color", "map[Color]Color{1: 0, 0: 1}")
	meth("meth", "struct{ C Color; c Color }", "struct{ C Color; c Color }{1, 1}")
	meth("meth", "[]error", "[]error{ErrV{1}, nil}")
	meth("meth", "map[string]error", `map[string]error{"a": ErrV{1}}`)
	meth("meth", "[]fmt.Stringer", "[]fmt.Stringer{StrV{1}, nil}")
	meth("meth", "struct{ S fmt.Stringer; E error }", "struct{ S fmt.Stringer; E error }{StrV{1}, ErrV{2}}")

	// more methods
	meth("meth", "EmbS", "EmbS{StrV{1}, 2}")
	meth("meth", "*EmbS", "&EmbS{StrV{1}, 2}")
	meth("meth", "EmbP", "EmbP{&StrP{7}}").PtrIn = true
	meth("meth", "EmbP", "EmbP{}")
	meth("meth", "EmbE", "EmbE{ErrV{1}, 2}")
	meth("meth", "Sn", `Sn("x")`)
	meth("meth", "[]Sn", `[]Sn{"a", "b"}`)
	meth("meth", "PanicE", "PanicE{}")
	meth("meth", "PanicI", "PanicI{}")
	meth("meth", "PanicG", "PanicG{}")
	meth("meth", "PanicF", "PanicF{}")
	meth("meth", "FmtStr", "FmtStr{}")
	meth("meth", "[]FmtStr", "[]FmtStr{{}, {}}")
	meth("meth", "Both", "Both{}")
	meth("meth", "GoAndStr", "GoAndStr{}")
	meth("meth", "*NilFmt", "nil")
	meth("meth", "[]StrP", "[]StrP{{1}, {2}}")
	meth("meth", "[]*StrP", "[]*StrP{{1}, nil}").PtrIn = true
	meth("meth", "map[string]StrP", `map[string]StrP{"a": {1}}`)
	meth("meth", "map[string]*StrP", `map[string]*StrP{"a": {1}, "b": nil}`).PtrIn = true
	meth("meth", "[2]StrV", "[2]StrV{{1}, {2}}")
	meth("meth", "struct{ P StrP; Q *StrP; R StrV }", "struct{ P StrP; Q *StrP; R StrV }{StrP{1}, &StrP{2}, StrV{3}}").PtrIn = true
	meth("meth", "[]*ErrV", "[]*ErrV{nil, {1}}").PtrIn = true
	meth("meth", "[]any", "[]any{StrV{1}, &StrP{2}, ErrV{3}, nil, Color(1)}").PtrIn = true
	meth("meth", "map[Color]string", `map[Color]string{1: "a", 0: "b"}`)
	meth("meth", "[]PanicS", "[]PanicS{{}, {}}")
	meth("meth", "map[PanicS]int", "map[PanicS]int{{}: 1}")
	meth("meth", "struct{ E error; S fmt.Stringer }", "struct{ E error; S fmt.Stringer }{}")

	// types and values for %T and the odd corners
	nilv := func(typ string) { global("nil", typ, "") }
	nilv("func(int, ...string) (bool, error)")
	nilv("chan<- int")
	nilv("[]interface{ M() }")
	nilv("*[3]struct {\n\tX int `json:\"x\"`\n}")
	nilv("map[Pt][]*Pair[int, string]")
	nilv("Pair[Pair[int, string], []Box[int]]")
	nilv("Box[func(int) error]")
	nilv("Pair[any, error]")
	nilv("Pair[MyInt, *Pt]")
	nilv("Pair[[2]int, map[string]bool]")
	nilv("Pair[struct{ A int }, [2]string]")
	nilv("Pair[chan int, func()]")
	nilv("Pair[uint8, rune]")
	nilv("Pair[Color, fmt.Stringer]")
	nilv("*Box[*Box[int]]")
	nilv("[]Pair[string, Pair[int, int]]")
	nilv("<-chan <-chan int")
	nilv("func() func() int")
	nilv("interface{ String() string; Error() string }")
	nilv("map[string]interface{}")
	nilv("[][2][]*struct{}")
	nilv("*Tree")
	comp("Tree", "Tree{V: 1}")
	comp("*Tree", "&Tree{V: 2}")
	comp("[]MyInt", "[]MyInt{1, -2}")
	comp("map[MyStr]MyInt", `map[MyStr]MyInt{"b": 1, "a": 2}`)
	comp("[3]int8", "[3]int8{-1, 0, 127}")
	comp("[]uint64", "[]uint64{0, 1<<63}")
	comp("[]MyU8", "[]MyU8{1, 2}")
	comp("[2][2]byte", "[2][2]byte{{1, 2}, {3, 4}}")
	comp("[]MyBool", "[]MyBool{true}")
	comp("[]MyF", "[]MyF{1.5, 2}")
	comp("[]uintptr", "[]uintptr{1, 2}")
	global("int", "MyPtr", "255")
	global("complex", "NamedC", "1+2i")
	comp("[]float32", "[]float32{1.5, 0.1, -3}")
	comp("[]MyBytes", `[]MyBytes{MyBytes("a"), nil}`)
	comp("struct{ B []byte; A [2]byte; S MyBytes }", `struct{ B []byte; A [2]byte; S MyBytes }{[]byte("x"), [2]byte{1, 2}, MyBytes("y")}`)
	comp("map[string][]byte", `map[string][]byte{"k": []byte("v")}`)
	comp("[]string", `[]string{"é", "\xff", ""}`)
	comp("[]rune", `[]rune{'a', 'é'}`)
	comp("[]int32", `[]int32{97, 233}`)
	comp("struct{ r rune; b byte }", `struct{ r rune; b byte }{'x', 'y'}`)
	comp("[]uint8", `[]uint8{}`)

	// errors for %w
	errv := func(typ, init string) { g := global("err", typ, init); g.Err = true }
	errv("error", "ErrV{1}")
	errv("error", "ErrV{2}")
	errv("error", `errors.New("plain")`)
	errv("error", `fmt.Errorf("inner %d", 1)`)
	errv("error", `fmt.Errorf("wrap: %w", ErrV{4})`)
	errv("error", `errors.Join(ErrV{5}, ErrV{6})`)
	errv("error", "nil")
	errv("error", `&StrErr{"se"}`)

	return vs
}

// valByClass groups the indexes of the values.
func classOf(vs []Val) map[string][]int {
	m := map[string][]int{}
	for i, v := range vs {
		m[v.Class] = append(m[v.Class], i)
	}
	return m
}

func declLine(v Val) string {
	if v.Init == "" {
		return fmt.Sprintf("var %s %s", v.Op, v.Decl)
	}
	return fmt.Sprintf("var %s %s = %s", v.Op, v.Decl, v.Init)
}

var _ = strings.Repeat
