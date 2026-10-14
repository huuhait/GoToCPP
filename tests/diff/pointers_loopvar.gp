// phase: 03

// Go 1.22: each iteration of a loop has its own variables, also when their address is taken:
// a pointer keeps the variable of its iteration. In a 3-clause for, the next iteration's
// variable starts with the value of the last one, before the post statement runs.
package main

import "fmt"

func main() {
	var p0, p1, p2 *int
	for i := 0; i < 3; i++ {
		switch i {
		case 0:
			p0 = &i
		case 1:
			p1 = &i
		default:
			p2 = &i
		}
	}
	fmt.Println(*p0, *p1, *p2)

	var first, last *int
	for i := 0; i < 6; i++ {
		if first == nil {
			first = &i
		}
		last = &i
		if i == 1 {
			i += 2 // the next iteration starts from 3
			continue
		}
	}
	fmt.Println(*first, *last)

	var pj *int
	for j := range 4 {
		if j == 2 {
			pj = &j
		}
	}
	fmt.Println(*pj)

	var pr *rune
	for _, r := range "aé☃" {
		if pr == nil || r > *pr {
			pr = &r
		}
	}
	fmt.Println(string(*pr))
}
