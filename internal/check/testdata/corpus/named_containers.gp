package main

// Named gocpp arrays, slices, maps, pointers and funcs with C parts: what is taken out of a value
// of one has no type in go/types, and is opaque like the same expression for the unnamed type.

type Chars [4]C.char
type Many [2][2]C.int
type List []C.int
type Dict map[string]C.int
type Ptr *C.int
type Fn func() C.int

func main() {
	var a Chars
	a[0] = 100
	println(a[0], len(a))
	for i := range a {
		println(i)
	}
	for i, v := range a {
		println(i, v)
	}
	b := a[1:3]
	_ = b
	c := a
	c[0] = 1

	var m Many
	m[1][1] = 3

	var l List
	l = append(l, 3)
	println(l[0], len(l))
	for _, v := range l {
		println(v)
	}

	var d Dict
	d["x"] = 1
	println(d["x"])

	var p Ptr
	println(p == nil)

	var f Fn
	println(f == nil)
}
