// The Go side of the differential test of the fmt shim (Phase 03 step 3.15a):
// runtime/test/fmt_diff.cpp is the same program written against the C++ runtime, as gocpp
// would generate it (the struct, named-type and method protocols of gocpp/fmtshim.hpp, and the
// type tags for int64, uint64 and uintptr). runtime/cxx_test.go runs both with each case name
// below and compares their exit codes and their stdout and stderr, merged into one stream (up
// to Go's stack trace).
package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"strconv"
)

type Point struct{ X, Y int }

type inner struct{ A, b int }

type Inner struct{ N int }

type Mixed struct {
	Name  string
	Tags  []string
	M     map[string]int
	P     *Point
	in    inner
	Inner     // embedded: %+v names it Inner
	_     int // blank: printed too
	F     float64
	Pt    Point
}

type Empty struct{}

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type Name string

type IDs []int

type Color int

func (c Color) String() string {
	switch c {
	case 0:
		return "red"
	case 1:
		return "green"
	}
	return "color(" + strconv.Itoa(int(c)) + ")"
}

// PtrS has String only through a pointer.
type PtrS struct{ V int }

func (p *PtrS) String() string {
	if p == nil {
		return "nil PtrS"
	}
	return "PtrS(" + strconv.Itoa(p.V) + ")"
}

// Deref has a pointer String that dereferences its receiver.
type Deref struct{ V int }

func (p *Deref) String() string { return "Deref(" + strconv.Itoa(p.V) + ")" }

// Both has Error and String: fmt uses Error.
type Both struct{}

func (Both) Error() string  { return "the error" }
func (Both) String() string { return "the string" }

// Wrap holds Stringers in exported and unexported fields.
type Wrap struct {
	C Celsius
	c Celsius
	L []Color
	l []Color
}

// Outer embeds Color, so its method set has Color's String.
type Outer struct {
	Color
	N int
}

type Boom struct{ N int }

func (b Boom) String() string {
	if b.N == 1 {
		panic("boom")
	}
	if b.N == 2 {
		s := []int{1, 2, 3}
		i := 5
		return strconv.Itoa(s[i])
	}
	if b.N == 3 {
		var p *Point
		return strconv.Itoa(p.X)
	}
	return "ok"
}

// Key is a struct map key.
type Key struct {
	A string
	B int
}

type Sizes struct {
	I64 int64
	U64 uint64
	UP  uintptr
	L   []int64
}

// Stamp's String changes the global that is printed after it.
type Stamp int

var later = []int{1, 2}

func (s Stamp) String() string {
	later = nil
	return "stamp"
}

// MapEdit's String deletes from the map that holds it.
type MapEdit int

var edited map[int]MapEdit

func (m MapEdit) String() string {
	for k := range edited {
		delete(edited, k)
	}
	return "edit" + strconv.Itoa(int(m))
}

// Clear's String empties held.L, Drop's drops.
type Clear int

type Holder struct{ L []Clear }

var held Holder

func (Clear) String() string {
	held.L = nil
	return "c"
}

type Drop int

var drops [][]Drop

func (Drop) String() string {
	drops = nil
	return "d"
}

// The read-only rules of reflect for embedded fields: emb is unexported and embedded in Pub,
// Pub is exported and embedded in the unexported hid, Top embeds hid and *pemb.
type emb struct {
	C Celsius
	c Celsius
	L []Celsius
	M map[string]Celsius
	P Pub2
	p Pub2
}

type Pub2 struct{ D Celsius }

type Pub struct {
	emb
	E Celsius
}

type hid struct {
	Pub
	F Celsius
	g Celsius
}

type pemb struct{ H Celsius }

type Top struct {
	hid
	*pemb
	Pubs []Pub
	pubs []Pub
	x    Pub
}

func main() {
	switch os.Args[1] {
	case "fmt-slices":
		slicesCase()
	case "fmt-maps":
		mapsCase()
	case "fmt-structs":
		structsCase()
	case "fmt-pointers":
		pointersCase()
	case "fmt-methods":
		methodsCase()
	case "fmt-panics":
		panicsCase()
	case "fmt-verbs":
		verbsCase()
	case "fmt-quote":
		quoteCase()
	case "fmt-floats":
		floatsCase()
	case "fmt-typed":
		typedCase()
	case "fmt-sprint":
		sprintCase()
	case "fmt-errors":
		errorsCase()
	case "fmt-big":
		bigCase()
	case "fmt-runes":
		runesCase()
	case "fmt-mutate":
		mutateCase()
	case "fmt-embedded":
		embeddedCase()
	case "fmt-complex":
		complexCase()
	default:
		fmt.Println("unknown case", os.Args[1])
		os.Exit(3)
	}
}

func slicesCase() {
	fmt.Println([]int{1, 2, 3}, []int{}, []int(nil), [3]int{4, 5, 6}, [0]int{})
	fmt.Println([]string{"a", "b c", ""}, []bool{true, false}, [][]int{{1, 2}, {}, nil, {3}})
	fmt.Println([]int8{-1, 2}, []uint16{65535}, []int32{'a', 'b'}, []uint8{1, 2, 255})
	fmt.Println([][]string{{"x"}, {"y", "z"}}, [2][2]int{{1, 2}, {3, 4}})
	b := []byte("hi\x00é")
	fmt.Printf("%v %d %s %q|%8s|%-8s|%.2s|\n", b, b, b, b, b, b, b)
	var nb []byte
	fmt.Printf("%v %s %q|%3s|\n", nb, nb, nb, nb)
	arr := [3]byte{'a', 'b', 'c'}
	fmt.Printf("%v %s %q %d\n", arr, arr, arr, arr)
	type B byte
	bs := []B{'x', 'y'}
	fmt.Printf("%v %s %q\n", bs, bs, bs)
	fmt.Printf("%5d|%-4d|%03d|%+d\n", []int{1, -2}, []int{3}, []int{4, 5}, []int{6})
	fmt.Printf("%t %s %d\n", []int{1}, []int{2, 3}, []string{"a"})
	fmt.Printf("%q %v %s\n", []string{"a\n", "b"}, []string{"x"}, []string{"y"})
	fmt.Printf("%q %v %d\n", []int32{'x', '☺'}, []int32{'x'}, []rune{'y'})
	fmt.Printf("%6.2f|%v|%.1v\n", []float64{1.5, -2.25}, []float64{0.1, 1e21}, []float64{1.25})
	deep := [][][][][][][][][][]int{{{{{{{{{{1, 2}}}}}}}}}, {}}
	fmt.Println(deep, len(fmt.Sprint(deep)))
	fmt.Printf("%v|%d|%s\n", [][]int{{1}, {2, 3}}, [][]int{{4}}, [][]string{{"s"}})
	fmt.Println(IDs{1, 2}, IDs(nil), []Name{"n1", "n2"})
	fmt.Println([]Empty{{}, {}}, []*int{nil, nil}, [2]bool{})
	fmt.Println([]map[string]int{{"a": 1}, nil}, [][2]string{{"p", "q"}})
}

func mapsCase() {
	fmt.Println(map[string]int{"b": 2, "a": 1, "c": 3}, map[string]int{}, map[string]int(nil))
	fmt.Println(map[int]string{3: "c", -1: "z", 0: "o", 10: "t", -20: "m"})
	fmt.Println(map[float64]string{2.5: "b", math.Inf(-1): "ninf", math.Inf(1): "inf", 0: "zero", -1.5: "neg", math.NaN(): "nan"})
	nans := map[float64]int{1: 1}
	nans[math.NaN()] = 7
	nans[math.NaN()] = 7
	nans[math.NaN()] = 7
	fmt.Println(nans, len(nans))
	negz := map[float64]int{math.Copysign(0, -1): 1}
	fmt.Println(negz)
	fmt.Println(map[bool]int{true: 1, false: 0}, map[uint8]bool{200: true, 3: false, 70: true})
	fmt.Println(map[[2]int]string{{2, 1}: "c", {1, 2}: "b", {1, 1}: "a"})
	fmt.Println(map[Key]int{{"b", 1}: 3, {"a", 2}: 2, {"a", 1}: 1})
	fmt.Println(map[string][]int{"x": {1, 2}, "w": nil}, map[string]map[string]int{"o": {"z": 26, "a": 1}})
	fmt.Println(map[int32]float32{'b': 0.1, 'a': 2.5}, map[int8]int{-128: 1, 127: 2, 0: 3})
	fmt.Println(map[uint64]int{math.MaxUint64: 1, 0: 2}, map[int64]int{math.MinInt64: 1, math.MaxInt64: 2})
	fmt.Printf("%d %s %v %+v\n", map[string]int{"k": 1}, map[string]int{"k": 2}, map[Name]IDs{"n": {1}}, map[string]Point{"p": {1, 2}})
	fmt.Printf("%5v|%-3d|%q\n", map[string]int{"a": 1}, map[int]int{1: 2}, map[string]string{"k": "v"})
	fmt.Println(map[*int]int{nil: 5}, map[Celsius]Color{1.5: 1, -3: 0, 100: 7})
	fmt.Println(map[string]Point{"b": {3, 4}, "a": {1, 2}}, map[Point]bool{{2, 1}: true, {1, 9}: false})
	fmt.Println(map[[0]int]int{{}: 1}, map[Empty]int{{}: 2}, map[string]Empty{"e": {}})
}

func structsCase() {
	p := Point{1, 2}
	fmt.Println(p, Empty{}, inner{3, 4}, struct{}{})
	fmt.Printf("%v %+v %d %s\n", p, p, p, p)
	m := Mixed{Name: "m", Tags: []string{"t1", "t2"}, M: map[string]int{"y": 2, "x": 1}, in: inner{5, 6}, Inner: Inner{7}, F: 0.5, Pt: Point{8, 9}}
	fmt.Println(m)
	fmt.Printf("%+v\n", m)
	fmt.Printf("%v|%+v\n", Mixed{}, Mixed{})
	fmt.Printf("%q\n", m)
	fmt.Printf("%5v|%-5v|%05d\n", p, p, p)
	fmt.Printf("%+v %+v\n", struct{ A []Point }{[]Point{{1, 2}}}, []Point{{3, 4}})
	fmt.Printf("%+d %+v\n", p, map[string]Point{"k": {5, 6}})
	fmt.Println(struct {
		S string
		N *Point
	}{"s", nil})
	fmt.Printf("%v %+v\n", Sizes{-1, 2, 3, []int64{4}}, Sizes{})
	fmt.Println([]Point{{1, 2}, {3, 4}}, [1]Point{{5, 6}})
}

func pointersCase() {
	p := &Point{1, 2}
	fmt.Println(p, &[]int{1, 2}, &[2]string{"a", "b"}, &map[string]int{"a": 1})
	fmt.Printf("%v %+v %d %s\n", p, p, p, p)
	var np *Point
	var ni *int
	fmt.Println(np, ni, []*Point{nil}, struct{ P *int }{nil})
	fmt.Printf("%v|%d|%s|%t|%5v|%-6v|\n", np, ni, ni, np, ni, np)
	fmt.Printf("%s %v\n", []*Point{nil}, map[string]*int{"k": nil})
	x := 5
	s := fmt.Sprint(&x)
	fmt.Println(s[:2], len(s) > 2, s == fmt.Sprintf("%v", &x))
	pp := &p
	s = fmt.Sprint(pp)
	fmt.Println(s[:2], len(s) > 2)
	s = fmt.Sprint([]*int{&x})
	fmt.Println(s[:3], s[len(s)-1:])
	s = fmt.Sprint(&Mixed{P: p})
	fmt.Println(s[:10])
	m := Mixed{Name: "m", P: nil}
	fmt.Printf("%v\n%+v\n", &m, &m)
	type Named *Point
	var nn Named
	fmt.Println(nn)
	ids := &IDs{3, 4}
	fmt.Println(ids, &[]Point{{1, 2}})
	fmt.Printf("%s|%d\n", &Point{7, 8}, &[]int{9})
	fmt.Printf("%d\n", ni)
}

func methodsCase() {
	c := Celsius(21.5)
	fmt.Println(c, []Celsius{1, 2.25}, map[string]Celsius{"t": -4})
	fmt.Printf("%v|%s|%d|%q|%8s|%-8v|%.3s|%f|%t\n", c, c, c, c, c, c, c, c, c)
	fmt.Println(Color(0), Color(1), Color(5), []Color{1, 0}, map[Color]int{1: 10, 0: 20})
	fmt.Printf("%d %v %s\n", Color(1), Color(1), []Color{0})
	ps := PtrS{3}
	fmt.Println(ps, &ps, []*PtrS{&ps, nil}, []PtrS{{4}})
	var nps *PtrS
	fmt.Println(nps)
	fmt.Printf("%v %s %d\n", nps, &ps, &ps)
	var nd *Deref
	fmt.Println(nd, &Deref{2}, []*Deref{nil})
	var nc *Celsius
	fmt.Println(nc, []*Celsius{nil})
	fmt.Println(Both{}, []Both{{}}, &Both{})
	fmt.Printf("%v %s %q %d\n", Both{}, Both{}, Both{}, Both{})
	w := Wrap{C: 1, c: 2, L: []Color{0, 1}, l: []Color{1, 0}}
	fmt.Println(w)
	fmt.Printf("%+v\n", w)
	fmt.Println(Outer{Color: 1, N: 5}, []Outer{{}})
	fmt.Printf("%v %d\n", Outer{}, Outer{Color: 1, N: 2})
	fmt.Print(c, Color(1), "\n")
	fmt.Print(Name("a"), Name("b"), 1, 2, Name("c"), "\n")
	fmt.Println(fmt.Sprint(c), fmt.Sprintln(c, Color(0)) == "21.5°C red\n")
	fmt.Println(Stamp(1), later)
	edited = map[int]MapEdit{1: 1, 2: 2, 3: 3}
	fmt.Println(edited, len(edited))
	var nm map[string]Celsius
	fmt.Println(nm, map[Celsius]string{2: "two", 1: "one"})
}

func panicsCase() {
	fmt.Println(Boom{0}, Boom{1})
	fmt.Println([]Boom{{0}, {1}, {2}})
	fmt.Printf("%s|%v|%d|%q|%5v\n", Boom{1}, Boom{2}, Boom{1}, Boom{1}, Boom{1})
	fmt.Println(Boom{3})
	fmt.Println(map[string]Boom{"a": {1}, "b": {0}})
	fmt.Println(&Deref{3}, struct{ D *Deref }{nil})
	var nd *Deref
	fmt.Printf("%v|%s|%8v|\n", nd, nd, nd)
	panic("end")
}

func verbsCase() {
	n := 42
	fmt.Printf("%d|%5d|%-5d|%05d|%+d|% d|%+05d|%-05d|% 05d\n", n, n, n, n, n, n, n, n, n)
	fmt.Printf("%d|%5d|%-5d|%05d|%+d|% d|%+05d|%-05d\n", -n, -n, -n, -n, -n, -n, -n, -n)
	fmt.Printf("%.3d|%8.3d|%-8.3d|%08.3d|%.0d|%5.0d|%.0d|%+.0d|\n", 7, 7, 7, 7, 0, 0, 1, 0)
	fmt.Printf("%v|%+v|%6v|%-6v|%06v|%+6v\n", n, n, n, n, n, -n)
	fmt.Printf("%d %d %d %d\n", int8(-128), uint64(math.MaxUint64), int64(math.MinInt64), uint8(255))
	fmt.Printf("%020d|%-20d|%+.30d\n", int64(math.MinInt64), uint64(math.MaxUint64), 1)
	fmt.Printf("%s|%10s|%-10s|%.2s|%5.1s|%-5.3s|%010s|%.0s|%.10s\n", "go", "go", "go", "gopher", "gopher", "gopher", "go", "go", "go")
	fmt.Printf("%8s|%-8s|%.3s|%3s|%.1s|\n", "héllo", "日本語", "日本語x", "☺", "\xffab")
	fmt.Printf("%v|%5v|%-5v|%.1v|%q\n", "s", "s", "s", "st", "s")
	fmt.Printf("%t|%v|%6t|%-6t|%06t|%.2t|%q\n", true, false, true, false, true, true, true)
	fmt.Printf("%f|%.0f|%.1f|%.2f|%8.2f|%-8.2f|%08.2f|%+.2f|% .2f|%+08.2f\n", math.Pi, 2.5, 0.05, 2.675, math.Pi, math.Pi, -1.5, 1.5, 1.5, 1.5)
	fmt.Printf("%f|%.3f|%.10f|%f|%.0f|%.0f|%.0f\n", 1e20, 1e-10, 1.0/3, -0.0, 0.5, 1.5, -2.5)
	fmt.Printf("%05f|%+f|% f|%-6f|%6f|%+06f|% 6f\n", math.NaN(), math.Inf(1), math.NaN(), math.Inf(-1), math.Inf(1), math.Inf(-1), math.Inf(1))
	fmt.Printf("%v|%8v|%-8v|%08v|%+v|% v|%+08v\n", 3.5, 3.5, 3.5, -3.5, 3.5, 3.5, -3.5)
	fmt.Printf("%.3v|%.3v|%.2v|%.3v|%10.4v|%.0v|%.1v|%.20v\n", math.Pi, 1234567.0, 100.0, 0.0, math.E, 2.5, 0.05, 0.1)
	fmt.Printf("%.3v|%.5v|%.2v|%.4v|%v|%.3v\n", 1e-5, 123456.0, 0.000123, 1e21, float32(0.1), float32(2.5))
	fmt.Printf("%.1f|%.2f|%.3f|%5.1f\n", float32(0.15), float32(2.675), float32(1)/3, float32(-0.05))
	fmt.Printf("%v %d %s\n", 'x', 'x', 'x')
	fmt.Printf("%%|%5%|%-5%|\n")
	fmt.Printf("%T %T %T %T %T %T\n", 1, "s", 1.5, float32(1), true, int8(1))
	fmt.Printf("%T %T %T %T %T\n", []int{}, map[string][]bool{}, &Point{}, [3]uint16{}, Celsius(1))
	fmt.Printf("%T|%10T|%-10T|\n", Point{}, 1, "s")
	fmt.Printf("%[2]d %[1]d|%[1]d %d|%[3]s|%[0]d|%[x]d|%[1]\n", 1, 2)
	fmt.Printf("%*d|%-*d|%*d|%.*f|%*.*f|\n", 5, 1, 4, 2, -4, 3, 2, math.Pi, 8, 3, math.E)
	fmt.Printf("%*d|%.*d|%*d\n", "x", 1, "y", 2, 10000000, 3)
	fmt.Printf("%[2]*[1]d|%[3]*.[2]*[1]f|\n", 12, 2, 6)
	fmt.Printf("%-+5d|%+-5d|%0-5d|%-05d|% +d|%+ d\n", 1, 2, 3, 4, 5, 6)
	fmt.Printf("%#v %#d %#q %#s\n", 5, 5, "a", "b")
	fmt.Printf("%#q %#q %#q\n", "a`b", "tab\there", "nl\n")
}

func quoteCase() {
	for _, s := range []string{"", "plain", "a\"b", `a\b`, "tab\t", "nl\n", "\x00\x01\x7f", "\a\b\f\r\v",
		"é", "日本", "\u00a0", "\u00ad", "\u2028", "\u200b", "\ufeff", "☺", "\U0001F600", "\U0010FFFF",
		"\xff", "a\xc0\x80b", "\xe2\x82", "\xed\xa0\x80", "\xf4\x90\x80\x80", "'"} {
		fmt.Printf("%q|%+q|%#q|%-12q|%12q|%.2q\n", s, s, s, s, s, s)
	}
	for _, r := range []int32{'x', '\n', '\'', '"', '\\', 0, 0x7f, 0xa0, 0xad, 0x2028, '☺', 0x1F600, 0x10FFFF, -1, 0xD800, 0x110000, math.MaxInt32} {
		fmt.Printf("%q|%+q|%5q|%-5q|%v\n", r, r, r, r, r)
	}
	fmt.Printf("%q %q %q %q\n", uint8('a'), int64(65), uint64(math.MaxUint64), int8(-1))
	fmt.Printf("%q %+q\n", []string{"é", "\x00"}, []string{"é"})
}

func floatsCase() {
	a, b := 0.1, 0.2
	f := []float64{0.1, 0.2, a + b, 1e21, 1e20, 123456789.0, 1e-5, 1e-4, 5e-324, math.MaxFloat64,
		math.NaN(), math.Inf(-1), math.Inf(1), math.Copysign(0, -1), 100, 1e6, 999999.9, 12345.678, 2.5e-7}
	fmt.Println(f)
	fmt.Printf("%v\n%.2f\n%8.3v\n", f, f[:5], f[:5])
	g := []float32{0.1, 16777216, 3.4028235e38, 1e-45, 1.0 / 3, -0.25, float32(math.NaN())}
	fmt.Println(g)
	fmt.Printf("%v|%.3f|%6.2v\n", g, g[:3], g[:2])
	fmt.Println(map[float64]float32{0.5: 0.1, -0.5: 1e10}, struct{ A, B float64 }{1e-7, 7e7})
	fmt.Printf("%+v|%5.1f\n", struct {
		F32 float32
		F64 float64
	}{0.1, 0.1}, struct{ A, B float64 }{1.25, -9.75})
	fmt.Println(float32(1e21), float32(1e20), float32(123456789), float32(0.000012345))
	for _, x := range []float64{1, 12, 123, 1234, 12345, 123456, 1234567, 0.1, 0.01, 0.001, 0.0001, 0.00001} {
		fmt.Printf("%v %.2v %.3v %.4v %.6v %.10v|", x, x, x, x, x, x)
	}
	fmt.Println()
}

func typedCase() {
	var i64 int64 = 5
	var u64 uint64 = 6
	var up uintptr = 7
	fmt.Printf("%s|%s|%s|%t\n", i64, u64, up, int(8))
	fmt.Printf("%s %t\n", []int64{1, 2}, map[string]uint64{"k": 3})
	fmt.Printf("%s %s\n", [2]uintptr{4, 5}, map[uint64][]int64{9: {10}})
	var pi *int64
	fmt.Printf("%s %T %T %T %T\n", pi, pi, []uintptr{}, map[int64]uint64{}, [1]int64{})
	fmt.Printf("%s\n", Sizes{1, 2, 3, []int64{4}})
	fmt.Printf("%d\n", 1, i64, u64, up, []int64{1}, int(2), uint(3))
	fmt.Printf("%T %T %T %T %T\n", i64, u64, up, int(1), uint(1))
	fmt.Println(i64, u64, up, []int64{-1}, map[uintptr]int64{1: 2})
}

func sprintCase() {
	s := fmt.Sprint("a", 1, 2, "b", 3, true, "c", 4.5, Name("n"), 5, Name("m"))
	fmt.Println(s, len(s))
	s = fmt.Sprintln("a", 1, []int{2}, Point{3, 4})
	fmt.Print(s)
	fmt.Println(len(s))
	s = fmt.Sprintf("%d-%s-%v", 1, "two", []float64{3})
	fmt.Println(s, len(s))
	fmt.Println(fmt.Sprint(), fmt.Sprintln() == "\n", fmt.Sprintf("") == "", fmt.Sprintf("x") == "x")
	fmt.Print([]string{"a"}, []string{"b"}, "\n")
	fmt.Print("x", []int{1}, 2, "\n")
	fmt.Print(1, 2, map[string]int{}, Point{}, "\n")
	fmt.Println(fmt.Sprint(Name("x"), Name("y")), fmt.Sprint(IDs{1}, IDs{2}))
	fmt.Print()
	fmt.Println()
}

func errorsCase() {
	fmt.Printf("%d %s %t %f\n", "x", 5, 1, 7)
	fmt.Printf("%v %d\n", 1)
	fmt.Printf("%d\n", 1, 2, "x", 2.5, true, []int{3}, Point{1, 2}, Celsius(3), Color(1), &Point{4, 5})
	fmt.Printf("100%% %z %!\n", 3)
	fmt.Printf("%\303\251|%\xff|%z\n", 1, 2, Point{1, 2})
	fmt.Printf("%t %v\n", "s", math.NaN())
	fmt.Printf("%s %d %t\n", true, 1.5, float32(0.5))
	fmt.Printf("%s %s %s %s\n", int8(1), uint16(2), int32(3), uint32(4))
	fmt.Printf("%5s|%-6d|%+s|% s|%05t\n", 3, "x", 4, 5, "y")
	fmt.Printf("%s|%d|%t\n", Celsius(1.5), "str", Color(1))
	fmt.Printf("%t %s\n", map[string]bool{"k": true}, struct{ A int }{1})
	fmt.Printf("%w %w\n", 1, Point{1, 2})
	fmt.Printf("no verbs\n")
	fmt.Printf("%d %d %d\n", 1, 2)
	fmt.Printf("%.d|%5.d|%.f|%-.v\n", 0, 1, 2.5, 3)
	fmt.Printf("end%")
	fmt.Println()
	fmt.Printf("%.12345678d|%12345678d|%123456789d|x\n", 1, 2, 3)
}

func bigCase() {
	s := make([]int, 100000)
	for i := range s {
		s[i] = i * 7919 % 100003
	}
	fmt.Println(s)
	m := make(map[int]int)
	for i := 0; i < 10000; i++ {
		m[i*31%10007] = i
	}
	fmt.Println(m)
	f := make([]float64, 20000)
	for i := range f {
		f[i] = float64(i) / 7
	}
	fmt.Println(f)
	strs := make([]string, 10000)
	for i := range strs {
		strs[i] = strconv.Itoa(i * i)
	}
	fmt.Printf("%q\n", strs)
}

// runesCase hashes %q and %+q of every rune, as a string and as a rune, and a few invalid UTF-8
// strings: the whole of strconv.IsPrint and the quoting.
func runesCase() {
	h := fnv.New64a()
	n := 0
	for r := rune(-2); r <= 0x110001; r++ {
		fmt.Fprintf(h, "%q%+q%q%+q", string(r), string(r), r, r)
		n++
	}
	fmt.Println(n, h.Sum64())
	h.Reset()
	for b := 0; b < 256; b++ {
		for c := 0; c < 256; c += 15 {
			s := string([]byte{byte(b), byte(c), 'x'})
			fmt.Fprintf(h, "%q%+q", s, s)
		}
	}
	fmt.Println(h.Sum64())
}

// mutateCase prints values whose String methods change what is being printed: an argument is a
// copy, but through a pointer fmt reads the slices again for each element.
func mutateCase() {
	held = Holder{[]Clear{1, 2, 3}}
	fmt.Println(held, held.L == nil)
	held = Holder{[]Clear{1, 2, 3}}
	fmt.Println(&held, held.L == nil)
	drops = [][]Drop{{1, 2}, {3}}
	fmt.Println(&drops, drops == nil)
	drops = [][]Drop{{1, 2}, {3}}
	fmt.Println(drops, drops == nil)
}

func embeddedCase() {
	e := emb{1, 2, []Celsius{3}, map[string]Celsius{"k": 4}, Pub2{5}, Pub2{6}}
	pub := Pub{e, 7}
	fmt.Println(pub)
	fmt.Printf("%+v\n%v\n%s\n%d\n", pub, &pub, []Pub{pub}, pub)
	fmt.Println(map[string]Pub{"m": pub}, [1]Pub{pub})
	h := hid{pub, 8, 9}
	fmt.Println(h)
	fmt.Printf("%+v\n", h)
	t := Top{h, nil, []Pub{pub}, []Pub{pub}, pub}
	fmt.Println(t)
	fmt.Printf("%+v\n", &t)
	fmt.Println([]Top{t}, map[int]Top{1: t})
	fmt.Println(e, struct{ emb }{e}, struct{ x emb }{e})
}

// complexCase: complex numbers as fmt's fmtComplex prints them (step 3.15c): each part as a
// float of half the size, the imaginary part always signed, a width and a precision for each
// part, the error forms with the whole number, complex map keys sorted (real part, then
// imaginary part, NaN first), and random values of both sizes.
func complexCase() {
	nan, inf := math.NaN(), math.Inf(1)
	c := []complex128{0, complex(math.Copysign(0, -1), math.Copysign(0, -1)), 1.5 - 2i, complex(inf, nan),
		complex(nan, -inf), 1e21 + 1e-7i, complex(5e-324, math.MaxFloat64), 123456 - 0.0001i}
	fmt.Println(c)
	fmt.Printf("%v\n%.2f\n%9.3f|%-9.1f|%+v|% .1f|%07.2f\n", c, c[:3], c[2], c[2], c[2], c[2], c[2])
	c64 := []complex64{0.1 + 0.2i, complex(float32(1.0/3), 16777216), complex(3.4028235e38, 1e-45)}
	fmt.Println(c64)
	fmt.Printf("%v|%.3f|%6.2v\n", c64, c64, c64[:1])
	fmt.Printf("%d|%s|%q|%t|%5d\n", c[2], c64[0], c[2], c64[0], c[2])
	fmt.Printf("%d\n", []complex64{1, 2i})
	m := map[complex128]int{2: 1, 1i: 2, -1: 3, complex(nan, 0): 4, complex(1, nan): 5, complex(1, -1): 6,
		complex(-inf, 5): 7, 1 + 1i: 8, complex(nan, 1): 9}
	fmt.Println(m)
	fmt.Println(map[complex64]string{1: "a", 1i: "b", -1i: "c", 0: "d"})
	x := uint64(0x9E3779B97F4A7C15)
	for i := 0; i < 300; i++ {
		x = x*6364136223846793005 + 1442695040888963407
		re := float64(int64(x>>11)%2000001-1000000) / 1000 / 7
		x = x*6364136223846793005 + 1442695040888963407
		im := math.Float64frombits(x)
		v := complex(re, im)
		fmt.Println(v, complex64(v))
		fmt.Printf("%f %.3f %10.4v\n", v, complex64(v), v)
	}
}
