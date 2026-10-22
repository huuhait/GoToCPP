// phase: 12
// opaque: reject

// The value behind an adapted &x is read when the call starts, after every other argument has been
// evaluated: a call among the arguments that changes the variable is seen by the callee, as in Go.
package main

import (
	"fmt"

	"./adapt.hpp"
)

var shared int64

func assignShared() int64 {
	shared = 100
	return 1
}

func addOne(value *int64) int64 {
	*value += 1
	return *value
}

func main() {
	var first int64 = 1
	var second int64 = 2
	fmt.Println(adapt.show_two(&first, &second, int(addOne(&first))), first, second)

	fmt.Println(adapt.add_to(&shared, int(assignShared())), shared)

	var gauge int64 = 5
	var meter adapt.Meter
	meter.level = 1
	fmt.Println(meter.raise(&gauge, int(addOne(&gauge))), gauge)

	var left int64 = 1
	var right int64 = 1
	fmt.Println(adapt.show_two(&left, &right, int(addOne(&right))), left, right)
}

// Output:
// 222 12 12
// 101 101
// 13 12
// 122 11 12
