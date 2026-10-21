// phase: 08

// The arguments of a defer or go statement that calls an extern function are evaluated at the
// statement, as for any call (ADR-128): a call in an argument runs once, then, and a variable that
// changes afterwards does not change the argument. show prints tag and value from C, so the
// output says when each argument was computed.
package main

import "time"

//gocpp:extern C
func show(tag, n int32)

//gocpp:extern C
func shown() int32

var calls int32

// next has a side effect: it counts its calls and returns 10 times the count.
func next() int32 {
	calls++
	return calls * 10
}

func main() {
	x := int32(5)

	defer show(1, next()) // next() runs here, once: the argument is 10
	defer show(2, x)      // the argument is 5, not the 100 that x gets below
	x = 100

	show(10, calls) // 1: the defer of show(1, next()) called next() once, not twice

	go show(3, next()+x) // 20 + 100, computed here, before x changes
	x = 7
	for shown() < 2 {
		time.Sleep(time.Millisecond) // wait for the goroutine's line
	}

	show(11, calls) // 2: the go statement called next() once

	for i := int32(0); i < 3; i++ {
		defer show(4, i) // each iteration has its own i
	}

	show(12, x) // 7
}

// Output:
// tag 10: 1
// tag 3: 120
// tag 11: 2
// tag 12: 7
// tag 4: 2
// tag 4: 1
// tag 4: 0
// tag 2: 5
// tag 1: 10
