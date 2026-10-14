// goarch: amd64
//
// Float to integer conversions with gp::f2i: in range they truncate toward
// zero; out of range and for NaN and ±Inf, Go's result depends on the CPU, and
// gocpp gives amd64's (Phase 02 README §2.3). So this program prints what Go
// prints only on amd64.
package main

import "fmt"

func conv(f float64) {
	fmt.Println(f, int(f), int8(f), int16(f), int32(f), int64(f))
	fmt.Println(" ", uint(f), uint8(f), uint16(f), uint32(f), uint64(f), uintptr(f))
}

func conv32(f float32) {
	fmt.Println(f, int(f), int8(f), int16(f), int32(f), int64(f))
	fmt.Println(" ", uint(f), uint8(f), uint16(f), uint32(f), uint64(f), uintptr(f))
}

func main() {
	zero := 0.0
	conv(0.0)
	conv(1.9)
	conv(-1.9)
	conv(-0.5)
	conv(127.5)
	conv(128.0)
	conv(-129.0)
	conv(255.9)
	conv(300.0)
	conv(-1.0)
	conv(65535.5)
	conv(65536.0)
	conv(3e9 + 7)
	conv(-2147483649.0)
	conv(2147483647.9)
	conv(4294967296.5)
	conv(9.3e18)
	conv(1e19)
	conv(1.8e19)
	conv(18446744073709551615.0)
	conv(36893488147419103232.0)
	conv(9223372036854775807.0)
	conv(-9223372036854775808.0)
	conv(1e300)
	conv(-1e300)
	conv(1 / zero)
	conv(-1 / zero)
	conv(zero / zero)
	conv32(1e10)
	conv32(-3.5)
	conv32(70000.75)
	conv32(1.5e19)
	conv32(1 / float32(zero))
	conv32(float32(zero / zero))
}
