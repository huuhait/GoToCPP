package main

import (
	"fmt"
	"sort"
	"time"
)

func main() {
	for _, d := range []time.Duration{time.Millisecond, 2 * time.Millisecond, 5 * time.Millisecond, 15 * time.Millisecond, 100 * time.Millisecond} {
		n := 200
		if d >= 100*time.Millisecond {
			n = 20
		}
		late := make([]int64, n)
		for i := range late {
			s := time.Now()
			time.Sleep(d)
			late[i] = int64(time.Since(s) - d)
		}
		sort.Slice(late, func(a, b int) bool { return late[a] < late[b] })
		fmt.Printf("gosleep %6d n=%d median=%d p90=%d p99=%d max=%d min=%d\n", d.Microseconds(), n, late[n/2]/1000, late[n*9/10]/1000, late[(n*99+99)/100-1]/1000, late[n-1]/1000, late[0]/1000)
	}
}
