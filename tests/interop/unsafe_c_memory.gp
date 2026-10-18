// phase: 04

// C memory seen from gocpp (step 4.10): unsafe.Slice makes a gocpp slice over C's array (a
// pointer from C owns nothing, so the slice is a view of C's bytes and C keeps them alive),
// unsafe.String a string over C's bytes without a copy, a C array member is sliced with a[lo:hi]
// as in cgo, and unsafe.SliceData gives the pointer of a gocpp slice back to C.
package main

import (
	"fmt"
	"unsafe"

	cstring "<string.h>"

	"./umem.hpp"
)

func main() {
	// An array of C strings, as argv is: **C.char to a []*C.char.
	argv := unsafe.Slice(umem.get_argv(), 3)
	for i, a := range argv {
		fmt.Print(i, "=", C.GoString(a), " ")
	}
	fmt.Println(len(argv), cap(argv))
	flag := unsafe.Slice(argv[1], 2) // an element of argv is a C pointer again
	fmt.Println(C.GoStringN(&flag[0], C.int(len(flag))), len(unsafe.Slice(argv[1:][0], 1)))
	words := unsafe.Slice(umem.get_words(), 3) // an array of const char* const
	fmt.Println(C.GoString(words[0]), C.GoString(words[2]), len(words))
	lens := ""
	for _, w := range words {
		lens += fmt.Sprint(int(cstring.strlen(w)), " ")
	}
	fmt.Println(lens + "|")

	// Array members of a C struct: a[:] is a gocpp slice over C's memory.
	rec := umem.get_rec()
	name := rec.name[:]
	fmt.Println(len(name), name[0] == 'r', C.GoString(&name[0]))
	vals := rec.vals[:]
	vals[1] = 99 // C's array changes
	fmt.Println(rec.vals[1], len(vals), cap(vals), vals)
	mid := rec.vals[1:3]
	fmt.Println(len(mid), cap(mid), mid)
	head := rec.vals[:2:3]
	fmt.Println(len(head), cap(head), head)
	rec.vals[0] = 7
	fmt.Println(vals[0], mid[0], head[0])
	pts := rec.pts[:]
	fmt.Println(len(pts), int(pts[1].y), int(umem.sum_points(&pts[0], C.int(len(pts)))))
	fmt.Println(int(umem.sum_ints(unsafe.SliceData(vals), C.size_t(len(vals)))))

	// A slice of C bytes can be appended to, which copies it into gocpp's memory.
	grown := append(name[:2:2], 'x')
	grown[0] = 'R' // a copy: C's name stays
	fmt.Println(C.GoString(&name[0]), len(grown), grown[0] == 'R')

	// A pointer to an array of C structs, as a gocpp slice.
	recs := unsafe.Slice(rec, 1)
	fmt.Println(len(recs), int(recs[0].pts[0].x), recs[0].vals[3])

	// A string over C memory without a NUL after its bytes: no copy, and no byte read past them.
	blob := umem.get_blob(6)
	s := unsafe.String(blob, 6)
	fmt.Println(s, len(s), s[1:3], s == "abcdef")
	m := unsafe.Slice(blob, 6)
	m[0] = 'A'
	fmt.Println(s[:3], int(cstring.strlen(C.cstr(s[:4]))))
	// The empty and the nil cases.
	var none *C.uchar
	fmt.Println(unsafe.String(none, 0) == "", unsafe.Slice(none, 0) == nil, len(unsafe.Slice(blob, 0)), unsafe.Slice(blob, 0) != nil)
	C.free(blob)
}

// Output:
// 0=prog 1=-v 2=file 3 3
// -v 1
// alpha gamma 3
// 5 4 5 |
// 8 true rec
// 99 4 4 [10 99 30 40]
// 2 3 [99 30]
// 2 3 [10 99]
// 7 99 7
// 2 4 10
// 176
// rec 3 true
// 1 1 40
// abcdef 6 bc true
// Abc 4
// true true 0 true
