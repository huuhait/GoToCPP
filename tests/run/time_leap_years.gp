// phase: 08

// Leap years and the length of months and years through Date normalization and YearDay.
package main

import (
	"fmt"
	"time"
)

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func main() {
	bad := 0
	for y := -400; y <= 2500; y++ {
		days := time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC).YearDay()
		feb := time.Date(y, 3, 0, 0, 0, 0, 0, time.UTC).Day()
		want, wfeb := 365, 28
		if isLeap(y) {
			want, wfeb = 366, 29
		}
		if days != want || feb != wfeb {
			bad++
			fmt.Println("wrong year", y, days, feb)
		}
	}
	fmt.Println("wrong years:", bad)
	for _, y := range []int{1600, 1700, 1800, 1900, 2000, 2020, 2023, 2024, 2100, 2400} {
		fmt.Println(y, isLeap(y), time.Date(y, 3, 0, 0, 0, 0, 0, time.UTC).Day(), time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC).YearDay())
	}
	// month lengths
	for m := time.January; m <= time.December; m++ {
		fmt.Print(time.Date(2023, m+1, 0, 0, 0, 0, 0, time.UTC).Day(), ",", time.Date(2024, m+1, 0, 0, 0, 0, 0, time.UTC).Day(), ";")
	}
	fmt.Println()
	// 400 years have 146097 days
	a := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	b := time.Date(2400, 1, 1, 0, 0, 0, 0, time.UTC)
	fmt.Println(b.Unix()-a.Unix() == 146097*86400, b.Sub(a) == 1<<63-1)
	// Feb 29 plus one year is Mar 1
	fmt.Println(time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC).AddDate(1, 0, 0).Format(time.DateOnly), time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC).AddDate(4, 0, 0).Format(time.DateOnly))
	// parsing Feb 29
	_, e1 := time.Parse(time.DateOnly, "2024-02-29")
	_, e2 := time.Parse(time.DateOnly, "2023-02-29")
	_, e3 := time.Parse(time.DateOnly, "1900-02-29")
	_, e4 := time.Parse(time.DateOnly, "2000-02-29")
	fmt.Println(e1, e2, e3, e4)
}

// Output:
// wrong years: 0
// 1600 true 29 366
// 1700 false 28 365
// 1800 false 28 365
// 1900 false 28 365
// 2000 true 29 366
// 2020 true 29 366
// 2023 false 28 365
// 2024 true 29 366
// 2100 false 28 365
// 2400 true 29 366
// 31,31;28,29;31,31;30,30;31,31;30,30;31,31;31,31;30,30;31,31;30,30;31,31;
// true true
// 2025-03-01 2028-02-29
// <nil> parsing time "2023-02-29": day out of range parsing time "1900-02-29": day out of range <nil>
