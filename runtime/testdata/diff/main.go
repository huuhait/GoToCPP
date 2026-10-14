// The Go side of the runtime's differential test: runtime/test/diff.cpp is the same program
// written against the C++ runtime, as gocpp would generate it. runtime/cxx_test.go runs both
// with each case name below and compares their exit codes and their stdout and stderr,
// merged into one stream (up to Go's stack trace).
package main

import (
	"fmt"
	"math"
	"os"
	"weak"
)

func main() {
	switch os.Args[1] {
	case "println":
		printlnCase()
	case "fmt":
		fmtCase()
	case "floats":
		floatsCase()
	case "arith":
		arithCase()
	case "f2i":
		f2iCase()
	case "strings":
		stringsCase()
	case "order":
		fmt.Print("a\n")
		println("b")
		fmt.Println("c")
		print("d\n")
		fmt.Printf("e%d\n", 1)
		fmt.Print("no newline, ")
		println("then stderr")
		panic("p")
	case "panic-str":
		fmt.Println("before")
		panic("boom")
	case "panic-multi":
		panic("a\nb\nc")
	case "panic-empty":
		panic("")
	case "panic-int":
		panic(42)
	case "panic-float":
		panic(1.5)
	case "panic-bool":
		panic(true)
	case "div":
		z := 0
		fmt.Println(10 / z)
	case "rem":
		z := int8(0)
		fmt.Println(int8(10) % z)
	case "shl":
		n := -1
		fmt.Println(1 << n)
	case "shr":
		n := int8(-5)
		fmt.Println(uint8(1) >> n)
	case "index":
		s, i := "abc", 5
		fmt.Println(s[i])
	case "negindex":
		s, i := "abc", -1
		fmt.Println(s[i])
	case "uindex":
		s, i := "abc", uint64(math.MaxUint64)
		fmt.Println(s[i])
	case "ptr":
		ptrCase()
	case "nil-deref":
		var p *int
		fmt.Println(*p)
	case "nil-store":
		var p *int
		*p = 1
	case "nil-field":
		var p *point
		fmt.Println(p.X)
	case "nil-method":
		var p *point
		fmt.Println(p.Sum())
	case "nil-addr":
		var p *point
		q := &p.Y
		fmt.Println(q == nil)
	case "exit0":
		fmt.Print("only stdout\n")
	case "big":
		fmt.Print(bigString())
		println("done")
	case "bigerr":
		// The builtin println ignores a broken pipe: the program goes on.
		println(bigString())
		println("still alive")
		os.Exit(3)
	}
}

// bigString is 1 MiB of lines: much more than a pipe holds.
func bigString() string {
	s := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde\n"
	for len(s) < 1<<20 {
		s = s + s
	}
	return s
}

func printlnCase() {
	var p *int
	println(true, false, -5, int8(-128), uint64(math.MaxUint64), int64(math.MinInt64), "str", 'x', "")
	println(0.0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), 1e300, 5e-324, 1e21)
	println(123456789.0, float32(0.1), float32(1.5), 0.1, 2.5e-10, 1.7976931348623157e308, 9.9999999)
	println(p)
	println()
	println(1.5, 3.0, -0.25, 'x')
	print("a", 1, 2, "b", true, 1.5, "\n")
	print()
	print("\n")
	println(byte(255), uint16(65535), uint32(4294967295), int16(-32768), int32(-2147483648))
}

func fmtCase() {
	fmt.Println(123456.0, 1234567.0, 1000000.0, 1e20, 1e21, 0.0001, 0.00001, 3.0, 2.5, float32(0.1), float32(16777216.0), float32(1e-5))
	fmt.Println(math.Inf(1), math.Inf(-1), math.NaN(), math.Copysign(0, -1), 0.0, 5e-324, 1.7976931348623157e308, -1.5e-7, 100000.0, 999999.0, float32(3.4028235e38))
	fmt.Println("x", 1, true, int8(-3), uint64(math.MaxUint64), 'a', byte(7))
	fmt.Println()
	fmt.Print("a", "b", 1, 2, true, "c", 3.5, "\n")
	fmt.Print(1, 2, "\n")
	fmt.Print("a", 1, 2, "b", "\n")
	fmt.Print()
	fmt.Print(true, false, 1.5, float32(2.5), "\n")
	fmt.Printf("%v|%d|%s|%t|%f|%v|%f|%f|%v\n", 1.5, -42, "s", true, 3.14159265, "str", 1e300, math.Inf(-1), int8(5))
	fmt.Printf("%f %f %f %f %f\n", 0.0, 1e-7, 2.5, float32(0.1), math.Copysign(0, -1))
	fmt.Printf("%d %s %t %f\n", "x", 5, 1, 7)
	fmt.Printf("%v %d\n", 1)
	fmt.Printf("%d\n", 1, 2)
	fmt.Printf("%d\n", 1, 2, "x", 2.5, true)
	fmt.Printf("100%% %z %\n", 3)
	fmt.Printf("%v %v\n", true, 'x')
	fmt.Printf("%d %d %v\n", uint8(200), uint64(math.MaxUint64), float32(0.1))
	fmt.Printf("%s %d %t\n", true, 1.5, float32(0.5))
	fmt.Printf("%s %s %s %s\n", int8(1), uint16(2), int32(3), uint32(4))
	fmt.Printf("%!\n")
	fmt.Printf("%é\n", 1)
	fmt.Printf("%t %v\n", "s", math.NaN())
	fmt.Printf("%f %f %f\n", math.NaN(), math.Inf(1), -2.5)
	fmt.Printf("%v %v\n", math.Copysign(0, -1), float32(-0.0))
	fmt.Printf("no verbs\n")
	fmt.Printf("end%")
	fmt.Println()
}

// floatsCase prints many floats in both formats: random bit patterns, and decimal-looking
// values at many scales.
func floatsCase() {
	x := uint64(0x9E3779B97F4A7C15)
	scales := []float64{1e-10, 1e-5, 1e-4, 1e-3, 0.1, 1, 10, 1e5, 1e6, 1e7, 1e20, 1e21, 1e22}
	for i := 0; i < 1500; i++ {
		x = x*6364136223846793005 + 1442695040888963407
		f := math.Float64frombits(x)
		g := math.Float32frombits(uint32(x >> 32))
		h := float64(int64(x>>11)%2000001-1000000) / 1000
		k := h / 7 * scales[x%uint64(len(scales))]
		k32 := float32(k)
		fmt.Println(f, g, h, k, k32)
		println(f, g, h, k, k32)
		fmt.Printf("%f %f %f %v\n", f, g, k, k32)
	}
}

type integer interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

var counts = []int64{0, 1, 2, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 64, 65, 200, 1 << 40}

func table[T integer](name string, vals []T) {
	for _, a := range vals {
		for _, b := range vals {
			if b != 0 {
				fmt.Println(name, a, "/", b, "=", a/b, a%b)
			}
		}
	}
	for _, a := range vals {
		for _, n := range counts {
			fmt.Println(name, a, "<<>>", n, "=", a<<n, a>>n, a<<uint8(n), a>>uint64(n))
		}
	}
}

func arithCase() {
	table("int8", []int8{math.MinInt8, math.MinInt8 + 1, -7, -2, -1, 0, 1, 2, 3, 7, math.MaxInt8 - 1, math.MaxInt8})
	table("int16", []int16{math.MinInt16, math.MinInt16 + 1, -7, -1, 0, 1, 2, 7, math.MaxInt16})
	table("int32", []int32{math.MinInt32, math.MinInt32 + 1, -7, -1, 0, 1, 2, 7, math.MaxInt32})
	table("int64", []int64{math.MinInt64, math.MinInt64 + 1, -7, -1, 0, 1, 2, 7, math.MaxInt64})
	table("uint8", []uint8{0, 1, 2, 3, 7, 128, math.MaxUint8 - 1, math.MaxUint8})
	table("uint16", []uint16{0, 1, 2, 7, 1 << 15, math.MaxUint16})
	table("uint32", []uint32{0, 1, 2, 7, 1 << 31, math.MaxUint32})
	table("uint64", []uint64{0, 1, 2, 7, 1 << 63, math.MaxUint64})
}

var f2iVals = []float64{0, -0.5, 0.9, -1, 1.5, 127.9, 128, -129, 255.5, 256, 300, -300, 32768, 65536 + 7, -32769,
	2147483647, 2147483648, -2147483648, -2147483648.9, -2147483649, 3e9 + 7, 4294967296 + 9, -3e9,
	9.2233720368547748e18, 9223372036854775808, -9223372036854775808, -9.223372036854777e18,
	1.8446744073709550e19, 18446744073709551616, 1e300, -1e300, math.Inf(1), math.Inf(-1), math.NaN()}

func f2iCase() {
	for _, f := range f2iVals {
		fmt.Println(f, int64(f), int32(f), int16(f), int8(f), uint64(f), uint32(f), uint16(f), uint8(f))
	}
	for _, f64 := range f2iVals {
		f := float32(f64)
		fmt.Println(f, int64(f), int32(f), int16(f), int8(f), uint64(f), uint32(f), uint16(f), uint8(f))
	}
}

func stringsCase() {
	a, b, e := "abc", "abd", ""
	s := a + b
	fmt.Println(s, len(s), s[0], s[5], len(e), len(a+e), e+e == e)
	fmt.Println(a < b, a == b, a != b, a <= b, a > b, a >= b, e < a, "\xff" > a, "ab" < a, s+e == s)
	t := "x"
	for i := 0; i < 3; i++ {
		t += "yz"
	}
	fmt.Println(t, len(t), t[len(t)-1])
	println(s, len(s), s[2], t)
	fmt.Printf("%s|%v|%d\n", s, t, len(s))
	fmt.Println("tab\tnl\\n bytes:\303\251", len("\303\251"), "a\000b" == "a\000b", len("a\000b"))
}

type point struct{ X, Y int }

func (p point) Sum() int { return p.X + p.Y }

// other has the same underlying type as point, so (*other)(p) converts a *point.
type other point

var globalPoint point
var globalArr [3]int

// newCounter returns the address of a local: the local lives on (gocpp boxes it).
func newCounter() *int {
	c := 1
	return &c
}

func ptrCase() {
	p := new(int)
	fmt.Println(*p, p == nil, p != nil)
	*p = 5
	q := p
	*q += 2
	fmt.Println(*p, *q, p == q)
	r := new(int)
	*r = 7
	fmt.Println(*r == *p, r == p)
	var n *int
	fmt.Println(n == nil, n == (*int)(nil), n == r)

	c, d := newCounter(), newCounter()
	*c++
	fmt.Println(*c, *d, c == d)

	pt := &point{1, 2}
	py := &pt.Y
	*py = 7
	fmt.Println(pt.X, pt.Y, *py, pt.Sum(), py == &pt.Y, &pt.X == &pt.Y)
	pt = nil
	*py = 8 // the point lives on through py
	fmt.Println(*py, pt == nil)

	g := &globalPoint
	g.X = 3
	fmt.Println(globalPoint.X, &globalPoint == g, &globalPoint.X == &g.X, &globalPoint.X == &globalPoint.Y)
	e := &globalArr[1]
	*e = 9
	fmt.Println(globalArr[1], e == &globalArr[1], e == &globalArr[2])

	o := (*other)(g)
	o.Y = 11
	fmt.Println(globalPoint.Y, (*point)(o) == g)

	pt2 := &point{4, 5}
	w := weak.Make(pt2)
	fmt.Println(w.Value() == pt2, w == weak.Make(pt2), w.Value().X, w == weak.Make(&point{4, 5}))
	wy := weak.Make(&pt2.Y)
	fmt.Println(*wy.Value(), wy.Value() == &pt2.Y)
	wg := weak.Make(&globalPoint)
	fmt.Println(wg.Value() == &globalPoint, wg == weak.Make(g), wg.Value().X)
	var z weak.Pointer[point]
	fmt.Println(z.Value() == nil, z == weak.Make[point](nil), z == w)
}
