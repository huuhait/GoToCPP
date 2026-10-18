// phase: 04

// <chrono>: durations with literals-free construction, duration_cast, arithmetic and comparison
// on durations, and the static members of clocks (`std.chrono.steady_clock.now()` needs `::`,
// found by the kind probe); only differences of clock readings are printed.
package main

import (
	"fmt"

	"<chrono>"
)

func main() {
	d := std.chrono.milliseconds(1500)
	s := std.chrono.duration_cast[std.chrono.seconds](d)
	fmt.Println(int64(d.count()), int64(s.count()))

	total := d + std.chrono.seconds(2)
	fmt.Println(int64(total.count()), total > d, d == std.chrono.milliseconds(1500))

	start := std.chrono.steady_clock.now()
	end := std.chrono.steady_clock.now()
	fmt.Println(end-start >= std.chrono.nanoseconds(0), end >= start)

	h := std.chrono.hours(2)
	fmt.Println(int64(std.chrono.duration_cast[std.chrono.minutes](h).count()))
	fmt.Println(std.chrono.system_clock.now().time_since_epoch().count() > 0)
}

// Output:
// 1500 1
// 3500 true true
// true true
// 120
// true
