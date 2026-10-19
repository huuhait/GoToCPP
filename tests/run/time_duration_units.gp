// phase: 08

// Duration unit conversions: Seconds, Minutes, Hours, Milliseconds, Microseconds, Nanoseconds, Abs.
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
	fmt.Println(int64(time.Nanosecond), int64(time.Microsecond), int64(time.Millisecond), int64(time.Second), int64(time.Minute), int64(time.Hour))
	ds := []time.Duration{0, 1, 1500, 1500000, 2500 * time.Millisecond, 90 * time.Minute, 36 * time.Hour,
		-2500 * time.Millisecond, -90 * time.Minute, 1<<63 - 1, -1 << 63, 123456789}
	for _, d := range ds {
		fmt.Println(d, d.Nanoseconds(), d.Microseconds(), d.Milliseconds(), d.Seconds(), d.Minutes(), d.Hours())
	}
	for _, d := range []time.Duration{5, -5, 0, maxInt64, minInt64, minInt64 + 1} {
		fmt.Println(d.Abs(), int64(d.Abs()))
	}
	fmt.Printf("%.3f %.6f\n", (3500 * time.Millisecond).Seconds(), (45 * time.Second).Minutes())
	fmt.Println(time.Duration(1.5*float64(time.Second)), time.Duration(2)*time.Hour)
}

// Output:
// 1 1000 1000000 1000000000 60000000000 3600000000000
// 0s 0 0 0 0 0 0
// 1ns 1 0 0 1e-09 1.6666666666666667e-11 2.777777777777778e-13
// 1.5µs 1500 1 0 1.5e-06 2.5e-08 4.166666666666667e-10
// 1.5ms 1500000 1500 1 0.0015 2.5e-05 4.1666666666666667e-07
// 2.5s 2500000000 2500000 2500 2.5 0.041666666666666664 0.0006944444444444445
// 1h30m0s 5400000000000 5400000000 5400000 5400 90 1.5
// 36h0m0s 129600000000000 129600000000 129600000 129600 2160 36
// -2.5s -2500000000 -2500000 -2500 -2.5 -0.041666666666666664 -0.0006944444444444445
// -1h30m0s -5400000000000 -5400000000 -5400000 -5400 -90 -1.5
// 2562047h47m16.854775807s 9223372036854775807 9223372036854775 9223372036854 9.223372036854776e+09 1.5372286728091294e+08 2.5620477880152157e+06
// -2562047h47m16.854775808s -9223372036854775808 -9223372036854775 -9223372036854 -9.223372036854776e+09 -1.5372286728091294e+08 -2.5620477880152157e+06
// 123.456789ms 123456789 123456 123 0.123456789 0.00205761315 3.42935525e-05
// 5ns 5
// 5ns 5
// 0s 0
// 2562047h47m16.854775807s 9223372036854775807
// 2562047h47m16.854775807s 9223372036854775807
// 2562047h47m16.854775807s 9223372036854775807
// 3.500 0.750000
// 1.5s 2h0m0s
