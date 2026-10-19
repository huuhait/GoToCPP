// phase: 08

// Day of the year (002) and the padding variants of the day (2, _2, 02) and the month (1, 01), in Format and Parse.
package main

import (
	"fmt"
	"time"
)

func main() {
	dates := []time.Time{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 4, 9, 0, 0, 0, 0, time.UTC), time.Date(2024, 10, 10, 0, 0, 0, 0, time.UTC), time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	for _, d := range dates {
		fmt.Printf("%s|%s|%s|%s|%s|%s|%s|%s|%d\n", d.Format("2006-002"), d.Format("__2"), d.Format("_2"), d.Format("2"), d.Format("02"),
			d.Format("1/2/06"), d.Format("01/_2/2006"), d.Format("Jan _2"), d.YearDay())
	}
	for _, in := range []string{"2024-001", "2024-060", "2024-366", "2023-366", "2024-000", "2024-367", "2024-1", "2024-12"} {
		t, err := time.Parse("2006-002", in)
		fmt.Println(in, t.Format("2006-01-02"), err)
	}
	for _, in := range []string{"  1", " 12", "123", "1", "01", "  ", "x"} {
		t, err := time.Parse("__2", in)
		fmt.Println(fmt.Sprintf("%q", in), t.YearDay(), err)
	}
	for _, in := range []string{"Jan  2", "Jan 2", "Jan 12", "Jan 02", "Jan   2", "Jan 2 ", "Jan"} {
		t, err := time.Parse("Jan _2", in)
		fmt.Println(fmt.Sprintf("%q", in), t.Day(), err)
	}
	for _, in := range []string{"3/4/24", "03/04/24", "3/4/2024", "13/4/24", "3/32/24", "3/31/24", "2/30/24", "2/29/24", "2/29/23"} {
		t, err := time.Parse("1/2/06", in)
		fmt.Println(in, t.Format("2006-01-02"), err)
	}
	// 2-digit years: 69-99 are 19xx, 00-68 are 20xx
	for _, in := range []string{"68", "69", "99", "00", "01"} {
		t, err := time.Parse("06", in)
		fmt.Println(in, t.Year(), err)
	}
}

// Output:
// 2024-001|  1| 1|1|01|1/1/24|01/ 1/2024|Jan  1|1
// 2024-009|  9| 9|9|09|1/9/24|01/ 9/2024|Jan  9|9
// 2024-010| 10|10|10|10|1/10/24|01/10/2024|Jan 10|10
// 2024-060| 60|29|29|29|2/29/24|02/29/2024|Feb 29|60
// 2024-061| 61| 1|1|01|3/1/24|03/ 1/2024|Mar  1|61
// 2023-060| 60| 1|1|01|3/1/23|03/ 1/2023|Mar  1|60
// 2024-100|100| 9|9|09|4/9/24|04/ 9/2024|Apr  9|100
// 2024-284|284|10|10|10|10/10/24|10/10/2024|Oct 10|284
// 2024-366|366|31|31|31|12/31/24|12/31/2024|Dec 31|366
// 2023-365|365|31|31|31|12/31/23|12/31/2023|Dec 31|365
// 2024-001 2024-01-01 <nil>
// 2024-060 2024-02-29 <nil>
// 2024-366 2024-12-31 <nil>
// 2023-366 0001-01-01 parsing time "2023-366": day-of-year out of range
// 2024-000 0001-01-01 parsing time "2024-000": day-of-year out of range
// 2024-367 0001-01-01 parsing time "2024-367": day-of-year out of range
// 2024-1 0001-01-01 parsing time "2024-1" as "2006-002": cannot parse "1" as "002"
// 2024-12 0001-01-01 parsing time "2024-12" as "2006-002": cannot parse "12" as "002"
// "  1" 1 <nil>
// " 12" 12 <nil>
// "123" 123 <nil>
// "1" 1 <nil>
// "01" 1 <nil>
// "  " 1 parsing time "  " as "__2": cannot parse "  " as "__2"
// "x" 1 parsing time "x" as "__2": cannot parse "x" as "__2"
// "Jan  2" 2 <nil>
// "Jan 2" 2 <nil>
// "Jan 12" 12 <nil>
// "Jan 02" 2 <nil>
// "Jan   2" 2 <nil>
// "Jan 2 " 1 parsing time "Jan 2 ": extra text: " "
// "Jan" 1 parsing time "Jan" as "Jan _2": cannot parse "" as "_2"
// 3/4/24 2024-03-04 <nil>
// 03/04/24 2024-03-04 <nil>
// 3/4/2024 0001-01-01 parsing time "3/4/2024": extra text: "24"
// 13/4/24 0001-01-01 parsing time "13/4/24": month out of range
// 3/32/24 0001-01-01 parsing time "3/32/24": day out of range
// 3/31/24 2024-03-31 <nil>
// 2/30/24 0001-01-01 parsing time "2/30/24": day out of range
// 2/29/24 2024-02-29 <nil>
// 2/29/23 0001-01-01 parsing time "2/29/23": day out of range
// 68 2068 <nil>
// 69 1969 <nil>
// 99 1999 <nil>
// 00 2000 <nil>
// 01 2001 <nil>
