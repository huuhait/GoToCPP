// phase: 08

// The 12-hour clock in layouts: 03, 3, PM, pm around midnight and noon.
package main

import (
	"fmt"
	"time"
)

func main() {
	for h := 0; h < 24; h++ {
		t := time.Date(2024, 6, 1, h, h+7, h+13, 0, time.UTC)
		fmt.Printf("%02d: %s | %s | %s | %s | %s\n", h, t.Format("3:04PM"), t.Format("03:04:05 pm"), t.Format("15:04"), t.Format(time.Kitchen), t.Format("3 PM / 03 pm / 15"))
	}
	for _, in := range []string{"12:00AM", "12:30AM", "1:00AM", "11:59AM", "12:00PM", "12:59PM", "1:00PM", "11:59PM", "0:00AM", "13:00PM", "12:00am", "12:00Pm", "12:00", "1:5PM", "01:05PM"} {
		t, err := time.Parse(time.Kitchen, in)
		if err != nil {
			fmt.Println(in, "->", err)
			continue
		}
		fmt.Println(in, "->", t.Format("15:04"), t.Hour())
	}
	for _, in := range []string{"12:00 am", "12:00 pm", "3:04 pm", "03:04 PM", "3:04 PM"} {
		t, err := time.Parse("3:04 pm", in)
		fmt.Println(in, t.Format("15:04"), err)
	}
	// parsing "PM" with an hour in 24-hour form
	t, err := time.Parse("15:04PM", "13:00PM")
	fmt.Println(t.Format("15:04"), err)
	t, err = time.Parse("15:04PM", "03:00PM")
	fmt.Println(t.Format("15:04"), err)
}

// Output:
// 00: 12:07AM | 12:07:13 am | 00:07 | 12:07AM | 12 AM / 12 am / 00
// 01: 1:08AM | 01:08:14 am | 01:08 | 1:08AM | 1 AM / 01 am / 01
// 02: 2:09AM | 02:09:15 am | 02:09 | 2:09AM | 2 AM / 02 am / 02
// 03: 3:10AM | 03:10:16 am | 03:10 | 3:10AM | 3 AM / 03 am / 03
// 04: 4:11AM | 04:11:17 am | 04:11 | 4:11AM | 4 AM / 04 am / 04
// 05: 5:12AM | 05:12:18 am | 05:12 | 5:12AM | 5 AM / 05 am / 05
// 06: 6:13AM | 06:13:19 am | 06:13 | 6:13AM | 6 AM / 06 am / 06
// 07: 7:14AM | 07:14:20 am | 07:14 | 7:14AM | 7 AM / 07 am / 07
// 08: 8:15AM | 08:15:21 am | 08:15 | 8:15AM | 8 AM / 08 am / 08
// 09: 9:16AM | 09:16:22 am | 09:16 | 9:16AM | 9 AM / 09 am / 09
// 10: 10:17AM | 10:17:23 am | 10:17 | 10:17AM | 10 AM / 10 am / 10
// 11: 11:18AM | 11:18:24 am | 11:18 | 11:18AM | 11 AM / 11 am / 11
// 12: 12:19PM | 12:19:25 pm | 12:19 | 12:19PM | 12 PM / 12 pm / 12
// 13: 1:20PM | 01:20:26 pm | 13:20 | 1:20PM | 1 PM / 01 pm / 13
// 14: 2:21PM | 02:21:27 pm | 14:21 | 2:21PM | 2 PM / 02 pm / 14
// 15: 3:22PM | 03:22:28 pm | 15:22 | 3:22PM | 3 PM / 03 pm / 15
// 16: 4:23PM | 04:23:29 pm | 16:23 | 4:23PM | 4 PM / 04 pm / 16
// 17: 5:24PM | 05:24:30 pm | 17:24 | 5:24PM | 5 PM / 05 pm / 17
// 18: 6:25PM | 06:25:31 pm | 18:25 | 6:25PM | 6 PM / 06 pm / 18
// 19: 7:26PM | 07:26:32 pm | 19:26 | 7:26PM | 7 PM / 07 pm / 19
// 20: 8:27PM | 08:27:33 pm | 20:27 | 8:27PM | 8 PM / 08 pm / 20
// 21: 9:28PM | 09:28:34 pm | 21:28 | 9:28PM | 9 PM / 09 pm / 21
// 22: 10:29PM | 10:29:35 pm | 22:29 | 10:29PM | 10 PM / 10 pm / 22
// 23: 11:30PM | 11:30:36 pm | 23:30 | 11:30PM | 11 PM / 11 pm / 23
// 12:00AM -> 00:00 0
// 12:30AM -> 00:30 0
// 1:00AM -> 01:00 1
// 11:59AM -> 11:59 11
// 12:00PM -> 12:00 12
// 12:59PM -> 12:59 12
// 1:00PM -> 13:00 13
// 11:59PM -> 23:59 23
// 0:00AM -> 00:00 0
// 13:00PM -> parsing time "13:00PM": hour out of range
// 12:00am -> parsing time "12:00am" as "3:04PM": cannot parse "am" as "PM"
// 12:00Pm -> parsing time "12:00Pm" as "3:04PM": cannot parse "Pm" as "PM"
// 12:00 -> parsing time "12:00" as "3:04PM": cannot parse "" as "PM"
// 1:5PM -> parsing time "1:5PM" as "3:04PM": cannot parse "5PM" as "04"
// 01:05PM -> 13:05 13
// 12:00 am 00:00 <nil>
// 12:00 pm 12:00 <nil>
// 3:04 pm 15:04 <nil>
// 03:04 PM 00:00 parsing time "03:04 PM" as "3:04 pm": cannot parse "PM" as "pm"
// 3:04 PM 00:00 parsing time "3:04 PM" as "3:04 pm": cannot parse "PM" as "pm"
// 13:00 <nil>
// 15:00 <nil>
