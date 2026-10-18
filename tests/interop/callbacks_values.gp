// phase: 04

// Funcs that reach C in other shapes (step 4.11): a deferred call of a C function with a callback
// (the callback is evaluated when the defer runs, the pointer used when it fires), a func that
// comes from a field, a method's result or a call, for a C++ API that takes a std::function.
package main

import (
	"fmt"
	"unsafe"

	"./cbcxx.hpp"
	"./cbtable.h"
)

func onEvent(x C.int) { fmt.Println("event", int(x)) }

type holder struct{ f func(C.int) C.int }

func triple(x C.int) C.int { return x * 3 }

func (h *holder) get() func(C.int) C.int { return h.f }

func mk() func(C.int) C.int { return triple }

// same returns the pointer that C gave it: the object lives in the caller, so this is fine, also
// when it is a struct that gocpp made (&T{}), or a pointer into one.
func same(u unsafe.Pointer) unsafe.Pointer { return u }

// retfp returns a C function pointer.
func retfp() cbtable.cb_unary_t { return neg }

func neg(x C.int) C.int { return -x }

// run is a gocpp function with a C function pointer parameter: the func goes in as a callback.
func run(f cbtable.cb_apply_t, x C.int) C.int { return f(nil, x) }

func dbl(user unsafe.Pointer, x C.int) C.int { return x * 2 }

func main() {
	defer cbtable.cb_fire(9)
	defer cbtable.cb_register(onEvent)
	h := &holder{f: triple}
	fmt.Println(cbx.apply(h.f, 1), cbx.apply(h.get(), 2), cbx.apply(mk(), 3))
	fmt.Println(cbx.twice(mk(), 1))

	// A callback may give back the pointer it got, also a gocpp-made struct or a pointer into it.
	ops := &cbtable.cb_ops{}
	fmt.Println(cbtable.cb_ud(same, unsafe.Pointer(ops)) == unsafe.Pointer(ops))
	fmt.Println(cbtable.cb_ud(same, unsafe.Pointer(&ops.user)) == unsafe.Pointer(&ops.user))

	// A callback that returns a C function pointer.
	fmt.Println(cbtable.cb_get(retfp, 5))

	// A gocpp function with a C function pointer parameter, and a slice of them.
	fmt.Println(run(dbl, 4))
	var fs []cbtable.cb_apply_t
	fs = append(fs, dbl, dbl)
	fmt.Println(len(fs), fs[1](nil, 8))
}

// Output:
// 4 7 10
// 9
// true
// true
// -5
// 8
// 2 16
// event 9
