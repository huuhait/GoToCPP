// phase: 09

// Scanning into pointers to named types (Go reaches them through reflect: the kind decides), into fields, elements, map values via
// a pointer, struct fields of struct pointers, embedded fields, pointers made by new, pointers to the elements of arrays and slices,
// pointers inside interfaces; and the verb check that a predeclared float has and a named float does not.
package main

import (
	"fmt"
)

type (
	Bool       bool
	Int        int
	Int8       int8
	Int16      int16
	Int32      int32
	Int64      int64
	Uint       uint
	Uint8      uint8
	Uint16     uint16
	Uint32     uint32
	Uint64     uint64
	Uintptr    uintptr
	Float32    float32
	Float64    float64
	Complex64  complex64
	Complex128 complex128
	String     string
	Bytes      []byte
	Rune       rune
	Byte       byte
	Celsius    float64
	ID         string
	Chain      Int
)

type rec struct {
	Name  string
	Age   int
	Tags  [2]string
	Score Celsius
	Inner struct{ X, Y int8 }
	P     *int
	B     []byte
}

type emb struct {
	rec
	Extra Int
}

func main() {
	var (
		b    Bool
		i    Int
		i8   Int8
		i16  Int16
		i32  Int32
		i64  Int64
		u    Uint
		u8   Uint8
		u16  Uint16
		u32  Uint32
		u64  Uint64
		up   Uintptr
		f32  Float32
		f64  Float64
		c64  Complex64
		c128 Complex128
		s    String
		bs   Bytes
		r    Rune
		by   Byte
		t    Celsius
		id   ID
		ch   Chain
	)
	n, err := fmt.Sscan("true -5 -6 -7 -8 -9 5 6 7 8 9 10 1.5 2.5 (1+2i) (3+4i) str bytes 65 66 36.6 id-1 77",
		&b, &i, &i8, &i16, &i32, &i64, &u, &u8, &u16, &u32, &u64, &up, &f32, &f64, &c64, &c128, &s, &bs, &r, &by, &t, &id, &ch)
	fmt.Println(n, err)
	fmt.Println(b, i, i8, i16, i32, i64, u, u8, u16, u32, u64, up, f32, f64, c64, c128, s, string(bs), r, by, t, id, ch)
	fmt.Printf("%T %T %T %T\n", b, bs, t, ch)

	fmt.Println("-- overflow on named types")
	n, err = fmt.Sscan("200", &i8)
	fmt.Println(n, err, i8)
	n, err = fmt.Sscan("300", &u8)
	fmt.Println(n, err, u8)
	n, err = fmt.Sscan("-1", &u16)
	fmt.Println(n, err, u16)
	n, err = fmt.Sscan("1e50", &f32)
	fmt.Println(n, err, f32)
	n, err = fmt.Sscan("(1e50+1i)", &c64)
	fmt.Println(n, err, c64)

	fmt.Println("-- the verb check of float32 and float64 only for the predeclared types")
	var pf32 float32
	var pf64 float64
	var pc64 complex64
	n, err = fmt.Sscanf("1.5", "%d", &pf32)
	fmt.Println(n, err, pf32)
	n, err = fmt.Sscanf("1.5", "%d", &f32)
	fmt.Println(n, err, f32)
	n, err = fmt.Sscanf("1.5", "%s", &pf64)
	fmt.Println(n, err, pf64)
	n, err = fmt.Sscanf("1.5", "%s", &t)
	fmt.Println(n, err, t)
	n, err = fmt.Sscanf("1.5", "%x", &t)
	fmt.Println(n, err, t)
	n, err = fmt.Sscanf("(1+1i)", "%d", &pc64)
	fmt.Println(n, err, pc64)
	n, err = fmt.Sscanf("(1+1i)", "%d", &c64)
	fmt.Println(n, err, c64)
	n, err = fmt.Sscanf("true", "%d", &b)
	fmt.Println(n, err, b)
	n, err = fmt.Sscanf("abc", "%d", &s)
	fmt.Println(n, err, s)
	n, err = fmt.Sscanf("abc", "%d", &bs)
	fmt.Println(n, err, bs)
	n, err = fmt.Sscanf("12", "%s", &i)
	fmt.Println(n, err, i)

	fmt.Println("-- fields, elements, new")
	var x rec
	n, err = fmt.Sscan("bob 42 a b 36.6 1 2", &x.Name, &x.Age, &x.Tags[0], &x.Tags[1], &x.Score, &x.Inner.X, &x.Inner.Y)
	fmt.Println(n, err, x)
	p := &x
	n, err = fmt.Sscan("eve 7", &p.Name, &p.Age)
	fmt.Println(n, err, *p)
	x.P = new(int)
	n, err = fmt.Sscan("99 bytes", x.P, &x.B)
	fmt.Println(n, err, *x.P, string(x.B))
	var e emb
	n, err = fmt.Sscan("al 3 5", &e.Name, &e.Age, &e.Extra)
	fmt.Println(n, err, e)
	arr := [3]int{}
	sl := make([]int8, 3)
	n, err = fmt.Sscan("1 2 3 4 5 6", &arr[0], &arr[1], &arr[2], &sl[0], &sl[1], &sl[2])
	fmt.Println(n, err, arr, sl)
	pn := new(Chain)
	n, err = fmt.Sscan("31", pn)
	fmt.Println(n, err, *pn)
	var pp *Int = new(Int)
	n, err = fmt.Sscan("32", pp)
	fmt.Println(n, err, *pp)
	m := map[string]*int{"a": new(int), "b": new(int)}
	n, err = fmt.Sscan("10 20", m["a"], m["b"])
	fmt.Println(n, err, *m["a"], *m["b"])
	var anys []any
	anys = append(anys, &i, &s, &f64)
	n, err = fmt.Sscan("-1 two 3.5", anys...)
	fmt.Println(n, err, i, s, f64)
	func(vals ...any) {
		n, err := fmt.Sscan("4 5", vals...)
		fmt.Println(n, err, i, i8)
	}(&i, &i8)
	var iface any = &i64
	n, err = fmt.Sscan("123", iface)
	fmt.Println(n, err, i64)
	n, err = fmt.Sscan("5", &iface)
	fmt.Println(n, err)
	n, err = fmt.Sscan("1 2 3")
	fmt.Println(n, err)

	fmt.Println("-- the same pointer twice, strings and bytes copy")
	var same int
	n, err = fmt.Sscan("1 2 3", &same, &same, &same)
	fmt.Println(n, err, same)
	var keep []byte
	var src = "xyz"
	n, err = fmt.Sscan(src, &keep)
	keep[0] = 'A'
	fmt.Println(n, err, string(keep), src)
}

// Output:
// 23 <nil>
// true -5 -6 -7 -8 -9 5 6 7 8 9 10 1.5 2.5 (1+2i) (3+4i) str bytes 65 66 36.6 id-1 77
// main.Bool main.Bytes main.Celsius main.Chain
// -- overflow on named types
// 0 integer overflow on token 200 -6
// 0 unsigned integer overflow on token 300 6
// 0 expected integer 7
// 0 strconv.ParseFloat: parsing "1e50": value out of range 1.5
// 0 strconv.ParseFloat: parsing "1e50": value out of range (1+2i)
// -- the verb check of float32 and float64 only for the predeclared types
// 0 bad verb '%d' for float32 0
// 1 <nil> 1.5
// 0 bad verb '%s' for float64 0
// 1 <nil> 1.5
// 1 <nil> 1.5
// 0 bad verb '%d' for complex (0+0i)
// 0 bad verb '%d' for complex (1+2i)
// 0 bad verb '%d' for boolean true
// 0 bad verb '%d' for string str
// 0 bad verb '%d' for string [98 121 116 101 115]
// 0 bad verb '%s' for integer -5
// -- fields, elements, new
// 7 <nil> {bob 42 [a b] 36.6 {1 2} <nil> []}
// 2 <nil> {eve 7 [a b] 36.6 {1 2} <nil> []}
// 2 <nil> 99 bytes
// 3 <nil> {{al 3 [ ] 0 {0 0} <nil> []} 5}
// 6 <nil> [1 2 3] [4 5 6]
// 1 <nil> 31
// 1 <nil> 32
// 2 <nil> 10 20
// 3 <nil> -1 two 3.5
// 2 <nil> 4 5
// 1 <nil> 123
// 0 can't scan type: *interface {}
// 0 <nil>
// -- the same pointer twice, strings and bytes copy
// 3 <nil> 3
// 1 <nil> Ayz xyz
