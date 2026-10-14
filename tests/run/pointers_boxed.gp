// phase: 03

// A variable whose address is taken lives in a box that outlives its function: the pointer
// that a function returns to its parameter, its named result or its local stays valid, and
// two pointers to one variable see each other's writes.
package main

import "fmt"

func param(n int) *int {
	n++
	return &n
}

func result() (r int, p *int) {
	p = &r
	r = 7
	return
}

func local() *string {
	s := "box"
	return &s
}

func main() {
	a, b := param(1), param(10)
	*a += 100
	fmt.Println(*a, *b)
	r, p := result()
	*p *= 6
	fmt.Println(r, *p)
	s := local()
	*s += "ed"
	t := s
	*t += "!"
	fmt.Println(*s, s == t, s == local())
}

// Output:
// 102 11
// 7 42
// boxed! true false
