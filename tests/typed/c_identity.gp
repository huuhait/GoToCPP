// phase: 12
// opaque: same

// The C number types are Go's fixed-size types: C.int is int32, C.uchar is uint8, C.longlong is int64,
// C.float is float32, C.double is float64, C.bool is bool. A value of one goes where the other is
// wanted without a conversion, in opaque and in typed mode, and the explicit conversion between them is
// legal (a conversion between identical types).
package main

import (
	"fmt"

	"./adapt.hpp"
)

func widen(a C.int, b C.uchar, c C.float, d C.bool) (int32, uint8, float32, bool) {
	return a, b, c, d
}

type switches struct {
	on C.bool
}

func isOn(value C.bool) bool {
	return bool(value)
}

func main() {
	var n32 int32 = 5
	var x C.int = n32
	var y int32 = x
	x += n32
	fmt.Println(x, y, x == 10, y < x)

	var u8 uint8 = 250
	var c C.uchar = u8
	c += 10
	fmt.Println(c, uint8(c), C.uchar(u8))

	var f32 float32 = 1.5
	var cf C.float = f32
	var d64 float64 = 0.25
	var cd C.double = d64
	fmt.Println(cf*2, cd+d64, float32(cf), C.double(d64))

	var big int64 = 1 << 40
	var cl C.longlong = big
	fmt.Println(cl+big, cl == big)

	var flag bool = true
	var cb C.bool = flag
	var back bool = cb
	fmt.Println(cb, back, cb == flag)

	// The explicit conversions of a C.bool: a variable, a field, a parameter, a Go bool.
	panel := switches{on: C.bool(flag)}
	fmt.Println(bool(cb), bool(panel.on), isOn(panel.on), C.bool(flag) == cb)

	// Parameters and results of gocpp functions.
	a, b, ff, dd := widen(n32, u8, f32, flag)
	fmt.Println(a, b, ff, dd)

	// Slices, appends and struct literals.
	xs := []C.int{n32, 2}
	xs = append(xs, n32)
	fmt.Println(xs)

	// A C++ function takes them as they are.
	fmt.Println(adapt.take_int(n32), adapt.take_uchar(u8), adapt.take_float(f32), adapt.take_bool(flag))
	adapt.flip(&flag)
	fmt.Println(flag)
}

// Output:
// 10 5 true true
// 4 4 250
// 3 0.5 1.5 0.25
// 2199023255552 true
// true true true
// true true true true
// 5 250 1.5 true
// [5 2 5]
// 5 250 1.5 true
// false
