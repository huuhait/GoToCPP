package main

import "fmt"

type point struct{ x, y int }

func main() {
	p := &point{1, 2} // a new reference-counted box
	px := &p.x        // shares the owner of *p
	xs := []int{1, 2, 3}
	pe := &xs[1] // shares the owner of the slice's array
	*px, *pe = 10, 20
	fmt.Println(*p, xs)
}
