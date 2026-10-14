// phase: 02

// A declaration that reads the name it declares (v := v, x := x + 1, the
// same name for a different type) reads the outer variable, in blocks, if,
// for and switch init statements, and in multi-assignments.
package main

import "fmt"

func main() {
	v := 1
	{
		v := v
		fmt.Println(v)
		{
			v := v + 10
			fmt.Println(v)
		}
	}
	fmt.Println(v)

	x := 2
	for i := range 3 {
		x := x + i
		x *= 10
		fmt.Println("loop", x)
	}
	fmt.Println(x)

	s := "s"
	if s := s + s; len(s) == 2 {
		s := s + s
		fmt.Println(s)
	}
	fmt.Println(s)

	a, b := 1, 2
	{
		a, b := b, a
		fmt.Println(a, b)
		b, c := a+b, a*b
		fmt.Println(b, c)
	}
	fmt.Println(a, b)

	n := 5
	{
		n := float64(n) / 2
		fmt.Println(n)
		{
			n := int8(n * 100)
			fmt.Println(n)
		}
	}
	fmt.Println(n)

	for n := n; n > 0; n -= 2 {
		fmt.Print(n, " ")
	}
	fmt.Println(n)

	switch n := n * n; {
	case n > 20:
		n := n - 20
		fmt.Println("switch", n)
	}

	y := 3
	y, z := y+1, y+2
	fmt.Println(y, z)
}
