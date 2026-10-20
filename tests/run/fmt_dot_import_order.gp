// phase: 05

// With import . "fmt" the arguments of Println and Sprint are copied or read in the order gc uses,
// as with fmt.Println.
package main

import . "fmt"

type pt struct{ x, y int }

func main() {
	p := pt{1, 2}
	Println("struct", p, func() int { p.x = 9; return 0 }())
	s := Sprint(p, func() int { p.x = 10; return 0 }())
	Println(s)
}

// Output:
// struct {1 2} 0
// {9 2} 0
