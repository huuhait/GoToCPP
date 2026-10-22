package main

import (
	"fmt"
	"time"
)

func main() {
	const n = 100000
	start := time.Now()
	for i := 0; i < n; i++ {
		t := time.NewTimer(time.Hour)
		t.Stop()
	}
	d := time.Since(start)
	fmt.Println(d.Nanoseconds() / n)
}
