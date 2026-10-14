// phase: 02

// Constants: package and function scope, iota in const blocks (skips,
// expressions, repeating the last expression), typed constants, untyped
// big values and untyped floats.
package main

import "fmt"

const (
	Sunday = iota
	Monday
	Tuesday
	_
	Thursday
)

const (
	KB = 1 << (10 * (iota + 1))
	MB
	GB
	TB
)

const big = 1 << 100
const pi = 3.14159265358979323846264338327950288419716939937510582097494459
const name = "gocpp"

const (
	a, b = iota, iota * 10
	c, d
	e, f
)

func main() {
	fmt.Println(Sunday, Monday, Tuesday, Thursday)
	fmt.Println(KB, MB, GB, TB)
	fmt.Println(big>>98, big/(1<<97))
	fmt.Println(pi, float32(pi), pi*2)
	fmt.Println(name, len(name), name+"!")
	fmt.Println(a, b, c, d, e, f)

	const (
		red uint8 = iota * 100
		green
		blue
	)
	fmt.Println(red, green, blue, blue-green)

	const local = 7
	var x int8 = local * 18
	fmt.Println(x, local/2, local/2.0)

	const (
		bit0, mask0 = 1 << iota, 1<<iota - 1
		bit1, mask1
		_, _
		bit3, mask3
	)
	fmt.Println(bit0, mask0, bit1, mask1, bit3, mask3)

	const typed int16 = -32768
	fmt.Println(typed, typed+1)
	const r = 'a'
	fmt.Println(r, r+1)
	const huge = 1e308
	fmt.Println(huge*10/100, 1e-320)
}

// Output:
// 0 1 2 4
// 1024 1048576 1073741824 1099511627776
// 4 8
// 3.141592653589793 3.1415927 6.283185307179586
// gocpp 5 gocpp!
// 0 0 1 10 2 20
// 0 100 200 100
// 126 3 3.5
// 1 0 2 1 8 7
// -32768 -32767
// 97 98
// 1e+307 1e-320
