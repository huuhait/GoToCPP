// phase: 08

// Time encodings: MarshalText, MarshalJSON, MarshalBinary and their inverses, AppendText.
package main

import (
	"fmt"
	"time"
)

func main() {
	ts := []time.Time{
		time.Date(2024, 3, 7, 5, 9, 4, 12345678, time.UTC),
		time.Date(2024, 3, 7, 5, 9, 4, 0, time.FixedZone("", 3600*5+60*30)),
		time.Date(1999, 12, 31, 23, 59, 59, 999999999, time.FixedZone("X", -3600*8)),
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
	}
	for _, t := range ts {
		b, err := t.MarshalText()
		fmt.Println(string(b), err)
		j, err := t.MarshalJSON()
		fmt.Println(string(j), err)
		bin, err := t.MarshalBinary()
		fmt.Println(len(bin), bin[0], err)
		var t2, t3, t4 time.Time
		fmt.Println(t2.UnmarshalText(b), t3.UnmarshalJSON(j), t4.UnmarshalBinary(bin))
		fmt.Println(t2.Equal(t), t3.Equal(t), t4.Equal(t))
		_, o1 := t.Zone()
		_, o2 := t2.Zone()
		_, o4 := t4.Zone()
		fmt.Println(o1 == o2, o1 == o4)
		a, err := t.AppendText([]byte("t="))
		fmt.Println(string(a), err)
	}
	// errors
	bad := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := bad.MarshalText()
	fmt.Println(err)
	_, err = bad.MarshalJSON()
	fmt.Println(err)
	var t time.Time
	fmt.Println(t.UnmarshalText([]byte("not a time")))
	fmt.Println(t.UnmarshalJSON([]byte(`"2024-13-01T00:00:00Z"`)))
	fmt.Println(t.UnmarshalJSON([]byte(`2024`)))
	fmt.Println(t.UnmarshalJSON([]byte(`null`)), t.IsZero())
	fmt.Println(t.UnmarshalBinary([]byte{1, 2}))
	fmt.Println(t.UnmarshalBinary([]byte{9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}))
}

// Output:
// 2024-03-07T05:09:04.012345678Z <nil>
// "2024-03-07T05:09:04.012345678Z" <nil>
// 15 1 <nil>
// <nil> <nil> <nil>
// true true true
// true true
// t=2024-03-07T05:09:04.012345678Z <nil>
// 2024-03-07T05:09:04+05:30 <nil>
// "2024-03-07T05:09:04+05:30" <nil>
// 15 1 <nil>
// <nil> <nil> <nil>
// true true true
// true true
// t=2024-03-07T05:09:04+05:30 <nil>
// 1999-12-31T23:59:59.999999999-08:00 <nil>
// "1999-12-31T23:59:59.999999999-08:00" <nil>
// 15 1 <nil>
// <nil> <nil> <nil>
// true true true
// true true
// t=1999-12-31T23:59:59.999999999-08:00 <nil>
// 0001-01-01T00:00:00Z <nil>
// "0001-01-01T00:00:00Z" <nil>
// 15 1 <nil>
// <nil> <nil> <nil>
// true true true
// true true
// t=0001-01-01T00:00:00Z <nil>
// 9999-12-31T23:59:59Z <nil>
// "9999-12-31T23:59:59Z" <nil>
// 15 1 <nil>
// <nil> <nil> <nil>
// true true true
// true true
// t=9999-12-31T23:59:59Z <nil>
// Time.MarshalText: year outside of range [0,9999]
// Time.MarshalJSON: year outside of range [0,9999]
// parsing time "not a time" as "2006-01-02T15:04:05Z07:00": cannot parse "not a time" as "2006"
// parsing time "2024-13-01T00:00:00Z": month out of range
// Time.UnmarshalJSON: input is not a JSON string
// <nil> true
// Time.UnmarshalBinary: invalid length
// Time.UnmarshalBinary: unsupported version
