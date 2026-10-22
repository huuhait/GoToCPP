// phase: 12
// opaque: reject

// The address of a variable of a Go number type goes to a pointer to another C number type: the
// compiler passes a temporary of the C type, filled from the variable and copied back after the call
// when the callee may write (a non-const pointer); a value that does not fit stops the program.
package main

import (
	"fmt"

	"./adapt.hpp"
)

type counter struct {
	hits int
}

func main() {
	// A Go int is 64 bits, the parameter an int*: the value goes in and comes back.
	n := 41
	fmt.Println(adapt.bump(&n), n)

	// A field is a variable too.
	c := counter{hits: 9}
	fmt.Println(adapt.bump(&c.hits), c.hits)

	// An int64 is a long on this target, the parameter a long long*: same width, another type.
	var big int64
	adapt.set_big(&big)
	fmt.Println(big)

	// A const pointer is only read: a narrower or a wider integer goes in.
	var small int16 = -3
	fmt.Println(adapt.read_int(&small))
	var wide uint64 = 12
	fmt.Println(adapt.read_int(&wide))

	// A float32 widens to a const double*.
	var f float32 = 5
	fmt.Println(adapt.halve(&f))

	// A uint64 is as wide as unsigned long here.
	var u uint64 = 6
	fmt.Println(adapt.add_one(&u), u)

	// A short is written; an int can hold it.
	var s int
	adapt.store_short(&s)
	fmt.Println(s)

	// A value that does not fit the parameter's type is a run-time panic, with the variable's name.
	func() {
		defer func() { fmt.Println(recover()) }()
		var huge int = 1 << 40
		fmt.Println(adapt.read_int(&huge))
	}()
}

// Output:
// 42 42
// 10 10
// 1099511627776
// -3
// 12
// 2.5
// 7 7
// 12
// runtime error: the variable huge holds 1099511627776, which does not fit the C type int of the parameter it is passed to
