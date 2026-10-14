// phase: 03

// Go's order of evaluation (gc's) with pointers: a variable whose address is taken, and *p,
// are read after the calls of their statement, which can change them through a pointer; a
// call in a target runs before the values' calls; in p, *p = q, v the store goes through the
// old p, also when the values are the results of one call; x op= y evaluates a call in its
// target once.
package main

import "fmt"

var p *int
var a, b = 10, 20

func f() int {
	fmt.Println("f")
	p = &b
	return 7
}

func bump() int {
	fmt.Println("bump")
	*p += 100
	return 1
}

func at(q *int) *int {
	fmt.Println("at")
	return q
}

func two() (*int, int) {
	fmt.Println("two")
	return &b, 5
}

func pair() (int, int) {
	fmt.Println("pair")
	return 3, 4
}

func main() {
	p = &a
	*p = f()
	fmt.Println(a, b)

	p = &a
	*p += f()
	fmt.Println(a, b)

	x := 5
	p = &x
	fmt.Println(x+bump(), x)
	y, z := x, bump()
	fmt.Println(y, z, x)
	var u, w = *p, bump()
	fmt.Println(u, w)

	c, d := 1, 2
	q := &c
	q, *q = &d, 5
	fmt.Println(c, d, *q)
	*q, q = 6, &c
	fmt.Println(c, d, *q)

	p = &a
	var v int
	*p, v = 1, f()
	fmt.Println(a, b, v)

	a, b = 10, 20
	p = &a
	p, *p = two()
	*at(&v), v = pair()
	fmt.Println(a, b, *p, v)

	*at(&x) += 1
	*at(&x)++
	*at(&x) = bump()
	fmt.Println(x)

	switch x {
	case bump():
		fmt.Println("one")
	default:
		fmt.Println("default", x)
	}

	k := 0
	p = &k
	for i := 0; i < k+bump() && i < 3; i++ {
	}
	fmt.Println(k)
}
