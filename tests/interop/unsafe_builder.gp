// phase: 04

// The ASan test of step 4.10: strings.Builder as Go writes it, a string that is unsafe.String
// over the builder's byte buffer. The Builder is gone when build returns, and the string still
// reads the bytes: unsafe.String and unsafe.SliceData keep the buffer's owner (D16), so nothing
// dangles. With a view that owned nothing, AddressSanitizer reports a heap-use-after-free here
// (make ci-sanitize runs this test with it). The second half builds a string over C's memory,
// which has no NUL after its bytes: C.cstr must copy it, and a string without an owner would
// read one byte too far (heap-buffer-overflow).
package main

import (
	"fmt"
	"unsafe"

	cstring "<string.h>"
	"<stdlib.h>"

	"./umem.hpp"
)

// A Builder is Go's strings.Builder, with its copy check done on an address (the comparison of
// a *Builder with itself would be a reference cycle here: the Builder would own itself).
type Builder struct {
	addr uintptr
	buf  []byte
}

func (b *Builder) copyCheck() {
	if b.addr == 0 {
		b.addr = uintptr(unsafe.Pointer(b))
	} else if b.addr != uintptr(unsafe.Pointer(b)) {
		panic("strings: illegal use of non-zero Builder copied by value")
	}
}

func (b *Builder) String() string {
	return unsafe.String(unsafe.SliceData(b.buf), len(b.buf))
}

func (b *Builder) Len() int { return len(b.buf) }
func (b *Builder) Cap() int { return cap(b.buf) }

func (b *Builder) Reset() {
	b.addr = 0
	b.buf = nil
}

func (b *Builder) grow(n int) {
	buf := make([]byte, len(b.buf), 2*cap(b.buf)+n)
	copy(buf, b.buf)
	b.buf = buf
}

func (b *Builder) Grow(n int) {
	b.copyCheck()
	if n < 0 {
		panic("strings.Builder.Grow: negative count")
	}
	if cap(b.buf)-len(b.buf) < n {
		b.grow(n)
	}
}

func (b *Builder) Write(p []byte) int {
	b.copyCheck()
	b.buf = append(b.buf, p...)
	return len(p)
}

func (b *Builder) WriteByte(c byte) {
	b.copyCheck()
	b.buf = append(b.buf, c)
}

func (b *Builder) WriteRune(r rune) int {
	b.copyCheck()
	n := len(b.buf)
	b.buf = append(b.buf, string(r)...)
	return len(b.buf) - n
}

func (b *Builder) WriteString(s string) int {
	b.copyCheck()
	b.buf = append(b.buf, s...)
	return len(s)
}

// build makes a string and drops its Builder: only the string keeps the buffer.
func build(n int) string {
	var b Builder
	for i := 0; i < n; i++ {
		b.WriteString("item ")
		b.WriteByte(byte('0' + i%10))
		b.WriteRune('é')
		b.WriteByte('\n')
	}
	return b.String()
}

func main() {
	small := build(3)
	fmt.Print(small)
	big := build(5000) // many reallocations of the buffer, then the Builder is freed
	fmt.Println(len(big), big[:9] == "item 0é\n", big[len(big)-3:] == "9é\n"[0:0]+big[len(big)-4:][1:])

	// More strings and builders in between: nothing that big or small points at is reused.
	var others []string
	for i := 0; i < 20; i++ {
		others = append(others, build(i))
	}
	fmt.Println(len(others[19]), small[:6], len(big))

	// A string over the buffer sees its bytes (unsafe.String shares them), and keeps the array
	// alive after the slice is dropped.
	buf := make([]byte, 0, 16)
	buf = append(buf, "hello"...)
	s := unsafe.String(&buf[0], len(buf))
	buf[0] = 'J'
	fmt.Println(s)
	buf = nil
	fmt.Println(s, len(s))
	sub := s[1:4]
	s = ""
	fmt.Println(sub)

	// A Builder that is reset and reused leaves the strings it gave out alone.
	var b Builder
	b.WriteString("first")
	first := b.String()
	b.Reset()
	b.WriteString("second!")
	fmt.Println(first, b.String(), b.Len())
	b.Grow(100)
	fmt.Println(b.String(), b.Cap() >= 107)

	// A string over C's memory: five bytes from malloc, no NUL after them.
	blob := umem.get_blob(5)
	cs := unsafe.String(blob, 5)
	fmt.Println(cs, len(cs), int(umem.view_len(cs)))
	fmt.Println(int(cstring.strlen(C.cstr(cs))), cs+"!"+cs[2:])
	bs := unsafe.Slice(blob, 5)
	bs[0] = 'z'
	fmt.Println(cs[:2], bs[1], len(bs), cap(bs))
	C.free(blob)
}

// Output:
// item 0é
// item 1é
// item 2é
// 45000 true true
// 171 item 0 45000
// Jello
// Jello 5
// ell
// first second! 7
// second! true
// abcde 5 5
// 5 abcde!cde
// zb 98 5 5
