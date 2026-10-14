// phase: 02

//go:build amd64

// float32 -> integer conversions at the edges and out of range (amd64
// behavior, like float_to_int.gp).
package main

import "fmt"

func main() {
	zero := float32(0)
	f := float32(-3e10)
	for range 60 {
		fmt.Println(f, int8(f), uint8(f), int16(f), uint16(f), int32(f), uint32(f), int64(f), uint64(f))
		f = f/-1.7 + 0.25
	}
	inf := 1 / zero
	fmt.Println(int32(inf), int64(inf), uint64(inf), uint8(inf))
	fmt.Println(int32(-inf), int64(-inf), uint64(-inf), uint8(-inf))
	nan := zero / zero
	fmt.Println(int32(nan), int64(nan), uint64(nan), uint8(nan), uint16(nan))
	m := float32(3.4028235e38)
	fmt.Println(int64(m), uint64(m), int16(m), uint32(m))
}
