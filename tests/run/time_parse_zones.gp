// phase: 08

// Parse and zones: abbreviations, numeric offsets, ParseInLocation, and the Local zone when it matches.
package main

import (
	"fmt"
	"time"
)

func show(t time.Time, err error) {
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	name, off := t.Zone()
	fmt.Printf("%s zone=%q offset=%d loc=%s\n", t.UTC().Format(time.RFC3339), name, off, t.Location())
}

func main() {
	// without a zone: UTC
	show(time.Parse("2006-01-02 15:04", "2024-06-01 10:00"))
	show(time.Parse(time.RFC3339, "2024-06-01T10:00:00Z"))
	show(time.Parse(time.RFC3339, "2024-06-01T10:00:00+02:00"))
	// an abbreviation of an unknown zone gets offset 0 and the name
	show(time.Parse(time.RFC1123, "Sat, 01 Jun 2024 10:00:00 QDT"))
	show(time.Parse(time.RFC1123, "Sat, 01 Jun 2024 10:00:00 UTC"))
	show(time.Parse(time.RFC1123, "Sat, 01 Jun 2024 10:00:00 GMT+5"))
	show(time.Parse(time.RFC1123, "Sat, 01 Jun 2024 10:00:00 GMT-11"))
	show(time.Parse(time.RFC1123, "Sat, 01 Jun 2024 10:00:00 +03"))
	// ParseInLocation: the default location when there is no zone in the value
	cet := time.FixedZone("CET", 3600)
	show(time.ParseInLocation("2006-01-02 15:04", "2024-06-01 10:00", cet))
	show(time.ParseInLocation("2006-01-02 15:04 MST", "2024-06-01 10:00 CET", cet))
	show(time.ParseInLocation("2006-01-02 15:04 MST", "2024-06-01 10:00 XYZ", cet))
	show(time.ParseInLocation("2006-01-02 15:04 -0700", "2024-06-01 10:00 +0100", cet))
	show(time.ParseInLocation("2006-01-02 15:04 -0700", "2024-06-01 10:00 +0200", cet))
	show(time.ParseInLocation("2006-01-02 15:04", "2024-06-01 10:00", time.UTC))
	show(time.ParseInLocation(time.RFC3339, "2024-06-01T10:00:00Z", cet))
	// the location that matches the offset is used
	show(time.ParseInLocation(time.RFC3339, "2024-06-01T10:00:00+01:00", cet))
	// offsets with seconds
	show(time.Parse("2006-01-02 15:04:05 -07:00:00", "2024-06-01 10:00:00 +05:30:15"))
	show(time.Parse("2006-01-02 15:04:05 Z07:00:00", "2024-06-01 10:00:00 Z"))
	show(time.Parse("2006-01-02 15:04:05 -070000", "2024-06-01 10:00:00 -013045"))
	// the instants are what matter
	a, _ := time.Parse(time.RFC3339, "2024-06-01T10:00:00+02:00")
	b, _ := time.Parse(time.RFC3339, "2024-06-01T08:00:00Z")
	fmt.Println(a.Equal(b), a == b, a.UTC() == b)
}

// Output:
// 2024-06-01T10:00:00Z zone="UTC" offset=0 loc=UTC
// 2024-06-01T10:00:00Z zone="UTC" offset=0 loc=UTC
// 2024-06-01T08:00:00Z zone="" offset=7200 loc=
// 2024-06-01T10:00:00Z zone="QDT" offset=0 loc=QDT
// 2024-06-01T10:00:00Z zone="UTC" offset=0 loc=UTC
// 2024-06-01T10:00:00Z zone="GMT+5" offset=18000 loc=GMT+5
// 2024-06-01T10:00:00Z zone="GMT-11" offset=-39600 loc=GMT-11
// 2024-06-01T10:00:00Z zone="+03" offset=0 loc=+03
// 2024-06-01T09:00:00Z zone="CET" offset=3600 loc=CET
// 2024-06-01T09:00:00Z zone="CET" offset=3600 loc=CET
// 2024-06-01T10:00:00Z zone="XYZ" offset=0 loc=XYZ
// 2024-06-01T09:00:00Z zone="CET" offset=3600 loc=CET
// 2024-06-01T08:00:00Z zone="" offset=7200 loc=
// 2024-06-01T10:00:00Z zone="UTC" offset=0 loc=UTC
// 2024-06-01T10:00:00Z zone="UTC" offset=0 loc=UTC
// 2024-06-01T09:00:00Z zone="CET" offset=3600 loc=CET
// 2024-06-01T04:29:45Z zone="" offset=19815 loc=
// 2024-06-01T10:00:00Z zone="UTC" offset=0 loc=UTC
// 2024-06-01T11:30:45Z zone="" offset=-5445 loc=
// true false true
