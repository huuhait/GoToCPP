// phase: 04

// Constants at known C integer targets retain their exact go/constant value, without a
// float64 intermediate. Runtime conversions of variables still truncate as in Go.
package main

import "fmt"

func main() {
	var a C.longlong = 9007199254740993 + 0i
	var b C.ulonglong = (18446744073709551615 + 0i)
	var lo C.longlong = -9223372036854775808 + 0i
	var hi C.longlong = 9223372036854775807 + 0i
	fmt.Println(int64(a), uint64(b), int64(lo), int64(hi))
	fmt.Println(int64(C.longlong(9007199254740993+0i)), uint64(C.ulonglong(18446744073709551615+0i)), int64(C.longlong(-9223372036854775808+0i)), int64(C.longlong(9223372036854775807+0i)))

	var values [1]C.ulonglong
	values[0] = 18446744073709551615 + 0i
	m := make(map[int]C.longlong)
	m[0] = 9007199254740993 + 0i
	p := &a
	*p = -9223372036854775808 + 0i
	fmt.Println(uint64(values[0]), int64(m[0]), int64(a))
	var real C.double = 1.5 + 0i
	var small C.float = 2 + 0i
	const z complex128 = 3 + 0i
	fmt.Println(float64(real), float32(small), float64(C.double(z)))

	const exact float64 = 9007199254740993 // Go rounds a typed float constant to float64.
	fmt.Println(int64(C.longlong(exact)))
	const integral float64 = 2
	fmt.Println(int(C.int(integral)), int(C.int(int64(3))))
	big := int64(1 << 40)
	frac := 2.5
	byteValue := uint16(200)
	fmt.Println(int(C.int(big)), int(C.int(frac)), int(C.char(byteValue)))
}

// Output:
// 9007199254740993 18446744073709551615 -9223372036854775808 9223372036854775807
// 9007199254740993 18446744073709551615 -9223372036854775808 9223372036854775807
// 18446744073709551615 9007199254740993 -9223372036854775808
// 1.5 2 3
// 9007199254740992
// 2 3
// 0 2 -56
