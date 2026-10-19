// phase: 08

// Time.String and GoString: the default format, with the monotonic suffix only for a time from Now.
package main

import (
	"fmt"
	"time"
)

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func contains(s, sub string) bool { return index(s, sub) >= 0 }

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

func count(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func main() {
	t := time.Date(2024, 3, 7, 5, 9, 4, 12345678, time.FixedZone("CET", 3600))
	fmt.Println(t.String())
	fmt.Println(t)
	fmt.Printf("%v|%s\n", t, t)
	fmt.Printf("%+v\n", t)
	fmt.Println(t.GoString())
	fmt.Printf("%#v\n", t)
	fmt.Println(time.Date(2024, 3, 7, 5, 9, 4, 0, time.UTC))
	fmt.Println(time.Date(2024, 3, 7, 5, 9, 4, 500, time.UTC).GoString())
	fmt.Println(time.Date(2024, 3, 7, 5, 9, 4, 0, time.FixedZone("", 3600*5+60*30)))
	fmt.Println(time.Date(-1, 3, 7, 5, 9, 4, 0, time.UTC))
	fmt.Println(time.Date(10000, 3, 7, 5, 9, 4, 0, time.UTC))
	fmt.Println(time.Unix(1700000000, 123).UTC())
	// a time from Now has a monotonic reading in String
	n := time.Now()
	s := n.String()
	i := index(s, " m=")
	fmt.Println(i > 0, hasPrefix(s[i:], " m=+0."), count(s, "m="))
	fmt.Println(contains(n.Round(0).String(), "m="), contains(n.UTC().String(), "m="), contains(n.Add(time.Second).String(), "m="))
	fmt.Println(contains(n.In(time.UTC).String(), "m="), contains(n.Truncate(time.Second).String(), "m="))
	// the layout of Time.String is "2006-01-02 15:04:05.999999999 -0700 MST"
	fmt.Println(t.Format("2006-01-02 15:04:05.999999999 -0700 MST") == t.String())
	// m=+... is seconds with nine decimals
	j := index(s, "m=+")
	rest := s[j+3:]
	k := index(rest, ".")
	fmt.Println(len(rest)-k-1)
	// the stringer in a slice and a struct
	fmt.Println([]time.Time{t, t.UTC()})
	fmt.Printf("%v\n", struct{ T time.Time }{t})
	var ip fmt.Stringer = t
	fmt.Println(ip)
}

// Output:
// 2024-03-07 05:09:04.012345678 +0100 CET
// 2024-03-07 05:09:04.012345678 +0100 CET
// 2024-03-07 05:09:04.012345678 +0100 CET|2024-03-07 05:09:04.012345678 +0100 CET
// 2024-03-07 05:09:04.012345678 +0100 CET
// time.Date(2024, time.March, 7, 5, 9, 4, 12345678, time.Location("CET"))
// time.Date(2024, time.March, 7, 5, 9, 4, 12345678, time.Location("CET"))
// 2024-03-07 05:09:04 +0000 UTC
// time.Date(2024, time.March, 7, 5, 9, 4, 500, time.UTC)
// 2024-03-07 05:09:04 +0530 +0530
// -0001-03-07 05:09:04 +0000 UTC
// 10000-03-07 05:09:04 +0000 UTC
// 2023-11-14 22:13:20.000000123 +0000 UTC
// true true 1
// false false true
// false false
// true
// 9
// [2024-03-07 05:09:04.012345678 +0100 CET 2024-03-07 04:09:04.012345678 +0000 UTC]
// {2024-03-07 05:09:04.012345678 +0100 CET}
// 2024-03-07 05:09:04.012345678 +0100 CET
