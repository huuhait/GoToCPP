// phase: 08

// A Ticker in a range loop; Tick in a range loop with break.
package main

import (
	"fmt"
	"time"
)

func main() {
	tk := time.NewTicker(5 * time.Millisecond)
	n := 0
	for range tk.C {
		n++
		if n == 3 {
			tk.Stop()
			break
		}
	}
	fmt.Println(n)
	m := 0
	for t := range time.Tick(5 * time.Millisecond) {
		if t.IsZero() {
			fmt.Println("zero")
		}
		m++
		if m == 4 {
			break
		}
	}
	fmt.Println(m)
}

// Output:
// 3
// 4
