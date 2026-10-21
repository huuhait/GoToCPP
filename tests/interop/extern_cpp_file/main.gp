// phase: 04

// C++ functions that no header declares (ADR-128), defined in shapes.cpp: the default linkage is
// C++, so the declaration has the parameter types in its mangled name, and a namespace is written
// in the name. A parameter that is a pointer to const is spelled std.add_const_t[T] (the type
// traits header); an overload is a second Go name for the same C++ name, and can be deferred.
package main

import (
	"fmt"
	"unsafe"

	"<type_traits>"
)

//gocpp:extern
func total(values *std.add_const_t[C.int], n C.int) C.int

//gocpp:extern shapes::area
func area(width, height float64) float64

//gocpp:extern shapes::count
func countInt(n C.int) C.int

//gocpp:extern shapes::count
func countDouble(x C.double) C.int

//gocpp:extern shapes::inner::name
func innerName() *std.add_const_t[C.char]

//gocpp:extern C
func shapes_fill(buffer unsafe.Pointer, n C.size_t, out *int32) bool

func main() {
	defer countInt(1) // an overloaded name: the deferred call is made by a lambda, which picks the overload
	defer fmt.Println("done")
	values := [3]C.int{1, 2, 3}
	fmt.Println(total(&values[0], 3))
	fmt.Println(area(2, 3.5))
	fmt.Println(countInt(5), countDouble(5.9))
	fmt.Println(C.GoString(innerName()))

	var buffer [4]byte
	var out int32
	ok := shapes_fill(unsafe.Pointer(&buffer[0]), 4, &out)
	fmt.Println(ok, buffer, out)
}

// Output:
// 6
// 7
// 5 105
// inner
// true [1 2 3 4] 42
// done
