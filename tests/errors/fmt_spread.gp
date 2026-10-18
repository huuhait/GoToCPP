// phase: 05

// fmt's final a... must be a []any and the only operand: the checker's messages are Go's.
package main

import "fmt"

func main() {
	a := []any{1}
	s := []string{"x"}
	fmt.Println("x", a...)    // ERROR "too many arguments in call to fmt.Println"
	fmt.Println(s...)         // ERROR "cannot use s \(variable of type \[\]string\) as \[\]any value in argument to fmt.Println"
	fmt.Printf("%d", 1, a...) // ERROR "too many arguments in call to fmt.Printf"
	fmt.Sprint(a, a...)       // ERROR "too many arguments in call to fmt.Sprint"
	fmt.Printf("%d", a...)
}
