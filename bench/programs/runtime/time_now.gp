// bench: time_now
// category: runtime
// size: 3000000
// smoke: 20000
// twin: no
// about: time.Now and time.Since in a loop (the clock read)
package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	ok := 0
	for i := 0; i < benchN; i++ {
		t := time.Now()
		if time.Since(start) >= 0 && !t.Before(start) {
			ok++
		}
	}
	fmt.Println(ok)
}
