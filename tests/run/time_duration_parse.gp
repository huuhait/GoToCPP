// phase: 08

// time.ParseDuration: valid forms, signs, fractions, and Go's exact error messages.
package main

import (
	"fmt"
	"time"
)

func main() {
	inputs := []string{
		"0", "+0", "-0", "0s", "1ns", "1us", "1µs", "1μs", "1ms", "1s", "1m", "1h",
		"1.5s", "1.5h", ".5s", "5.s", "-1.5h", "+1.5h", "1h2m3s", "1h2m3.5s", "1h2m3s4ms5us6ns",
		"100000h", "2562047h47m16.854775807s", "2562047h47m16.854775808s", "-2562047h47m16.854775808s",
		"-2562047h47m16.854775809s", "9223372036854775807ns", "9223372036854775808ns",
		"0.000000001s", "0.0000000001s", "1.0000000001ns", "0.9999999999999999999ns", "3000000h",
		"1e3s", "1 s", " 1s", "1s ", "s", "", "+", "-", ".", "1", "100", "1x", "1.5", "h1", "1.s5", "--1s",
		"1h-1m", "1h1", "0x10s", "99999999999999999999s", "1.99999999999999999999999999s", "0.0ns", "00s",
		"-.s", "1d", "1w", "1y", "\xff", "1µs", "1\xb5s",
	}
	for _, s := range inputs {
		d, err := time.ParseDuration(s)
		fmt.Printf("%q: %d %v", s, int64(d), err)
		fmt.Println()
	}
}

// Output:
// "0": 0 <nil>
// "+0": 0 <nil>
// "-0": 0 <nil>
// "0s": 0 <nil>
// "1ns": 1 <nil>
// "1us": 1000 <nil>
// "1µs": 1000 <nil>
// "1μs": 1000 <nil>
// "1ms": 1000000 <nil>
// "1s": 1000000000 <nil>
// "1m": 60000000000 <nil>
// "1h": 3600000000000 <nil>
// "1.5s": 1500000000 <nil>
// "1.5h": 5400000000000 <nil>
// ".5s": 500000000 <nil>
// "5.s": 5000000000 <nil>
// "-1.5h": -5400000000000 <nil>
// "+1.5h": 5400000000000 <nil>
// "1h2m3s": 3723000000000 <nil>
// "1h2m3.5s": 3723500000000 <nil>
// "1h2m3s4ms5us6ns": 3723004005006 <nil>
// "100000h": 360000000000000000 <nil>
// "2562047h47m16.854775807s": 9223372036854775807 <nil>
// "2562047h47m16.854775808s": 0 time: invalid duration "2562047h47m16.854775808s"
// "-2562047h47m16.854775808s": -9223372036854775808 <nil>
// "-2562047h47m16.854775809s": 0 time: invalid duration "-2562047h47m16.854775809s"
// "9223372036854775807ns": 9223372036854775807 <nil>
// "9223372036854775808ns": 0 time: invalid duration "9223372036854775808ns"
// "0.000000001s": 1 <nil>
// "0.0000000001s": 0 <nil>
// "1.0000000001ns": 1 <nil>
// "0.9999999999999999999ns": 1 <nil>
// "3000000h": 0 time: invalid duration "3000000h"
// "1e3s": 0 time: unknown unit "e" in duration "1e3s"
// "1 s": 0 time: unknown unit " s" in duration "1 s"
// " 1s": 0 time: invalid duration " 1s"
// "1s ": 0 time: unknown unit "s " in duration "1s "
// "s": 0 time: invalid duration "s"
// "": 0 time: invalid duration ""
// "+": 0 time: invalid duration "+"
// "-": 0 time: invalid duration "-"
// ".": 0 time: invalid duration "."
// "1": 0 time: missing unit in duration "1"
// "100": 0 time: missing unit in duration "100"
// "1x": 0 time: unknown unit "x" in duration "1x"
// "1.5": 0 time: missing unit in duration "1.5"
// "h1": 0 time: invalid duration "h1"
// "1.s5": 0 time: missing unit in duration "1.s5"
// "--1s": 0 time: invalid duration "--1s"
// "1h-1m": 0 time: unknown unit "h-" in duration "1h-1m"
// "1h1": 0 time: missing unit in duration "1h1"
// "0x10s": 0 time: unknown unit "x" in duration "0x10s"
// "99999999999999999999s": 0 time: invalid duration "99999999999999999999s"
// "1.99999999999999999999999999s": 2000000000 <nil>
// "0.0ns": 0 <nil>
// "00s": 0 <nil>
// "-.s": 0 time: invalid duration "-.s"
// "1d": 0 time: unknown unit "d" in duration "1d"
// "1w": 0 time: unknown unit "w" in duration "1w"
// "1y": 0 time: unknown unit "y" in duration "1y"
// "\xff": 0 time: invalid duration "\xff"
// "1µs": 1000 <nil>
// "1\xb5s": 0 time: unknown unit "\xb5s" in duration "1\xb5s"
