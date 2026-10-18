// phase: 04

// Owning pointers (step 4.10) next to closures and defers (step 4.8): the address of an element of
// a gocpp array of C elements shares the array's box, the raw address of a part of a C++ object
// holds the local that the function's deferred calls still use, a closure captures a pointer from
// &T{...} (and the object lives as long as the closure does), and a deferred call on a pointer
// from &T{...} (a mutex: built in place) keeps the object.
package main

import (
	"fmt"
	"unsafe"

	"<mutex>"
	"<vector>"

	"./umem.hpp"
)

func arrayElement() {
	var a [4]C.int
	defer func() { fmt.Println("array", int(a[2]), int(a[0])) }()
	p := &a[2] // shares a's box, which the closure shares too
	*p = 7
	q := unsafe.Pointer(&a[0])
	*(*C.int)(q) = 3
}

func rawPart() {
	v := std.vector[int]{1, 2, 3}
	defer func() { fmt.Println("vector", v[1], v.size()) }()
	p := &v[1] // a raw address into the C++ object; the deferred call holds v
	*p = 20
	fmt.Println(v[1])
}

func ownedInClosure() func() int {
	n := &umem.upoint{3, 4}
	get := func() int { return int(n.x) + int(n.y) }
	n.x = 30
	return get // n's object lives as long as the closure does
}

func mutexInFunction() {
	m := &std.mutex{}
	m.lock()
	defer m.unlock()
	fmt.Println("locked", m.try_lock() == false)
}

func deferOnOwned() {
	xs := &std.vector[int]{1, 2, 3}
	defer xs.clear()
	defer func() { fmt.Println("owned vector", xs.size()) }()
	xs.push_back(4)
}

func main() {
	arrayElement()
	rawPart()
	f := ownedInClosure()
	fmt.Println("closure", f())
	mutexInFunction()
	m := &std.mutex{}
	m.lock()
	m.unlock()
	fmt.Println("unlocked", m.try_lock())
	m.unlock()
	deferOnOwned()
}

// Output:
// array 7 3
// 20
// vector 20 3
// closure 34
// locked true
// unlocked true
// owned vector 4
