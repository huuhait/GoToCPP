// phase: 08

// Time.Sub, Until and Since arithmetic: saturation at the Duration limits, Add with overflow.
package main

import (
	"fmt"
	"time"
)

func main() {
	a := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, d := range []time.Duration{0, 1, -1, time.Second, time.Hour, 24 * time.Hour, 365 * 24 * time.Hour, 100 * 365 * 24 * time.Hour,
		1<<63 - 1, -1 << 63} {
		b := a.Add(d)
		fmt.Println(d, b.Sub(a), a.Sub(b))
	}
	// saturation
	far := time.Date(2400, 1, 1, 0, 0, 0, 0, time.UTC)
	past := time.Date(1600, 1, 1, 0, 0, 0, 0, time.UTC)
	fmt.Println(far.Sub(past), past.Sub(far), far.Sub(past) == 1<<63-1, past.Sub(far) == -1<<63)
	var z time.Time
	fmt.Println(a.Sub(z), z.Sub(a), a.Sub(z) == 1<<63-1)
	// subtraction across zones
	x := time.Date(2024, 3, 10, 12, 0, 0, 0, time.FixedZone("A", 3600))
	y := time.Date(2024, 3, 10, 12, 0, 0, 0, time.FixedZone("B", -3600))
	fmt.Println(x.Sub(y), y.Sub(x))
	// Since and Until with the clock: only the sign is checked
	fmt.Println(time.Since(time.Now().Add(-time.Hour)) >= time.Hour, time.Until(time.Now().Add(time.Hour)) > 59*time.Minute)
	fmt.Println(time.Since(time.Now()) >= 0, time.Since(time.Now()) < time.Minute)
	fmt.Println(time.Until(time.Now().Add(-time.Hour)) < 0)
	// a time with a monotonic reading uses it
	t0 := time.Now()
	t1 := t0.Add(5 * time.Second)
	fmt.Println(t1.Sub(t0), t0.Sub(t1))
	// Add with a huge duration moves the date and drops the monotonic reading's meaning
	big := t0.Add(1<<62 - 1)
	fmt.Println(big.After(t0), big.Sub(t0) == 1<<62-1)
	fmt.Println(a.Add(-1<<63).Year() < 0, a.Add(1<<63-1).Year())
	// Sub is exact at nanosecond level
	n1 := time.Date(2024, 1, 1, 0, 0, 0, 999999999, time.UTC)
	n2 := time.Date(2024, 1, 1, 0, 0, 1, 1, time.UTC)
	fmt.Println(n2.Sub(n1), n1.Sub(n2))
}

// Output:
// 0s 0s 0s
// 1ns 1ns -1ns
// -1ns -1ns 1ns
// 1s 1s -1s
// 1h0m0s 1h0m0s -1h0m0s
// 24h0m0s 24h0m0s -24h0m0s
// 8760h0m0s 8760h0m0s -8760h0m0s
// 876000h0m0s 876000h0m0s -876000h0m0s
// 2562047h47m16.854775807s 2562047h47m16.854775807s -2562047h47m16.854775807s
// -2562047h47m16.854775808s -2562047h47m16.854775808s 2562047h47m16.854775807s
// 2562047h47m16.854775807s -2562047h47m16.854775808s true true
// 2562047h47m16.854775807s -2562047h47m16.854775808s true
// -2h0m0s 2h0m0s
// true true
// true true
// true
// 5s -5s
// true true
// false 2316
// 2ns -2ns
