// phase: 02

//go:build amd64

// float64 -> integer conversions at the edges and out of range. Out of range
// is implementation-specific in Go; gocpp copies Go on amd64 (CVTTSD2SI
// gives MIN; the small types go through int32), so this test runs only
// there.
package main

import "fmt"

func value(i int) float64 {
	zero := 0.0
	switch i {
	case 0:
		return 0
	case 1:
		return -0.5
	case 2:
		return 0.999999
	case 3:
		return -1.5
	case 4:
		return 127.9
	case 5:
		return 128
	case 6:
		return -129
	case 7:
		return 255.5
	case 8:
		return 256
	case 9:
		return 300
	case 10:
		return -300
	case 11:
		return 32767.5
	case 12:
		return 65536
	case 13:
		return 70000
	case 14:
		return 2147483647.5
	case 15:
		return 2147483648
	case 16:
		return -2147483649
	case 17:
		return 3e9 + 7
	case 18:
		return 4294967296
	case 19:
		return 5e9
	case 20:
		return 9.223372036854775e18
	case 21:
		return 9223372036854775808
	case 22:
		return -9223372036854775808
	case 23:
		return -9.3e18
	case 24:
		return 1.8446744073709552e19
	case 25:
		return 1.5e19
	case 26:
		return 2e19
	case 27:
		return 1e300
	case 28:
		return -1e300
	case 29:
		return 1 / zero
	case 30:
		return -1 / zero
	case 31:
		return zero / zero
	}
	return -1e-300
}

func main() {
	for i := range 33 {
		f := value(i)
		fmt.Println(f, int8(f), uint8(f), int16(f), uint16(f))
		fmt.Println("  ", int32(f), uint32(f), int64(f), uint64(f), int(f), uint(f), uintptr(f))
	}
}
