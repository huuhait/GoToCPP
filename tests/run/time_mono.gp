// phase: 08

// Monotonic clock readings: Sub and Since use them, Round(0) strips them, Equal ignores them, == does not.
package main

import (
	"fmt"
	"time"
)

func main() {
	t0 := time.Now()
	t1 := t0.Round(0) // the same instant without the monotonic reading
	fmt.Println(t0.Equal(t1), t0 == t1, t0.Sub(t1))
	fmt.Println(t1 == t1.Round(0), t0.Round(0) == t1)
	// Add keeps the reading, and the difference of two readings is exact
	t2 := t0.Add(1500 * time.Millisecond)
	fmt.Println(t2.Sub(t0), t0.Sub(t2), t2.Sub(t1), t1.Sub(t2))
	// UTC, Local and In keep the reading; Truncate and Round(d>0) strip it
	fmt.Println(t0.UTC() == t0.UTC().Round(0), t0.Local() == t0.Local().Round(0), t0.In(time.UTC) == t0.In(time.UTC).Round(0))
	fmt.Println(t0.Truncate(time.Second) == t0.Truncate(time.Second).Round(0), t0.Round(time.Second) == t0.Round(time.Second).Round(0))
	// AddDate strips the reading
	fmt.Println(t0.AddDate(0, 0, 1) == t0.AddDate(0, 0, 1).Round(0), t0.AddDate(0, 0, 0) == t0.Round(0))
	// two Now calls never go backwards
	prev := time.Now()
	ok := true
	for i := 0; i < 1000; i++ {
		n := time.Now()
		if n.Before(prev) || n.Sub(prev) < 0 {
			ok = false
		}
		prev = n
	}
	fmt.Println(ok)
	// a duration measured with the monotonic clock is not affected by the wall clock of the time:
	// a Time built by Date has no reading, so Sub falls back to the wall clock
	d := time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC).Sub(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	fmt.Println(d)
	// mixing: one with and one without
	fmt.Println(t0.Sub(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) > 0, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Sub(t0) < 0)
	// wall clock sanity: Now is between 2020 and 2100
	fmt.Println(t0.Year() >= 2020 && t0.Year() < 2100, t0.Unix() > 1577836800, t0.UnixNano() > 1577836800000000000)
	fmt.Println(t0.Location() == time.Local, t0.Local().Location() == time.Local, t0.UTC().Location() == time.UTC)
	// Since and Until stay positive/negative
	fmt.Println(time.Since(t0) >= 0, time.Until(t0) <= 0, time.Since(t1) >= 0)
	// the process monotonic clock is shared by all Now readings: a reading is not before the process start
	fmt.Println(time.Since(t0) < time.Hour)
}

// Output:
// true false 0s
// true true
// 1.5s -1.5s 1.5s -1.5s
// true true true
// true true
// true true
// true
// 1s
// true true
// true true true
// true true true
// true true true
// true
