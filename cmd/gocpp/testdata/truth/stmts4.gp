package main

import "fmt"

var d = 3

func main() {
	for i := 0; i < 2; i++ {
		x := 1 +
			2/d
		if x > 0 &&
			10/d > 1 {
			fmt.Println("a" +
				"b")
		}
		var s = fmt.Sprint(i) +
			fmt.Sprint(i/d)
		y := x *
			(3 / d)
		_, _, _ = x, s, y
	}
}
