// phase: 08

// Duration.String: Go's format (units h, m, s, ms, µs, ns; fractions without trailing zeros).
package main

import (
	"fmt"
	"time"
)

const (
	maxInt64 = 1<<63 - 1
	minInt64 = -1 << 63
)

func main() {
	ds := []time.Duration{
		0, 1, -1, 999, 1000, 1001, 1500, 999999, 1000000, 1000001,
		time.Millisecond, 1500 * time.Microsecond, 999 * time.Millisecond,
		time.Second, time.Second + time.Nanosecond, 1500 * time.Millisecond,
		59*time.Second + 999*time.Millisecond, time.Minute, time.Minute + time.Second,
		90 * time.Second, time.Hour, time.Hour + time.Nanosecond,
		time.Hour + 2*time.Minute + 3*time.Second + 500*time.Millisecond,
		100 * time.Hour, 24 * time.Hour, 1000000 * time.Hour,
		-time.Second, -1500 * time.Millisecond, -time.Hour - 30*time.Minute,
		-90 * time.Minute, -999, -1000,
		maxInt64, minInt64, maxInt64 - 1, minInt64 + 1,
		time.Duration(1<<62), time.Duration(123456789012345678),
	}
	for _, d := range ds {
		fmt.Printf("%d %s\n", int64(d), d.String())
	}
	var z time.Duration
	fmt.Println(z, z.String() == "0s")
}

// Output:
// 0 0s
// 1 1ns
// -1 -1ns
// 999 999ns
// 1000 1µs
// 1001 1.001µs
// 1500 1.5µs
// 999999 999.999µs
// 1000000 1ms
// 1000001 1.000001ms
// 1000000 1ms
// 1500000 1.5ms
// 999000000 999ms
// 1000000000 1s
// 1000000001 1.000000001s
// 1500000000 1.5s
// 59999000000 59.999s
// 60000000000 1m0s
// 61000000000 1m1s
// 90000000000 1m30s
// 3600000000000 1h0m0s
// 3600000000001 1h0m0.000000001s
// 3723500000000 1h2m3.5s
// 360000000000000 100h0m0s
// 86400000000000 24h0m0s
// 3600000000000000000 1000000h0m0s
// -1000000000 -1s
// -1500000000 -1.5s
// -5400000000000 -1h30m0s
// -5400000000000 -1h30m0s
// -999 -999ns
// -1000 -1µs
// 9223372036854775807 2562047h47m16.854775807s
// -9223372036854775808 -2562047h47m16.854775808s
// 9223372036854775806 2562047h47m16.854775806s
// -9223372036854775807 -2562047h47m16.854775807s
// 4611686018427387904 1281023h53m38.427387904s
// 123456789012345678 34293h33m9.012345678s
// 0s true
