// phase: 08

// A Time and a Location through fmt: String is used for %v and %s, %#v is GoString, %d prints the fields.
package main

import (
	"fmt"
	"time"
)

type event struct {
	When time.Time
	Dur  time.Duration
	Loc  *time.Location
}

func main() {
	t := time.Date(2024, 3, 7, 5, 9, 4, 12345678, time.UTC)
	fmt.Printf("%v\n%s\n%q\n%+v\n", t, t, t, t)
	fmt.Printf("%#v\n", t)
	fmt.Printf("%T %T %T %T\n", t, time.UTC, time.March, time.Second)
	fmt.Printf("%30v|%-30v|\n", t, t)
	var z time.Time
	fmt.Printf("%d %v\n", z, z)
	e := event{t, time.Minute, time.UTC}
	fmt.Printf("%v\n%+v\n", e, e)
	fmt.Println(e)
	fmt.Println(time.UTC, time.FixedZone("EST", -18000), time.UTC.String())
	fmt.Printf("%v %s %q\n", time.UTC, time.FixedZone("X", 1), time.FixedZone("Y", 2))
	m := map[string]time.Time{"a": t}
	fmt.Println(m)
	var s fmt.Stringer = t
	fmt.Println(s.String() == t.String())
	var iface any = t
	switch v := iface.(type) {
	case fmt.Stringer:
		fmt.Println("stringer", v)
	}
	if tt, ok := iface.(time.Time); ok {
		fmt.Println(tt.Equal(t))
	}
	fmt.Println(fmt.Sprint(t), fmt.Sprintln(t) == fmt.Sprint(t)+"\n")
	ts := []time.Time{t, t.Add(time.Hour)}
	fmt.Println(ts)
	var np *time.Time
	fmt.Println(np)
	np = &t
	fmt.Println(np.String(), *np)
}

// Output:
// 2024-03-07 05:09:04.012345678 +0000 UTC
// 2024-03-07 05:09:04.012345678 +0000 UTC
// "2024-03-07 05:09:04.012345678 +0000 UTC"
// 2024-03-07 05:09:04.012345678 +0000 UTC
// time.Date(2024, time.March, 7, 5, 9, 4, 12345678, time.UTC)
// time.Time *time.Location time.Month time.Duration
// 2024-03-07 05:09:04.012345678 +0000 UTC|2024-03-07 05:09:04.012345678 +0000 UTC|
// {0 0 0} 0001-01-01 00:00:00 +0000 UTC
// {2024-03-07 05:09:04.012345678 +0000 UTC 1m0s UTC}
// {When:2024-03-07 05:09:04.012345678 +0000 UTC Dur:1m0s Loc:UTC}
// {2024-03-07 05:09:04.012345678 +0000 UTC 1m0s UTC}
// UTC EST UTC
// UTC X "Y"
// map[a:2024-03-07 05:09:04.012345678 +0000 UTC]
// true
// stringer 2024-03-07 05:09:04.012345678 +0000 UTC
// true
// 2024-03-07 05:09:04.012345678 +0000 UTC true
// [2024-03-07 05:09:04.012345678 +0000 UTC 2024-03-07 06:09:04.012345678 +0000 UTC]
// <nil>
// 2024-03-07 05:09:04.012345678 +0000 UTC 2024-03-07 05:09:04.012345678 +0000 UTC
