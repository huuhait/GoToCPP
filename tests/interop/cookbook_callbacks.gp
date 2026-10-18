// phase: 04

package main

import (
	"fmt"
	"unsafe"

	"./cookbook_callbacks.h"
	"<stdlib.h>"
)

// A callback is a function of the package, or a func literal that captures nothing: C gets a
// plain function pointer. Its parameters are the types of the C signature.
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

// State that a callback needs goes through the void* argument that the C API provides: here a
// pointer to a Go variable (borrowed: it must outlive the C call).
func count(user unsafe.Pointer, i C.int) {
	*(*int)(user) += int(i)
}

// A closure can't be a C function pointer, but it can be the user data: keep it in a variable, pass
// its address, and let one named callback call it.
func callFunc(user unsafe.Pointer, i C.int) {
	f := *(*func(C.int))(user)
	f(i)
}

func main() {
	nums := []C.int{5, 2, 9, 1}
	stdlib.qsort(unsafe.Pointer(&nums[0]), C.size_t(len(nums)), 4, byValue)
	fmt.Println(nums)

	total := 0
	cookbook_callbacks.each(5, count, unsafe.Pointer(&total))
	fmt.Println(total)

	seen := []int{}
	f := func(i C.int) { seen = append(seen, int(i)*10) }
	cookbook_callbacks.each(3, callFunc, unsafe.Pointer(&f))
	fmt.Println(seen)
}

// Output:
// [1 2 5 9]
// 10
// [0 10 20]
