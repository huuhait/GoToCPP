// phase: 02

// Shadowed names keep their Go names in C++, each in a block of its own
// (README.md, "Shadowed names"): "print x" shows the outer x on the lines
// above the declaration and on its line, and the inner x after it: in a
// block, in an if with an init, in a loop body that declares its range
// variable again, and in a declaration of two names.
package main

import "fmt"

func main() {
	x := 1
	for i := range 2 {
		fmt.Println(x, i) // gdb: print x = 1; next 16; print x = 1; next 17; print x = 11
		x := x + 10 + i
		fmt.Println(x) // gdb: print x = 11; print i = 0
	}
	if x := x * 5; x > 0 {
		fmt.Println(x) // gdb: print x = 5
	}
	for i := range 2 {
		i := i + 100
		fmt.Println(i) // gdb: print i = 100
	}
	{
		fmt.Println(x) // gdb: print x = 1; next 28; print x = 1; next 29
		x, y := 2, x
		fmt.Println(x, y) // gdb: print x = 2; print y = 1
	}
	fmt.Println(x) // gdb: print x = 1
}
