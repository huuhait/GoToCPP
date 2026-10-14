// phase: 02

// Shadowing: v := v and x := x + 1 in nested blocks read the outer
// variable, if and for init statements make new variables, and the outer
// variable keeps its value.
package main

import "fmt"

func main() {
	v := 1
	{
		v := v
		v++
		fmt.Println("inner", v)
	}
	fmt.Println("outer", v)

	x := 10
	if x := x + 1; x > 10 {
		fmt.Println("if init", x)
	}
	fmt.Println("after if", x)

	for i := 0; i < 2; i++ {
		x := x * (i + 2)
		fmt.Println("loop", i, x)
	}
	fmt.Println("after loop", x)

	s := "a"
	{
		s := s + "b"
		{
			s := s + "c"
			fmt.Println(s)
		}
		fmt.Println(s)
	}
	fmt.Println(s)

	n := 3
	switch n := n * 2; n {
	case 6:
		fmt.Println("switch init", n)
	}
	fmt.Println(n)

	for i := range 3 {
		i := i * 10
		fmt.Print(i, " ")
	}
	fmt.Println("|")

	f := 1.5
	if true {
		f, g := f*2, f
		fmt.Println(f, g)
	}
	fmt.Println(f)
}

// Output:
// inner 2
// outer 1
// if init 11
// after if 10
// loop 0 20
// loop 1 30
// after loop 10
// abc
// ab
// a
// switch init 6
// 3
// 0 10 20 |
// 3 1.5
// 1.5
