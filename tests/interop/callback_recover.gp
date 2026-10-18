// phase: 05

// recover() inside a callback (the open item of step 4.11, possible since step 5.10): a deferred
// function of the callback itself recovers the panic, the callback returns normally to C (here
// with a named result that the deferred function sets), and C goes on with the next element. The
// callback's panic never unwinds through the C frames of cb_sum. A C++ exception thrown by a C++
// function that the callback calls is a panic of the callback too, and recoverable the same way.
package main

import (
	"fmt"
	"unsafe"

	"./cbtable.h"
	"<stdexcept>"
	"<string>"
)

func visit(user unsafe.Pointer, x C.int) (r C.int) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Println("recovered at", int(x), p)
			r = -100
		}
	}()
	if x == 2 {
		panic("stop at 2")
	}
	return x
}

func visitCxx(user unsafe.Pointer, x C.int) (r C.int) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Println("recovered:", p)
			r = 1000
		}
	}()
	return C.int(int(std.stoi(std.string("n"))) + int(x)) // throws std::invalid_argument
}

func main() {
	fmt.Println(int(cbtable.cb_sum(4, visit, nil)))
	fmt.Println(int(cbtable.cb_sum(2, visitCxx, nil)))
}

// Output:
// recovered at 2 stop at 2
// -96
// recovered: C++ exception std::invalid_argument: stoi
// recovered: C++ exception std::invalid_argument: stoi
// 2000
