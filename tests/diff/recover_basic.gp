// phase: 05

package main

import "fmt"

// safe runs f and reports what a deferred closure recovered.
func safe(f func()) {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r)
	}()
	f()
}

func main() {
	safe(func() { panic("boom") })
	safe(func() { panic(42) })
	safe(func() { panic(3.5) })
	safe(func() { panic(true) })
	safe(func() { panic('x') })
	safe(func() { panic(int8(-3)) })
	safe(func() { panic(uint16(65535)) })
	safe(func() { panic("") })
	safe(func() {})
	fmt.Println("done")
}
