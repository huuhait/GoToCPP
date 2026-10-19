// phase: 08
// zoneinfo: required

// LoadLocation with the system's time zone database: offsets, DST transitions, abbreviations, ZoneBounds.
package main

import (
	"fmt"
	"time"
)

func main() {
	ny, err := time.LoadLocation("America/New_York")
	fmt.Println(ny, err)
	be, _ := time.LoadLocation("Europe/Berlin")
	kol, _ := time.LoadLocation("Asia/Kolkata")
	fmt.Println(be, kol)
	// the abbreviations and offsets over a year
	for _, d := range []time.Time{
		time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), time.Date(2024, 3, 9, 12, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC), time.Date(2024, 3, 31, 12, 0, 0, 0, time.UTC),
		time.Date(2024, 7, 4, 12, 0, 0, 0, time.UTC), time.Date(2024, 10, 27, 12, 0, 0, 0, time.UTC),
		time.Date(2024, 11, 3, 12, 0, 0, 0, time.UTC), time.Date(2024, 12, 25, 12, 0, 0, 0, time.UTC),
	} {
		for _, l := range []*time.Location{ny, be, kol} {
			t := d.In(l)
			name, off := t.Zone()
			fmt.Printf("%s %-16s %-5s %6d dst=%v %s\n", d.Format("2006-01-02"), l, name, off, t.IsDST(), t.Format(time.RFC3339))
		}
	}
	// the spring-forward instant in New York: 2024-03-10 07:00 UTC
	for _, off := range []time.Duration{-time.Second, 0, time.Second} {
		t := time.Date(2024, 3, 10, 7, 0, 0, 0, time.UTC).Add(off).In(ny)
		fmt.Println(t.Format("15:04:05 MST -0700"))
	}
	// a wall clock time that does not exist (02:30 on 2024-03-10 in New York) and one that exists twice (01:30 on 2024-11-03)
	skip := time.Date(2024, 3, 10, 2, 30, 0, 0, ny)
	fmt.Println(skip.Format(time.RFC3339), skip.UTC().Format(time.RFC3339))
	twice := time.Date(2024, 11, 3, 1, 30, 0, 0, ny)
	fmt.Println(twice.Format(time.RFC3339), twice.UTC().Format(time.RFC3339))
	// AddDate keeps the wall clock across a DST change, Add does not
	d0 := time.Date(2024, 3, 9, 12, 0, 0, 0, ny)
	fmt.Println(d0.AddDate(0, 0, 1).Format(time.RFC3339), d0.Add(24*time.Hour).Format(time.RFC3339))
	d1 := time.Date(2024, 11, 2, 12, 0, 0, 0, ny)
	fmt.Println(d1.AddDate(0, 0, 1).Format(time.RFC3339), d1.Add(24*time.Hour).Format(time.RFC3339))
	fmt.Println(d1.AddDate(0, 0, 1).Sub(d1), d0.AddDate(0, 0, 1).Sub(d0))
	// ZoneBounds
	b := time.Date(2024, 7, 4, 12, 0, 0, 0, ny)
	s, e := b.ZoneBounds()
	fmt.Println(s.Format(time.RFC3339), e.Format(time.RFC3339))
	// Parse in a location
	p, err := time.ParseInLocation("2006-01-02 15:04", "2024-07-04 09:30", ny)
	fmt.Println(p.Format(time.RFC3339), err, p.Location())
	p, err = time.ParseInLocation("2006-01-02 15:04 MST", "2024-01-04 09:30 EST", ny)
	fmt.Println(p.Format(time.RFC3339), err)
	p, err = time.ParseInLocation("2006-01-02 15:04 MST", "2024-07-04 09:30 EST", ny)
	fmt.Println(p.Format(time.RFC3339), err)
	// the same location twice is the same pointer? No: each LoadLocation makes a new one
	ny2, _ := time.LoadLocation("America/New_York")
	fmt.Println(ny == ny2, ny.String() == ny2.String())
	// UTC and Local names
	u, err := time.LoadLocation("")
	fmt.Println(u, err, u == time.UTC)
	u, err = time.LoadLocation("UTC")
	fmt.Println(u, err, u == time.UTC)
	l, err := time.LoadLocation("Local")
	fmt.Println(err, l == time.Local)
	// errors
	for _, name := range []string{"Foo/Bar", "america/new_york", "../etc/passwd", "/etc/passwd", "a/../b", "America", "America/", "\\x"} {
		_, err := time.LoadLocation(name)
		fmt.Printf("%q: %v\n", name, err)
	}
	// older dates use the local mean time or the standard time of the zone
	old := time.Date(1850, 6, 1, 12, 0, 0, 0, time.UTC).In(ny)
	n, o := old.Zone()
	fmt.Println(n, o, old.Format(time.RFC3339))
	// far future uses the last rule
	far := time.Date(2090, 7, 1, 12, 0, 0, 0, time.UTC).In(ny)
	n, o = far.Zone()
	fmt.Println(n, o)
}

// Output:
// America/New_York <nil>
// Europe/Berlin Asia/Kolkata
// 2024-01-15 America/New_York EST   -18000 dst=false 2024-01-15T07:00:00-05:00
// 2024-01-15 Europe/Berlin    CET     3600 dst=false 2024-01-15T13:00:00+01:00
// 2024-01-15 Asia/Kolkata     IST    19800 dst=false 2024-01-15T17:30:00+05:30
// 2024-03-09 America/New_York EST   -18000 dst=false 2024-03-09T07:00:00-05:00
// 2024-03-09 Europe/Berlin    CET     3600 dst=false 2024-03-09T13:00:00+01:00
// 2024-03-09 Asia/Kolkata     IST    19800 dst=false 2024-03-09T17:30:00+05:30
// 2024-03-10 America/New_York EDT   -14400 dst=true 2024-03-10T08:00:00-04:00
// 2024-03-10 Europe/Berlin    CET     3600 dst=false 2024-03-10T13:00:00+01:00
// 2024-03-10 Asia/Kolkata     IST    19800 dst=false 2024-03-10T17:30:00+05:30
// 2024-03-31 America/New_York EDT   -14400 dst=true 2024-03-31T08:00:00-04:00
// 2024-03-31 Europe/Berlin    CEST    7200 dst=true 2024-03-31T14:00:00+02:00
// 2024-03-31 Asia/Kolkata     IST    19800 dst=false 2024-03-31T17:30:00+05:30
// 2024-07-04 America/New_York EDT   -14400 dst=true 2024-07-04T08:00:00-04:00
// 2024-07-04 Europe/Berlin    CEST    7200 dst=true 2024-07-04T14:00:00+02:00
// 2024-07-04 Asia/Kolkata     IST    19800 dst=false 2024-07-04T17:30:00+05:30
// 2024-10-27 America/New_York EDT   -14400 dst=true 2024-10-27T08:00:00-04:00
// 2024-10-27 Europe/Berlin    CET     3600 dst=false 2024-10-27T13:00:00+01:00
// 2024-10-27 Asia/Kolkata     IST    19800 dst=false 2024-10-27T17:30:00+05:30
// 2024-11-03 America/New_York EST   -18000 dst=false 2024-11-03T07:00:00-05:00
// 2024-11-03 Europe/Berlin    CET     3600 dst=false 2024-11-03T13:00:00+01:00
// 2024-11-03 Asia/Kolkata     IST    19800 dst=false 2024-11-03T17:30:00+05:30
// 2024-12-25 America/New_York EST   -18000 dst=false 2024-12-25T07:00:00-05:00
// 2024-12-25 Europe/Berlin    CET     3600 dst=false 2024-12-25T13:00:00+01:00
// 2024-12-25 Asia/Kolkata     IST    19800 dst=false 2024-12-25T17:30:00+05:30
// 01:59:59 EST -0500
// 03:00:00 EDT -0400
// 03:00:01 EDT -0400
// 2024-03-10T01:30:00-05:00 2024-03-10T06:30:00Z
// 2024-11-03T01:30:00-04:00 2024-11-03T05:30:00Z
// 2024-03-10T12:00:00-04:00 2024-03-10T13:00:00-04:00
// 2024-11-03T12:00:00-05:00 2024-11-03T11:00:00-05:00
// 25h0m0s 23h0m0s
// 2024-03-10T03:00:00-04:00 2024-11-03T01:00:00-05:00
// 2024-07-04T09:30:00-04:00 <nil> America/New_York
// 2024-01-04T09:30:00-05:00 <nil>
// 2024-07-04T10:30:00-04:00 <nil>
// false true
// UTC <nil> true
// UTC <nil> true
// <nil> true
// "Foo/Bar": unknown time zone Foo/Bar
// "america/new_york": unknown time zone america/new_york
// "../etc/passwd": time: invalid location name
// "/etc/passwd": time: invalid location name
// "a/../b": time: invalid location name
// "America": is a directory
// "America/": is a directory
// "\\x": time: invalid location name
// LMT -17762 1850-06-01T07:03:58-04:56
// EDT -14400
