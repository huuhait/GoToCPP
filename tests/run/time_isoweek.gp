// phase: 08

// Time.ISOWeek and YearDay across year boundaries, and Weekday over a range of dates.
package main

import (
	"fmt"
	"time"
)

func main() {
	for y := 2019; y <= 2022; y++ {
		for _, md := range [][2]int{{1, 1}, {1, 2}, {1, 3}, {1, 4}, {1, 5}, {12, 28}, {12, 29}, {12, 30}, {12, 31}} {
			t := time.Date(y, time.Month(md[0]), md[1], 12, 0, 0, 0, time.UTC)
			yy, w := t.ISOWeek()
			fmt.Printf("%s %s: %d-W%02d yday=%d\n", t.Format("2006-01-02"), t.Weekday(), yy, w, t.YearDay())
		}
	}
	// weeks 52/53
	for _, d := range []time.Time{
		time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2015, 12, 31, 0, 0, 0, 0, time.UTC), time.Date(2016, 1, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC),
	} {
		y, w := d.ISOWeek()
		fmt.Println(d.Format("2006-01-02 Mon"), y, w)
	}
	// every day of a leap year: week numbers change on Mondays only
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	prev := 0
	changes := 0
	for i := 0; i < 366; i++ {
		_, w := t.ISOWeek()
		if w != prev {
			changes++
			if t.Weekday() != time.Monday && prev != 0 {
				fmt.Println("week changed on", t.Weekday())
			}
		}
		prev = w
		t = t.AddDate(0, 0, 1)
	}
	fmt.Println("changes:", changes)
	// Weekday for a range of dates
	for _, d := range []time.Time{
		time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC),
		time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC), time.Date(1969, 12, 31, 0, 0, 0, 0, time.UTC), time.Date(1900, 2, 28, 0, 0, 0, 0, time.UTC),
		time.Date(1900, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2100, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(1582, 10, 15, 0, 0, 0, 0, time.UTC),
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2038, 1, 19, 3, 14, 7, 0, time.UTC), time.Date(2262, 4, 11, 0, 0, 0, 0, time.UTC),
	} {
		fmt.Println(d.Format("2006-01-02"), d.Weekday(), int(d.Weekday()))
	}
	days := 0
	counts := map[time.Weekday]int{}
	for t := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC); t.Year() < 2400; t = t.AddDate(0, 0, 1) {
		days++
		if t.Day() == 13 {
			counts[t.Weekday()]++
		}
	}
	fmt.Println(days, counts[time.Friday], counts[time.Saturday], counts[time.Sunday], counts[time.Monday], counts[time.Tuesday], counts[time.Wednesday], counts[time.Thursday])
}

// Output:
// 2019-01-01 Tuesday: 2019-W01 yday=1
// 2019-01-02 Wednesday: 2019-W01 yday=2
// 2019-01-03 Thursday: 2019-W01 yday=3
// 2019-01-04 Friday: 2019-W01 yday=4
// 2019-01-05 Saturday: 2019-W01 yday=5
// 2019-12-28 Saturday: 2019-W52 yday=362
// 2019-12-29 Sunday: 2019-W52 yday=363
// 2019-12-30 Monday: 2020-W01 yday=364
// 2019-12-31 Tuesday: 2020-W01 yday=365
// 2020-01-01 Wednesday: 2020-W01 yday=1
// 2020-01-02 Thursday: 2020-W01 yday=2
// 2020-01-03 Friday: 2020-W01 yday=3
// 2020-01-04 Saturday: 2020-W01 yday=4
// 2020-01-05 Sunday: 2020-W01 yday=5
// 2020-12-28 Monday: 2020-W53 yday=363
// 2020-12-29 Tuesday: 2020-W53 yday=364
// 2020-12-30 Wednesday: 2020-W53 yday=365
// 2020-12-31 Thursday: 2020-W53 yday=366
// 2021-01-01 Friday: 2020-W53 yday=1
// 2021-01-02 Saturday: 2020-W53 yday=2
// 2021-01-03 Sunday: 2020-W53 yday=3
// 2021-01-04 Monday: 2021-W01 yday=4
// 2021-01-05 Tuesday: 2021-W01 yday=5
// 2021-12-28 Tuesday: 2021-W52 yday=362
// 2021-12-29 Wednesday: 2021-W52 yday=363
// 2021-12-30 Thursday: 2021-W52 yday=364
// 2021-12-31 Friday: 2021-W52 yday=365
// 2022-01-01 Saturday: 2021-W52 yday=1
// 2022-01-02 Sunday: 2021-W52 yday=2
// 2022-01-03 Monday: 2022-W01 yday=3
// 2022-01-04 Tuesday: 2022-W01 yday=4
// 2022-01-05 Wednesday: 2022-W01 yday=5
// 2022-12-28 Wednesday: 2022-W52 yday=362
// 2022-12-29 Thursday: 2022-W52 yday=363
// 2022-12-30 Friday: 2022-W52 yday=364
// 2022-12-31 Saturday: 2022-W52 yday=365
// 2020-12-31 Thu 2020 53
// 2021-01-03 Sun 2020 53
// 2026-12-31 Thu 2026 53
// 2027-01-01 Fri 2026 53
// 2015-12-31 Thu 2015 53
// 2016-01-03 Sun 2015 53
// 2024-02-29 Thu 2024 9
// 1900-01-01 Mon 1900 1
// 0001-01-01 Mon 1 1
// 9999-12-31 Fri 9999 52
// changes: 53
// 1970-01-01 Thursday 4
// 2000-01-01 Saturday 6
// 2000-02-29 Tuesday 2
// 1999-12-31 Friday 5
// 1969-12-31 Wednesday 3
// 1900-02-28 Wednesday 3
// 1900-03-01 Thursday 4
// 2100-03-01 Monday 1
// 1582-10-15 Friday 5
// 0001-01-01 Monday 1
// 2038-01-19 Tuesday 2
// 2262-04-11 Friday 5
// 146097 688 684 687 685 685 687 684
