// phase: 03

// print and println of an array or a struct are errors, as in Go ("illegal types for operand:
// print").
package main

import "fmt"

type S struct{ X int }

func main() {
	a := [3]int{1, 2, 3}
	println(len(a), a)    // ERROR "illegal types for operand: println a \(\[3\]int\): println doesn't take arrays or structs, in Go too; use fmt.Println"
	print([2]string{}, 1) // ERROR "illegal types for operand: print \[2\]string\{\} \(\[2\]string\): print doesn't take"
	print(S{})            // ERROR "illegal types for operand: print S\{\} \(S\): print doesn't take arrays or structs"
	println(struct{}{})   // ERROR "illegal types for operand: println struct\{\}\{\} \(struct\{\}\): println doesn't take"
	var p *S
	println(a[0], len(a), p == nil)
	fmt.Println(len(a))
}
