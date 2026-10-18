// phase: 05

package main

import "fmt"

func main() {
	defer fmt.Println("main defer 1")
	for i := 0; i < 3; i++ {
		defer fmt.Println("loop defer", i)
	}
	for i := 0; i < 3; i++ {
		defer func() { fmt.Println("closure defer", i) }()
	}
	x := 10
	defer fmt.Println("arg evaluated early:", x)
	defer func() { fmt.Println("closure sees late:", x) }()
	x = 20
	defer func() {
		defer fmt.Println("nested defer a")
		defer fmt.Println("nested defer b")
		fmt.Println("in a deferred closure")
	}()
	fmt.Println("end of main, x =", x)
}
