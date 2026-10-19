// phase: 07

// Conversions between all pairs of numeric types of a type set, constants and variables.
package main

import "fmt"

type Num interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}

type Level uint8
type Celsius float64

func Conv[To, From Num](x From) To { return To(x) }

func line[From Num](x From) {
	fmt.Println(
		fmt.Sprintf("%T", x), x,
		Conv[int8](x), Conv[int16](x), Conv[int32](x), Conv[int64](x),
		Conv[uint8](x), Conv[uint16](x), Conv[uint32](x), Conv[uint64](x),
		Conv[float32](x), Conv[float64](x),
		Conv[Level](x), Conv[Celsius](x),
	)
}

func main() {
	line(int8(-1))
	line(int8(127))
	line(int16(-300))
	line(int32(70000))
	line(int64(-5000000000))
	line(uint8(200))
	line(uint16(65535))
	line(uint32(4000000000))
	line(uint64(1) << 63)
	line(float32(3.75))
	line(float32(-3.75))
	line(float64(1234.5))
	line(float64(-0.5))
	line(Level(7))
	line(Celsius(36.6))
}

// Output:
// int8 -1 -1 -1 -1 -1 255 65535 4294967295 18446744073709551615 -1 -1 255 -1
// int8 127 127 127 127 127 127 127 127 127 127 127 127 127
// int16 -300 -44 -300 -300 -300 212 65236 4294966996 18446744073709551316 -300 -300 212 -300
// int32 70000 112 4464 70000 70000 112 4464 70000 70000 70000 70000 112 70000
// int64 -5000000000 0 3584 -705032704 -5000000000 0 3584 3589934592 18446744068709551616 -5e+09 -5e+09 0 -5e+09
// uint8 200 -56 200 200 200 200 200 200 200 200 200 200 200
// uint16 65535 -1 -1 65535 65535 255 65535 65535 65535 65535 65535 255 65535
// uint32 4000000000 0 10240 -294967296 4000000000 0 10240 4000000000 4000000000 4e+09 4e+09 0 4e+09
// uint64 9223372036854775808 0 0 0 -9223372036854775808 0 0 0 9223372036854775808 9.223372e+18 9.223372036854776e+18 0 9.223372036854776e+18
// float32 3.75 3 3 3 3 3 3 3 3 3.75 3.75 3 3.75
// float32 -3.75 -3 -3 -3 -3 253 65533 4294967293 18446744073709551613 -3.75 -3.75 253 -3.75
// float64 1234.5 -46 1234 1234 1234 210 1234 1234 1234 1234.5 1234.5 210 1234.5
// float64 -0.5 0 0 0 0 0 0 0 0 -0.5 -0.5 0 -0.5
// main.Level 7 7 7 7 7 7 7 7 7 7 7 7 7
// main.Celsius 36.6 36 36 36 36 36 36 36 36 36.6 36.6 36 36.6
