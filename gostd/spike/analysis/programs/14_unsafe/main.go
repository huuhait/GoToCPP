// unsafe: Sizeof, Alignof and Offsetof of Go-layout types, unsafe.String and unsafe.SliceData,
// a pointer round trip through unsafe.Pointer, and a nil typed pointer. The numbers are the
// linux/amd64 ones (int is 8 bytes, int64 is 8-aligned); the manifest records that.
package main

import "unsafe"

type S struct {
	A int32
	B int64
	C int32
}

func main() {
	var v S
	println(unsafe.Sizeof(int(0)), unsafe.Sizeof(int32(0)), unsafe.Sizeof(int64(0)))
	println(unsafe.Alignof(int64(0)))
	println(unsafe.Offsetof(v.A), unsafe.Offsetof(v.B), unsafe.Offsetof(v.C))
	println(unsafe.Sizeof(v))
	b := []byte("hello")
	s := unsafe.String(unsafe.SliceData(b), len(b))
	println(s, len(s))
	p := unsafe.Pointer(&b[0])
	bp := (*byte)(p)
	println(*bp)
	q := (*S)(nil)
	println(q == nil)
	println(unsafe.Sizeof(struct {
		A int32
		B int64
	}{}))
}
