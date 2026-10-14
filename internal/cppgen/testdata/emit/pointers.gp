// Pointers and boxing (Phase 03 step 3.3): a variable whose address is taken lives in a box,
// gp::ptr<T> x = gp::new_<T>(...), and each use is (*x); &x is the box, &global a
// gp::static_ptr, new(T) gp::new_<T>(). Boxed parameters are copied into their boxes at the
// entry, boxed named results start as new zero boxes, and a boxed variable of a 3-clause for
// gets a new box for each iteration (Go 1.22). Two pointers to one variable see each other's
// writes.
package main

import "fmt"

var g = 3
var gptr *int
var gnew = new(int)
var gaddr = &g

func inc(p *int) {
	*p++
}

func swap(a, b *int) {
	*a, *b = *b, *a
}

// grow takes the address of its parameter: n is boxed.
func grow(n int, s string) (*int, *string) {
	n *= 2
	s += "!"
	return &n, &s
}

// named takes the address of its named result.
func named() (r int, q *int) {
	q = &r
	r = 5
	*q += 1
	return
}

func pick(p *int) *int {
	fmt.Println("pick")
	return p
}

func main() {
	x := 1
	p := &x
	*p = 10
	inc(p)
	q := p
	*q *= 2
	fmt.Println(x, *p, *q, p == q, p == &x, p != nil)

	y := 2
	swap(&x, &y)
	fmt.Println(x, y)

	n, s := grow(21, "hi")
	fmt.Println(*n, *s)
	r, rp := named()
	fmt.Println(r, *rp)

	gptr = &g
	*gptr += 4
	*gaddr++
	*gnew = 7
	fmt.Println(g, *gptr, *gnew, gptr == gaddr)

	pp := &p
	**pp += 1
	*pick(*pp) += 100
	fmt.Println(x, **pp)

	var np *int
	fmt.Println(np == nil, (*int)(nil) == np, nil != p)

	// Each iteration has its own i: last keeps the one of the second.
	var last *int
	for i := 0; i < 3; i++ {
		if i == 1 {
			last = &i
		}
	}
	fmt.Println(*last)
	for i := range 3 {
		if i == 2 {
			last = &i
		}
	}
	fmt.Println(*last)
	str := "héllo"
	var pr *rune
	for _, c := range str {
		if c > 127 {
			pr = &c
		}
	}
	fmt.Println(*pr)

	// p, *p = ...: the store through p uses the old p.
	a, b := 1, 2
	pa := &a
	pa, *pa = &b, 5
	fmt.Println(a, b, *pa)

	// A call in the target of op= runs once, also in a post statement.
	cnt := 0
	for i := 0; i < 2; *pick(&cnt) += 1 {
		i++
	}
	fmt.Println(cnt)

	// A call that writes through a pointer runs before the reads around it.
	fmt.Println(x+bump(p), x)
}

func bump(p *int) int {
	*p += 1000
	return 1
}
