// phase: 08

// The zero Time: January 1, year 1, 00:00:00 UTC.
package main

import (
	"fmt"
	"time"
)

func main() {
	var z time.Time
	fmt.Println(z)
	fmt.Println(z.IsZero(), z.Year(), z.Month(), z.Day(), z.Hour(), z.Minute(), z.Second(), z.Nanosecond(), z.Weekday(), z.YearDay())
	fmt.Println(z.Unix(), z.UnixMilli(), z.UnixMicro())
	fmt.Println(z.Location(), z.UTC(), z.Location() == time.UTC)
	name, off := z.Zone()
	fmt.Println(name, off)
	fmt.Println(z.Format(time.RFC3339), z.Format(time.RFC1123), z.Format(time.Kitchen), z.Format(time.ANSIC))
	fmt.Println(z.Add(time.Hour), z.AddDate(1, 1, 1))
	fmt.Println(z.Sub(z), z.Equal(z), z.Before(z), z.After(z), z.Compare(z))
	fmt.Printf("%v|%s|%d|%+v\n", z, z, z.Unix(), z)
	y, w := z.ISOWeek()
	fmt.Println(y, w)
	fmt.Println(z.String(), z.GoString())
	ts := fmt.Sprint(z)
	fmt.Println(len(ts))
	var p *time.Time
	fmt.Println(p == nil)
	p = &z
	fmt.Println(p.IsZero(), p.Year())
	s := struct {
		T time.Time
		D time.Duration
	}{}
	fmt.Printf("%v %+v\n", s, s)
	zt := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	fmt.Println(zt == z, zt.Equal(z))
}

// Output:
// 0001-01-01 00:00:00 +0000 UTC
// true 1 January 1 0 0 0 0 Monday 1
// -62135596800 -62135596800000 -62135596800000000
// UTC 0001-01-01 00:00:00 +0000 UTC true
// UTC 0
// 0001-01-01T00:00:00Z Mon, 01 Jan 0001 00:00:00 UTC 12:00AM Mon Jan  1 00:00:00 0001
// 0001-01-01 01:00:00 +0000 UTC 0002-02-02 00:00:00 +0000 UTC
// 0s true false false 0
// 0001-01-01 00:00:00 +0000 UTC|0001-01-01 00:00:00 +0000 UTC|-62135596800|0001-01-01 00:00:00 +0000 UTC
// 1 1
// 0001-01-01 00:00:00 +0000 UTC time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
// 29
// true
// true 1
// {0001-01-01 00:00:00 +0000 UTC 0s} {T:0001-01-01 00:00:00 +0000 UTC D:0s}
// true true
