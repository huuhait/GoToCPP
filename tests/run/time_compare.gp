// phase: 08

// Before, After, Equal, Compare, IsZero, and == against Equal (the location and the monotonic reading differ).
package main

import (
	"fmt"
	"time"
)

func main() {
	a := time.Date(2024, 5, 6, 7, 8, 9, 10, time.UTC)
	b := a.Add(time.Nanosecond)
	c := a.Add(-time.Nanosecond)
	fmt.Println(a.Before(b), a.After(b), a.Equal(b), a.Compare(b))
	fmt.Println(a.Before(c), a.After(c), a.Equal(c), a.Compare(c))
	fmt.Println(a.Before(a), a.After(a), a.Equal(a), a.Compare(a))
	// the same instant in another zone: Equal, but not ==
	plus5 := time.FixedZone("X", 5*3600)
	a2 := a.In(plus5)
	fmt.Println(a.Equal(a2), a == a2, a.Compare(a2), a2.Format(time.RFC3339Nano))
	// the same instant, UTC both: ==
	a3 := time.Date(2024, 5, 6, 7, 8, 9, 10, time.UTC)
	fmt.Println(a == a3, a.Equal(a3))
	// the same wall clock in two zones: different instants
	w1 := time.Date(2024, 5, 6, 12, 0, 0, 0, time.UTC)
	w2 := time.Date(2024, 5, 6, 12, 0, 0, 0, plus5)
	fmt.Println(w1.Equal(w2), w1.Compare(w2), w1.After(w2), w2.Before(w1))
	// zero values
	var z time.Time
	fmt.Println(z.IsZero(), z.Equal(time.Time{}), z == time.Time{}, a.IsZero())
	fmt.Println(z.Before(a), a.After(z), z.Compare(a), a.Compare(z))
	fmt.Println(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC).IsZero(), time.Date(1, 1, 1, 0, 0, 0, 0, plus5).IsZero(), time.Unix(0, 0).IsZero())
	fmt.Println(time.Date(1, 1, 1, 0, 0, 1, 0, time.UTC).IsZero())
	// the year 1 in a zone ahead of UTC is before the zero time
	zz := time.Date(1, 1, 1, 0, 0, 0, 0, plus5)
	fmt.Println(zz.Before(z), zz.Unix(), zz.Year(), zz.Format(time.RFC3339))
	// with a monotonic reading: Equal and Before use it, == does not ignore it
	n1 := time.Now()
	n2 := n1.Round(0)
	fmt.Println(n1.Equal(n2), n1 == n2, n1.Compare(n2), n2 == n2.Round(0))
	n3 := n1.Add(time.Hour)
	fmt.Println(n1.Before(n3), n3.After(n1), n1.Compare(n3), n3.Compare(n1))
	// sorting by Compare
	ts := []time.Time{a.Add(3), a.Add(1), a.Add(2), a}
	for i := range ts {
		for j := i + 1; j < len(ts); j++ {
			if ts[j].Compare(ts[i]) < 0 {
				ts[i], ts[j] = ts[j], ts[i]
			}
		}
	}
	for _, t := range ts {
		fmt.Print(t.Nanosecond(), ";")
	}
	fmt.Println()
	// map keys: == semantics
	m := map[time.Time]int{a: 1}
	m[a2] = 2
	m[a3] = 3
	fmt.Println(len(m), m[a], m[a2])
	// Min and max times
	fmt.Println(time.Unix(1<<62, 0).After(a), time.Unix(-1<<62, 0).Before(a))
}

// Output:
// true false false -1
// false true false 1
// false false true 0
// true false 0 2024-05-06T12:08:09.00000001+05:00
// true true
// false 1 true true
// true true true false
// true true -1 1
// true false false
// false
// true -62135614800 1 0001-01-01T00:00:00+05:00
// true false 0 true
// true true -1 1
// 10;11;12;13;
// 2 3 2
// true true
