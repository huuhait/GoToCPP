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
	if sliceCase(os.Args[1]) {
		return
	}
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
	case "substr":
		substrCase()
	case "utf8":
		utf8Case()
	case "substr-alen":
		s, i, j := "abc", 1, 5
		fmt.Println(s[i:j])
	case "substr-b":
		s, i, j := "abc", 3, 2
		fmt.Println(s[i:j])
	case "substr-neg":
		s, i := "abc", -1
		fmt.Println(s[i:])
	case "substr-from":
		s, i := "abc", uint8(4)
		fmt.Println(s[i:])
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

// --- slices (Phase 03 step 3.5a) ---

// sliceCase runs the slice case name and reports whether there is one. Each panic case ends
// the program with Go's message.
func sliceCase(name string) bool {
	switch name {
	case "slices":
		slicesCase()
	case "slice-grow":
		sliceGrowCase()
	case "slice-free":
		sliceFreeCase()
	case "slice-index":
		s, i := make([]int, 3), 5
		fmt.Println(s[i])
	case "slice-index-neg":
		s, i := make([]int, 3), -1
		fmt.Println(s[i])
	case "slice-index-u8":
		s, i := make([]int, 3), uint8(200)
		fmt.Println(s[i])
	case "slice-index-nil":
		var s []int
		i := 0
		fmt.Println(s[i])
	case "slice-hi":
		s, h := make([]int, 3, 5), 6
		fmt.Println(len(s[:h]))
	case "slice-hi-neg":
		s, h := make([]int, 3, 5), -1
		fmt.Println(len(s[:h]))
	case "slice-hi-u":
		s, h := make([]int, 3, 5), uint64(math.MaxUint64)
		fmt.Println(len(s[:h]))
	case "slice-lo":
		s, l, h := make([]int, 3, 5), 3, 2
		fmt.Println(len(s[l:h]))
	case "slice-lo-neg":
		s, l := make([]int, 3, 5), -2
		fmt.Println(len(s[l:]))
	case "slice-from":
		s, l := make([]int, 3, 5), 4
		fmt.Println(len(s[l:]))
	case "slice-nil-sub":
		var s []int
		h := 1
		fmt.Println(len(s[:h]))
	case "slice3-max":
		s, m := make([]int, 3, 5), 6
		fmt.Println(len(s[0:1:m]))
	case "slice3-max-neg":
		s, m := make([]int, 3, 5), -1
		fmt.Println(len(s[0:1:m]))
	case "slice3-max-u":
		s, m := make([]int, 3, 5), uint(math.MaxUint64)
		fmt.Println(len(s[0:1:m]))
	case "slice3-hi":
		s, h, m := make([]int, 3, 5), 4, 2
		fmt.Println(len(s[0:h:m]))
	case "slice3-hi-neg":
		s, h, m := make([]int, 3, 5), -1, 2
		fmt.Println(len(s[0:h:m]))
	case "slice3-lo":
		s, l, h, m := make([]int, 3, 5), 3, 2, 4
		fmt.Println(len(s[l:h:m]))
	case "slice3-lo-neg":
		s, l, h, m := make([]int, 3, 5), -1, 2, 4
		fmt.Println(len(s[l:h:m]))
	case "make-len":
		n := -1
		fmt.Println(len(make([]int, n)))
	case "make-len-big":
		n := 1 << 60
		fmt.Println(len(make([]int, n)))
	case "make-cap":
		n, c := 2, 1
		fmt.Println(len(make([]int, n, c)))
	case "make-cap-big":
		n, c := 1, 1<<60
		fmt.Println(len(make([]int, n, c)))
	case "slice-convert":
		s := []int{1, 2}
		a := [3]int(s)
		fmt.Println(a[0])
	default:
		return false
	}
	return true
}

// dump prints a name, the length and the elements of s.
func dump(name string, s []int) {
	fmt.Print(name, " ", len(s), " [")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Print("]\n")
}

func dumpBytes(name string, s []byte) {
	fmt.Print(name, " ", len(s), " [")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Print("]\n")
}

func slicesCase() {
	// nil and empty slices
	var n []int
	fmt.Println(n == nil, len(n), cap(n), n[0:0] == nil, len(n[:]), n[:] == nil)
	e := []int{}
	fmt.Println(e == nil, len(e), cap(e), e[0:0] == nil)
	m0 := make([]int, 0)
	fmt.Println(m0 == nil, len(m0), cap(m0), m0[:0:0] == nil)

	// make: zeroed, with the exact len and cap; sub-slices have Go's exact len and cap
	m := make([]int, 3, 10)
	dump("make", m)
	fmt.Println(len(m), cap(m), len(m[:10]), cap(m[2:]), len(m[2:5:7]), cap(m[2:5:7]), cap(m[10:10]), len(m[3:]), cap(m[3:]))
	m = m[:10]
	dump("make10", m)
	m1 := make([]int, 4)
	fmt.Println(len(m1), cap(m1))

	// two slices of one array see each other's writes
	a := []int{1, 2, 3, 4, 5}
	fmt.Println(len(a), cap(a))
	b := a[1:3]
	b[0] = 20
	dump("a", a)
	dump("b", b)
	fmt.Println(len(b), cap(b))
	b = append(b, 30) // fits: overwrites a[3]
	dump("a", a)
	dump("b", b)
	c := a[1:3:3]
	c = append(c, 99) // doesn't fit: a new array
	c[0] = -1
	dump("a", a)
	dump("c", c)
	fmt.Println(cap(c) >= len(c))

	// append forms
	var x []int
	x = append(x)
	fmt.Println(x == nil)
	x = append(x, 1)
	x = append(x, 2, 3, 4)
	x = append(x, x...)
	dump("x", x)
	x = append(x[:1], x[2:]...) // removes x[1], in place
	dump("x", x)
	y := []int{1, 2, 3, 4}
	y = append(y[:1], y[2], y[1]) // the arguments are read first
	dump("y", y)
	z := make([]int, 2, 8)
	z[0], z[1] = 7, 8
	z = append(z, z...)
	z = append(z, z...)
	dump("z", z)
	w := make([]int, 5, 10)
	for i := range w {
		w[i] = i
	}
	w2 := append(w[:1], w[0:4]...) // overlaps inside the array
	dump("w", w)
	dump("w2", w2)
	var nn []int
	nn = append(nn, []int{}...)
	fmt.Println(nn == nil)
	nn = append(nn, nil...)
	fmt.Println(nn == nil)
	nn = append(nn, []int{6, 7}...)
	dump("nn", nn)

	// copy: the number copied; overlap safe
	k := []int{1, 2, 3, 4, 5}
	fmt.Println(copy(k[1:], k))
	dump("k", k)
	k = []int{1, 2, 3, 4, 5}
	fmt.Println(copy(k, k[2:]))
	dump("k", k)
	fmt.Println(copy(k, []int{}), copy([]int(nil), k), copy(k[:2], []int{8, 9, 10}))
	dump("k", k)

	// []byte and strings
	bs := []byte{'h', 'i'}
	bs = append(bs, "llo"...)
	bs = append(bs, ""...)
	dumpBytes("bs", bs)
	nb := make([]byte, 3)
	fmt.Println(copy(nb, "abcdef"), copy(nb[2:], "z"), copy(nb, ""))
	dumpBytes("nb", nb)
	var nilb []byte
	nilb = append(nilb, ""...)
	fmt.Println(nilb == nil)
	nilb = append(nilb, "go"...)
	dumpBytes("nilb", nilb)

	// clear
	cl := []int{1, 2, 3, 4}
	clear(cl[1:3])
	dump("cl", cl)
	var cln []int
	clear(cln)

	// strings as elements
	ss := []string{"a", "b"}
	ss = append(ss, "c", "d", "e")
	t := ss[1:4]
	t[0] = "B"
	clear(ss[3:])
	fmt.Println(len(ss), ss[0], ss[1], ss[2], ss[3] == "", ss[4] == "", t[0], len(t))

	// nested slices
	grid := make([][]int, 3)
	for i := range grid {
		grid[i] = make([]int, i+1)
		grid[i][i] = i * 10
	}
	grid = append(grid, grid[1])
	grid[3][0] = 55
	fmt.Println(len(grid), grid[1][0], grid[2][2], len(grid[3]), grid[0] == nil)

	// pointers to elements keep the array alive
	pe := []int{1, 2, 3}
	p := &pe[1]
	*p = 22
	fmt.Println(pe[1])
	pe = nil
	*p += 1
	fmt.Println(*p, pe == nil)
	pe2 := []int{4, 5}
	q := &pe2[0]
	pe2 = append(pe2, 6) // a new array: q still points into the old one
	*q = 40
	fmt.Println(pe2[0], *q, len(pe2))

	// index types
	it := []int{10, 20, 30}
	u8, i8, u64 := uint8(2), int8(1), uint64(0)
	fmt.Println(it[u8], it[i8], it[u64], len(it[u8:]), len(it[i8:u8]), cap(it[i8:u8:u8]))

	// slice to array: the length check passes
	arr := [2]int(it)
	fmt.Println(arr[0], arr[1])
}

func sliceGrowCase() {
	var s []int
	ok := true
	reallocs := 0
	for i := 0; i < 1000000; i++ {
		before := cap(s)
		s = append(s, i)
		if cap(s) < len(s) {
			ok = false
		}
		if cap(s) != before {
			reallocs++
		}
	}
	sum := 0
	for _, v := range s {
		sum += v
	}
	fmt.Println(len(s), sum, ok, s[999999], reallocs < 100)
	var bs []byte
	for i := 0; i < 100000; i++ {
		bs = append(bs, byte(i))
	}
	fmt.Println(len(bs), bs[99999], cap(bs) >= len(bs))
	var ss []string
	for i := 0; i < 10000; i++ {
		ss = append(ss, "x")
	}
	fmt.Println(len(ss), ss[9999], cap(ss) >= len(ss))
	t := make([]int, 0, 3)
	t = append(t, 1, 2, 3, 4, 5, 6, 7)
	fmt.Println(len(t), cap(t) >= 7, t[6])
	fmt.Println(cap(make([]int, 5)), cap(make([]int, 0, 7)), cap([]int{1, 2, 3}))
}

type node struct {
	kids []node
	v    int
}

func nodeVals(q []node) {
	fmt.Print(len(q), " [")
	for i, n := range q {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(n.v)
	}
	fmt.Print("]\n")
}

// sliceFreeCase stores elements whose source array is owned only by the element that the
// store overwrites (q[0].kids): in gocpp, the store must not free the source while it reads it.
func sliceFreeCase() {
	q := []node{{kids: []node{{v: 5}, {v: 6}, {v: 7}}}}
	q = append(q[:0], q[0].kids...)
	nodeVals(q)
	q = []node{{kids: []node{{v: 1}}}, {v: 5}}
	fmt.Println(copy(q[:1], q[0].kids))
	nodeVals(q)
	q = []node{{kids: []node{{v: 3}, {v: 5}, {v: 6}, {v: 7}}}, {v: 1}, {v: 2}}
	fmt.Println(copy(q, q[0].kids))
	nodeVals(q)
	q = []node{{kids: []node{{v: 5}, {v: 8}}}}
	q = append(q[:0], q[0].kids[0])
	nodeVals(q)
	q = []node{{kids: []node{{v: 5}, {v: 8}}}}
	q = append(q[:0], q[0].kids[1], q[0].kids[0])
	nodeVals(q)
	k := []node{{v: 9}}
	k[0].kids = k
	clear(k[0].kids)
	fmt.Println(k[0].v, k[0].kids == nil)
}

func substrCase() {
	s := "hello, world"
	i, j := 7, 12
	fmt.Println(s[i:j], s[i:], s[:5], s[:], s[j:], len(s[3:3]), s[1:4] == "ell", s[i:j][1:3])
	t := s[:5] + "!" + s[i:] + "?"
	fmt.Println(t, len(t), t[5], s[0:0] == "")
	u := ""
	for k := 0; k < 5; k++ {
		u += s[k:k+1] + "-"
	}
	fmt.Println(u, len(u))
}

func showRunes(s string) {
	for i, r := range s {
		fmt.Printf("%d:%d ", i, r)
	}
	fmt.Println(len(s))
}

func showString(r rune) {
	s := string(r)
	fmt.Printf("%d:", len(s))
	for i := 0; i < len(s); i++ {
		fmt.Printf(" %d", s[i])
	}
	fmt.Println()
}

func utf8Case() {
	showRunes("")
	showRunes("abc")
	showRunes("h\u00e9llo, \u4e16\u754c \U0001F600")
	showRunes("a\xffb\xc0\x80c")
	showRunes("\xe2\x82 \xed\xa0\x80 \xf4\x90\x80\x80 \xf0\x9f\x98")
	showRunes("\xc2\x80\xdf\xbf\xe0\xa0\x80\xef\xbf\xbf\xf0\x90\x80\x80\xf4\x8f\xbf\xbf")
	for _, r := range "A\u00e9\u20ac\U0001F600" {
		showString(r)
	}
	showString(0)
	showString(-1)
	showString(0xD800)
	showString(0xDFFF)
	showString(0x10FFFF)
	showString(0x110000)
	showString(-2147483648)
}
