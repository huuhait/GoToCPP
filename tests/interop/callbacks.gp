// phase: 04

// Callbacks (step 4.11): gocpp functions that C calls. A function of the package, or a func
// literal that captures nothing, is a C function pointer (a static trampoline): qsort's and
// bsearch's comparators, the fields of a callback table that C calls later, a callback that C
// keeps. The user-data pointer of a C API carries state: a gocpp pointer as unsafe.Pointer, also
// a closure (unsafe.Pointer(&f)). A callback runs on a thread of C's own too.
package main

import (
	"fmt"
	"unsafe"

	"./cbtable.h"
	"<stdlib.h>"
)

// byValue compares two C.int in the array that qsort sorts: its parameters are the void*
// pointers of C's comparator.
func byValue(a, b unsafe.Pointer) C.int {
	x, y := *(*C.int)(a), *(*C.int)(b)
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}

// addUser adds x to the gocpp int that the user pointer points to.
func addUser(user unsafe.Pointer, x C.int) C.int {
	p := (*int)(user)
	*p += int(x)
	return C.int(*p)
}

func finish(user unsafe.Pointer) {
	*(*string)(user) = "finished"
}

var events []int

func onEvent(x C.int) {
	events = append(events, int(x))
}

// callFunc calls the closure that the user pointer points to.
func callFunc(user unsafe.Pointer, x C.int) C.int {
	f := *(*func(C.int) C.int)(user)
	return f(x)
}

// label is C memory that lives for the whole run: a callback may hand out its address.
var label *C.char = C.CString("pkg label")

func nameOf(x C.int) *C.char { return label }

// A C function pointer type: a variable of it holds a callback, also a package variable.
var handler cbtable.cb_apply_t = addUser

func main() {
	nums := []C.int{5, 2, 9, 1, 7}
	stdlib.qsort(unsafe.Pointer(&nums[0]), C.size_t(len(nums)), 4, byValue)
	fmt.Println(nums)

	// A func literal without captures is a callback too.
	stdlib.qsort(unsafe.Pointer(&nums[0]), C.size_t(len(nums)), 4, func(a, b unsafe.Pointer) C.int {
		return *(*C.int)(b) - *(*C.int)(a)
	})
	fmt.Println(nums)

	stdlib.qsort(unsafe.Pointer(&nums[0]), C.size_t(len(nums)), 4, byValue)
	key := C.int(7)
	hit := stdlib.bsearch(unsafe.Pointer(&key), unsafe.Pointer(&nums[0]), C.size_t(len(nums)), 4, byValue)
	key = 8
	miss := stdlib.bsearch(unsafe.Pointer(&key), unsafe.Pointer(&nums[0]), C.size_t(len(nums)), 4, byValue)
	fmt.Println(hit != nil, *(*C.int)(hit), miss == nil)

	// A callback table: function pointers in the fields of a C struct, called by C later.
	total := 100
	state := "running"
	var ops cbtable.cb_ops
	ops.apply = addUser
	ops.done = finish
	ops.user = unsafe.Pointer(&total)
	fmt.Println(cbtable.cb_apply(&ops, 5), cbtable.cb_apply(&ops, 6), total)
	ops2 := cbtable.cb_ops{apply: addUser, user: unsafe.Pointer(&total)}
	fmt.Println(cbtable.cb_apply(&ops2, 1), ops2.done == nil)
	ops.user = unsafe.Pointer(&state)
	cbtable.cb_done(&ops)
	fmt.Println(state)

	// A variable of a C function pointer type; gocpp calls it like any C function pointer.
	var apply cbtable.cb_apply_t = addUser
	mine := 7
	fmt.Println(apply(unsafe.Pointer(&mine), 3), handler(unsafe.Pointer(&mine), 1), mine)

	// A callback that returns a pointer: it must point to memory that outlives the callback.
	fmt.Println(C.GoString(cbtable.cb_name(nameOf, 1)))

	// A user pointer to a gocpp int, passed straight to the C function that calls back.
	count := 0
	fmt.Println(cbtable.cb_sum(4, addUser, unsafe.Pointer(&count)), count)

	// A callback that C keeps and calls later.
	cbtable.cb_register(onEvent)
	cbtable.cb_fire(3)
	cbtable.cb_fire(4)
	fmt.Println(events)

	// A closure through the user pointer: a func variable's address.
	base := 10
	f := func(x C.int) C.int { return x*2 + C.int(base) }
	fmt.Println(cbtable.cb_sum(4, callFunc, unsafe.Pointer(&f)))

	// The same from a thread that C created.
	fmt.Println(cbtable.cb_in_thread(callFunc, unsafe.Pointer(&f), 5))
	n := 1
	fmt.Println(cbtable.cb_in_thread(addUser, unsafe.Pointer(&n), 41), n)
}

// Output:
// [1 2 5 7 9]
// [9 7 5 2 1]
// true 7 true
// 105 111 111
// 112 true
// finished
// 10 11 11
// pkg label
// 10 6
// [3 4]
// 52
// 20
// 42 42
