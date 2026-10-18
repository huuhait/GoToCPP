// phase: 04

// package unsafe on gocpp memory (step 4.10, D16): Pointer conversions, Add, uintptr arithmetic,
// SliceData, Slice, String and StringData, as Go's rules give them. unsafe.Pointer keeps its
// object alive (here: a pointer is the only owner of the int), and a string over a slice's array
// sees the slice's bytes. Real Go prints the same (the Sizeof of a pointer is left out: D15).
package main

import (
	"fmt"
	"unsafe"
)

type Header struct {
	kind  uint8
	flags uint8
	size  uint32
}

type Builder struct {
	buf []byte
}

func (b *Builder) WriteString(s string) { b.buf = append(b.buf, s...) }
func (b *Builder) String() string {
	return unsafe.String(unsafe.SliceData(b.buf), len(b.buf))
}

func newInt(v int64) unsafe.Pointer {
	p := new(int64)
	*p = v
	return unsafe.Pointer(p)
}

func greeting() string {
	var b Builder
	b.WriteString("hello, ")
	b.WriteString("unsafe")
	return b.String()
}

func main() {
	x := 5
	p := unsafe.Pointer(&x)
	*(*int)(p) = 6
	fmt.Println(x, p == unsafe.Pointer(&x), p != nil)

	q := newInt(41)
	*(*int64)(q)++
	fmt.Println(*(*int64)(q))

	nums := [4]int32{10, 20, 30, 40}
	base := unsafe.Pointer(&nums[0])
	third := (*int32)(unsafe.Add(base, 2*unsafe.Sizeof(nums[0])))
	*third = 33
	fmt.Println(*third, *(*int32)(unsafe.Pointer(uintptr(base) + 4)), nums)

	h := Header{1, 2, 300}
	*(*uint32)(unsafe.Add(unsafe.Pointer(&h), unsafe.Offsetof(h.size))) = 512
	fmt.Println(h)

	var none unsafe.Pointer
	m := map[unsafe.Pointer]string{p: "x", q: "q"}
	fmt.Println(none == nil, m[p], m[q], len(m), m[none] == "")
	fmt.Println(none)

	s := greeting()
	fmt.Println(s, len(s))

	buf := make([]byte, 3, 8)
	copy(buf, "abc")
	bs := unsafe.String(unsafe.SliceData(buf), len(buf))
	buf[0] = 'X'
	fmt.Println(bs)
	buf = nil
	fmt.Println(bs, unsafe.String(&[]byte("raw")[0], 2))

	sd := unsafe.StringData("data")
	fmt.Println(*sd, *(*byte)(unsafe.Add(unsafe.Pointer(sd), 3)))
	words := []string{"a", "b", "c"}
	sl := unsafe.Slice(unsafe.SliceData(words), 2)
	sl[1] = "B"
	fmt.Println(sl, words, len(sl), cap(sl))
	ints := []int{1, 2, 3, 4, 5}
	tail := unsafe.Slice(&ints[2], 3)
	tail[0] = 30
	fmt.Println(tail, ints, cap(tail))
	var np *int
	fmt.Println(unsafe.Slice(np, 0) == nil, unsafe.String(nil, 0) == "", unsafe.SliceData([]int(nil)) == nil)
	fmt.Println(unsafe.Slice(&ints[0], 0) != nil, len(unsafe.Slice(&ints[1], 0)))
}

// Output:
// 6 true true
// 42
// 33 20 [10 20 33 40]
// {1 2 512}
// true x q 2 true
// <nil>
// hello, unsafe 13
// Xbc
// Xbc ra
// 100 97
// [a B] [a B c] 2 2
// [30 4 5] [1 2 30 4 5] 3
// true true true
// true 0
