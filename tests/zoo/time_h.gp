// phase: 04

// <time.h> (the package is named time, like the Go package; this program doesn't import both):
// struct tm, timegm and gmtime_r are deterministic, strftime writes into a C buffer.
package main

import (
	"fmt"

	"<time.h>"
)

func main() {
	var tm time.struct_tm
	tm.tm_year = 2024 - 1900
	tm.tm_mon = 1
	tm.tm_mday = 29
	tm.tm_hour = 12
	secs := time.timegm(&tm)
	fmt.Println(int64(secs))

	var back time.struct_tm
	time.gmtime_r(&secs, &back)
	var buf [64]C.char
	n := time.strftime(&buf[0], 64, "%Y-%m-%d %H:%M:%S %a", &back)
	fmt.Println(int(n), C.GoString(&buf[0]), int(back.tm_yday))
	fmt.Println(time.time(nil) > 1700000000)
}

// Output:
// 1709208000
// 23 2024-02-29 12:00:00 Thu 59
// true
