// phase: 03

// Pointers to locals, to parameters, to package variables, from new, and to pointers: every
// pointer to a variable sees every write to it, and == compares the addresses.
package main

import "fmt"

var g = 1
var gp *int
var gnew = new(string)

func set(p *int, v int) {
	*p = v
}

func addr(n int) *int {
	return &n
}

func main() {
	x, y := 1, 2
	p, q := &x, &y
	set(p, 10)
	*q += *p
	fmt.Println(x, y, *p, *q, p == q, p == &x)
	q = p
	*q *= 3
	fmt.Println(x, *p, p == q)

	a, b := addr(5), addr(5)
	fmt.Println(*a == *b, a == b)

	gp = &g
	*gp += 41
	set(&g, *gp+1)
	fmt.Println(g, *gp, gp == &g)
	*gnew += "new"
	fmt.Println(*gnew, len(*gnew))

	pp := &p
	**pp = 7
	*pp = &y
	**pp = 8
	fmt.Println(x, y, *p)

	n := new(int)
	m := n
	*m = 3
	fmt.Println(*n, n == m, n != new(int))

	var np *int
	var npp **int
	fmt.Println(np == nil, npp == nil, (*int)(nil) == np)
	npp = &np
	*npp = &x
	fmt.Println(*np)
}
