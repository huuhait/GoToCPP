// phase: 08

// FixedZone, In, UTC, Local, Zone, Location, and conversions between zones.
package main

import (
	"fmt"
	"time"
)

func main() {
	utc := time.Date(2024, 7, 4, 23, 30, 15, 500, time.UTC)
	zs := []*time.Location{
		time.UTC, time.FixedZone("PST", -8*3600), time.FixedZone("IST", 5*3600+1800), time.FixedZone("", 7200),
		time.FixedZone("NZST", 12*3600), time.FixedZone("LINT", 14*3600), time.FixedZone("AoE", -12*3600),
	}
	for _, z := range zs {
		t := utc.In(z)
		name, off := t.Zone()
		fmt.Printf("%-5q %6d %s %s | %s | %s %s %d:%d:%d\n", name, off, z, t.Location(), t.Format(time.RFC3339Nano), t.Weekday(), t.Month(), t.Hour(), t.Minute(), t.Second())
		fmt.Println(t.Equal(utc), t.Unix() == utc.Unix(), t.UTC() == utc, t.YearDay(), t.Day())
		// back and forth
		back := t.In(time.UTC)
		fmt.Println(back.Equal(utc), back == utc)
	}
	// the same wall clock in different zones
	for _, z := range zs {
		t := time.Date(2024, 7, 4, 12, 0, 0, 0, z)
		fmt.Println(t.UTC().Format("15:04 Jan 2"), t.Format("15:04 MST"))
	}
	// zone and date line
	t := time.Date(2024, 12, 31, 23, 0, 0, 0, time.UTC)
	fmt.Println(t.In(zs[5]).Format("2006-01-02 15:04"), t.In(zs[6]).Format("2006-01-02 15:04"))
	// Local: only what does not depend on the machine
	l := time.Now().Local()
	fmt.Println(l.Location() == time.Local, l.UTC().Location() == time.UTC)
	fmt.Println(time.UTC.String(), time.FixedZone("Foo", 3).String(), (*time.Location)(nil).String())
	// a nil location is UTC for In? No: In panics. UTC() and Location() of a zero value are UTC.
	var z time.Time
	fmt.Println(z.Location(), z.UTC().Location())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		_ = utc.In(nil)
	}()
	// Date with a nil location panics
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		_ = time.Date(2024, 1, 1, 0, 0, 0, 0, nil)
	}()
	// Unix(...).In
	fmt.Println(time.Unix(0, 0).In(zs[1]).Format(time.RFC1123), time.Unix(0, 0).In(zs[2]).Format(time.RFC1123Z))
	// ZoneBounds of a fixed zone
	s, e := utc.In(zs[1]).ZoneBounds()
	fmt.Println(s.IsZero(), e.IsZero())
	fmt.Println(utc.In(zs[1]).IsDST(), utc.IsDST())
}

// Output:
// "UTC"      0 UTC UTC | 2024-07-04T23:30:15.0000005Z | Thursday July 23:30:15
// true true true 186 4
// true true
// "PST" -28800 PST PST | 2024-07-04T15:30:15.0000005-08:00 | Thursday July 15:30:15
// true true true 186 4
// true true
// "IST"  19800 IST IST | 2024-07-05T05:00:15.0000005+05:30 | Friday July 5:0:15
// true true true 187 5
// true true
// ""      7200   | 2024-07-05T01:30:15.0000005+02:00 | Friday July 1:30:15
// true true true 187 5
// true true
// "NZST"  43200 NZST NZST | 2024-07-05T11:30:15.0000005+12:00 | Friday July 11:30:15
// true true true 187 5
// true true
// "LINT"  50400 LINT LINT | 2024-07-05T13:30:15.0000005+14:00 | Friday July 13:30:15
// true true true 187 5
// true true
// "AoE" -43200 AoE AoE | 2024-07-04T11:30:15.0000005-12:00 | Thursday July 11:30:15
// true true true 186 4
// true true
// 12:00 Jul 4 12:00 UTC
// 20:00 Jul 4 12:00 PST
// 06:30 Jul 4 12:00 IST
// 10:00 Jul 4 12:00 +0200
// 00:00 Jul 4 12:00 NZST
// 22:00 Jul 3 12:00 LINT
// 00:00 Jul 5 12:00 AoE
// 2025-01-01 13:00 2024-12-31 11:00
// true true
// UTC Foo UTC
// UTC UTC
// recovered: time: missing Location in call to Time.In
// recovered: time: missing Location in call to Date
// Wed, 31 Dec 1969 16:00:00 PST Thu, 01 Jan 1970 05:30:00 +0530
// true true
// false false
