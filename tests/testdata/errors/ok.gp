// phase: 01

package main

import "fmt"

func main() {
	x := 1 // ERROR "declared and not used: x"
	fmt.Println(y) // ERROR "undefined: y" "second regexp"
	var s string = 1 // ERROR "cannot use 1 \(untyped int constant\) as string value"
	z := 2 // ERROR "x\x22y|other"
	fmt.Println(z) //// ERROR "turned off"
	_ = "no ERROR comment here"
	a, b := 1 // ERROR "assignment mismatch" "LINE-1, LINE and LINE+2"
	_, _ = a, b // ERRORS is not an ERROR comment
	_ = s // ERROR ""
}
