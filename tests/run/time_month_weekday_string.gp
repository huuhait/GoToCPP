// phase: 08

// Month and Weekday: String, constants, values out of range, fmt verbs.
package main

import (
	"fmt"
	"time"
)

func main() {
	for m := time.January; m <= time.December; m++ {
		fmt.Printf("%d %s %v %d|", int(m), m, m, m)
	}
	fmt.Println()
	for d := time.Sunday; d <= time.Saturday; d++ {
		fmt.Printf("%d %s %v|", int(d), d, d)
	}
	fmt.Println()
	for _, m := range []time.Month{0, 13, -1, 100, 1 << 20} {
		fmt.Println(m.String(), m)
	}
	for _, d := range []time.Weekday{-1, 7, 8, 100} {
		fmt.Println(d.String(), d)
	}
	fmt.Printf("%T %T\n", time.March, time.Friday)
	fmt.Printf("%v %v\n", []time.Month{1, 2, 3}, map[time.Weekday]bool{time.Monday: true})
	fmt.Printf("%q %x %05d %s\n", time.April, time.May, time.June, time.July)
	var i any = time.August
	_, ok := i.(fmt.Stringer)
	fmt.Println(ok, time.September+1, time.Saturday+1)
	fmt.Println(time.January == 1, time.Sunday == 0, time.December == 12)
}

// Output:
// 1 January January 1|2 February February 2|3 March March 3|4 April April 4|5 May May 5|6 June June 6|7 July July 7|8 August August 8|9 September September 9|10 October October 10|11 November November 11|12 December December 12|
// 0 Sunday Sunday|1 Monday Monday|2 Tuesday Tuesday|3 Wednesday Wednesday|4 Thursday Thursday|5 Friday Friday|6 Saturday Saturday|
// %!Month(0) %!Month(0)
// %!Month(13) %!Month(13)
// %!Month(18446744073709551615) %!Month(18446744073709551615)
// %!Month(100) %!Month(100)
// %!Month(1048576) %!Month(1048576)
// %!Weekday(18446744073709551615) %!Weekday(18446744073709551615)
// %!Weekday(7) %!Weekday(7)
// %!Weekday(8) %!Weekday(8)
// %!Weekday(100) %!Weekday(100)
// time.Month time.Weekday
// [January February March] map[Monday:true]
// "April" 4d6179 00006 July
// true October %!Weekday(7)
// true true true
