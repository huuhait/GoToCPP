// phase: 08

// Time.Truncate and Time.Round: multiples of d since the zero time (not the Unix epoch), d <= 0, strips the monotonic reading.
package main

import (
	"fmt"
	"time"
)

func main() {
	t := time.Date(2024, 3, 10, 13, 47, 29, 987654321, time.UTC)
	ds := []time.Duration{time.Nanosecond, 100, time.Microsecond, time.Millisecond, 10 * time.Millisecond, time.Second, 5 * time.Second,
		time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour, 7 * 24 * time.Hour, 0, -1, -time.Hour, 7 * time.Millisecond, 1<<63 - 1}
	for _, d := range ds {
		fmt.Println(d, t.Truncate(d).Format("2006-01-02 15:04:05.000000000"), t.Round(d).Format("2006-01-02 15:04:05.000000000"))
	}
	// half rounds up
	h := time.Date(2024, 1, 1, 0, 0, 30, 0, time.UTC)
	fmt.Println(h.Round(time.Minute).Format(time.TimeOnly), h.Add(-1).Round(time.Minute).Format(time.TimeOnly), h.Add(1).Round(time.Minute).Format(time.TimeOnly))
	// the times before the zero time and year 1
	e := time.Date(1, 1, 1, 0, 0, 0, 1, time.UTC)
	fmt.Println(e.Truncate(time.Second).IsZero(), e.Round(time.Second).IsZero(), e.Truncate(time.Hour).IsZero())
	p := time.Date(-5, 6, 7, 8, 9, 10, 11, time.UTC)
	fmt.Println(p.Truncate(time.Hour).Format(time.RFC3339), p.Round(time.Hour).Format(time.RFC3339))
	// zones: truncation is of the absolute time, not of the local wall clock
	z := time.Date(2024, 3, 10, 13, 47, 29, 0, time.FixedZone("X", 3*3600+1800))
	fmt.Println(z.Truncate(time.Hour).Format(time.RFC3339), z.Truncate(24*time.Hour).Format(time.RFC3339))
	// monotonic reading
	n := time.Now()
	fmt.Println(n.Round(0) == n, n.Truncate(time.Hour) == n.Truncate(time.Hour).Round(0), n.Round(0).Equal(n))
	fmt.Println(n.Round(0).String() == n.Round(0).Round(0).String())
}

// Output:
// 1ns 2024-03-10 13:47:29.987654321 2024-03-10 13:47:29.987654321
// 100ns 2024-03-10 13:47:29.987654300 2024-03-10 13:47:29.987654300
// 1µs 2024-03-10 13:47:29.987654000 2024-03-10 13:47:29.987654000
// 1ms 2024-03-10 13:47:29.987000000 2024-03-10 13:47:29.988000000
// 10ms 2024-03-10 13:47:29.980000000 2024-03-10 13:47:29.990000000
// 1s 2024-03-10 13:47:29.000000000 2024-03-10 13:47:30.000000000
// 5s 2024-03-10 13:47:25.000000000 2024-03-10 13:47:30.000000000
// 1m0s 2024-03-10 13:47:00.000000000 2024-03-10 13:47:00.000000000
// 15m0s 2024-03-10 13:45:00.000000000 2024-03-10 13:45:00.000000000
// 1h0m0s 2024-03-10 13:00:00.000000000 2024-03-10 14:00:00.000000000
// 6h0m0s 2024-03-10 12:00:00.000000000 2024-03-10 12:00:00.000000000
// 24h0m0s 2024-03-10 00:00:00.000000000 2024-03-11 00:00:00.000000000
// 168h0m0s 2024-03-04 00:00:00.000000000 2024-03-11 00:00:00.000000000
// 0s 2024-03-10 13:47:29.987654321 2024-03-10 13:47:29.987654321
// -1ns 2024-03-10 13:47:29.987654321 2024-03-10 13:47:29.987654321
// -1h0m0s 2024-03-10 13:47:29.987654321 2024-03-10 13:47:29.987654321
// 7ms 2024-03-10 13:47:29.986000000 2024-03-10 13:47:29.986000000
// 2562047h47m16.854775807s 1754-08-30 22:43:41.128654842 2046-12-09 22:30:57.983430649
// 00:01:00 00:00:00 00:01:00
// true true true
// -0005-06-07T08:00:00Z -0005-06-07T08:00:00Z
// 2024-03-10T13:30:00+03:30 2024-03-10T03:30:00+03:30
// false true true
// true
