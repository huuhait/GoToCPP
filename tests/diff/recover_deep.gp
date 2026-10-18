// phase: 05

package main

import "fmt"

var order []int

func dive(n int) {
	defer func() { order = append(order, n) }()
	if n == 0 {
		panic("deep")
	}
	dive(n - 1)
}

func main() {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r, "frames:", len(order))
		ok := true
		for i, v := range order {
			if v != i {
				ok = false
			}
		}
		fmt.Println("deferred calls ran innermost first:", ok)
	}()
	dive(2000)
}
