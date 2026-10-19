// phase: 08

// Format then Parse over many times and layouts: the instant (to the precision of the layout) comes back.
package main

import (
	"fmt"
	"time"
)

func main() {
	zones := []*time.Location{time.UTC, time.FixedZone("A", 5*3600+1800), time.FixedZone("B", -(8 * 3600)), time.FixedZone("", 3600)}
	layouts := []struct {
		l    string
		prec time.Duration
	}{
		{time.RFC3339Nano, time.Nanosecond}, {time.RFC3339, time.Second}, {time.RFC1123Z, time.Second}, {time.RubyDate, time.Second},
		{time.DateTime + " -0700", time.Second}, {"2006-01-02T15:04:05.000000Z07:00", time.Microsecond}, {"Jan _2 2006 3:04:05.999PM -07:00", time.Millisecond},
		{"20060102150405.999999999-0700", time.Nanosecond}, {"2006-002 15:04 Z07:00", time.Minute},
		{"01/02/2006 03:04:05.00 pm -07:00", time.Duration(10 * time.Millisecond)},
	}
	bad, total := 0, 0
	seed := uint64(88172645463325252)
	next := func() uint64 {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		return seed
	}
	for i := 0; i < 400; i++ {
		sec := int64(next()%4000000000) - 600000000 // 1951 to 2096
		ns := int(next() % 1000000000)
		for _, z := range zones {
			t := time.Unix(sec, int64(ns)).In(z)
			for _, l := range layouts {
				s := t.Format(l.l)
				p, err := time.Parse(l.l, s)
				total++
				want := t.Truncate(l.prec)
				if err != nil || !p.Equal(want) {
					bad++
				}
			}
		}
	}
	fmt.Println("round trips:", total, "mismatches:", bad)
}

// Output:
// round trips: 16000 mismatches: 0
