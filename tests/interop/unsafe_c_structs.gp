// phase: 04

// Pointers to C/C++ values (step 4.10): &T{...} makes a gocpp pointer that owns the new object
// (it converts to the raw T* that C takes, borrowed), &s.f and &v[i] are raw addresses into C/C++
// memory (C++ lifetime rules), (*T)(p) converts an unsafe.Pointer back to a C pointer, the old
// (*[N]T)(p)[:n:n] idiom makes a slice over C memory, and a C function's address goes through
// unsafe.Pointer and comes back as its function pointer type.
package main

import (
	"fmt"
	"unsafe"

	"<vector>"

	"./umem.hpp"
)

type Shape struct {
	name string
	at   C.int
}

// scaleAll scales the points that p points to, through a pointer to the first one.
func scaleAll(p *umem.upoint, n int, k C.int) {
	for i := 0; i < n; i++ {
		umem.scale_point((*umem.upoint)(unsafe.Add(unsafe.Pointer(p), i*int(umem.point_size()))), k)
	}
}

func main() {
	// &T{...}: a gocpp pointer that owns the object; C borrows it for the call.
	p := &umem.upoint{3, 4}
	umem.scale_point(p, 2)
	fmt.Println(int(p.x), int(p.y), p != nil, int(umem.sum_points(p, 1)))
	p.x = 100
	fmt.Println(int((*p).x), int(umem.sum_points(&umem.upoint{1, 2}, 1)))
	var cpy umem.upoint = *p // a C++ copy
	cpy.y = 0
	fmt.Println(int(p.y), int(cpy.y))
	keep := unsafe.Pointer(&umem.upoint{7, 8}) // the unsafe.Pointer keeps the object alive
	fmt.Println(int((*umem.upoint)(keep).y), int(umem.sum_points((*umem.upoint)(keep), 1)))

	// &v[i] and &s.f: raw addresses into C++ memory.
	v := std.vector[C.int]{5, 6, 7}
	e := &v[1]
	*e = 60
	umem.set_int(&v[2], 70)
	fmt.Println(int(v[0]), int(v[1]), int(v[2]))
	var rec umem.urec
	rec.vals[1] = 9
	pv := &rec.vals[1]
	*pv += 1
	px := &rec.pts[1].x
	*px = 33
	fmt.Println(int(rec.vals[1]), int(rec.pts[1].x), int(rec.pts[1].y))
	sh := Shape{"s", 1}
	pa := &sh.at
	*pa = 12
	fmt.Println(sh.at)
	xs := []C.int{1, 2, 3, 4}
	umem.set_int(&xs[3], 40)
	fmt.Println(xs, int(umem.sum_ints(&xs[0], C.size_t(len(xs)))))

	// A gocpp array of C structs, walked with unsafe.Add through a pointer to the first one.
	pts := [3]umem.upoint{{1, 1}, {2, 2}, {3, 3}}
	scaleAll(&pts[0], len(pts), 10)
	fmt.Println(int(pts[0].x), int(pts[1].y), int(pts[2].x))

	// The old idiom: a pointer to a huge array type, sliced to the length.
	ints := (*[1 << 20]C.int)(unsafe.Pointer(&umem.get_rec().vals[0]))[:4:4]
	fmt.Println(len(ints), cap(ints), int(ints[0]), int(ints[3]))
	ints[3] = 41
	fmt.Println(int(umem.get_rec().vals[3]))

	// A C function's address, as an unsafe.Pointer, converted to its function pointer type.
	fp := unsafe.Pointer(umem.triple)
	f := umem.unary_fn(fp)
	fmt.Println(int(umem.call_unary(f, 5)), f != nil, int(umem.call_unary(umem.unary_fn(unsafe.Pointer(umem.triple)), 6)))
}

// Output:
// 6 8 true 14
// 100 3
// 8 0
// 8 15
// 5 60 70
// 10 33 0
// 12
// [1 2 3 40] 46
// 10 20 30
// 4 4 10 40
// 41
// 15 true 18
