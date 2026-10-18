// phase: 08

// The variables of a receive case are scoped to the case: they shadow outer ones, closures capture
// their own copy, and their address can be taken.
package main

import "fmt"

func main() {
	v := "outer"
	c := make(chan int, 4)
	for i := 1; i <= 4; i++ {
		c <- i * 10
	}
	var fs []func() int
	var ps []*int
	for i := 0; i < 3; i++ {
		select {
		case v := <-c:
			fs = append(fs, func() int { v++; return v })
			ps = append(ps, &v)
			fmt.Println("inner", v)
		}
	}
	fmt.Println(v)
	for i, f := range fs {
		a, b := f(), f()
		fmt.Println(a, b, *ps[i])
	}
	select {
	case v, ok := <-c:
		fmt.Println(v, ok)
		w := v * 2
		fmt.Println(w)
	}
	close(c)
	select {
	case v, ok := <-c:
		v, ok2 := v+1, !ok
		fmt.Println(v, ok, ok2)
	}
}


// Output:
// inner 10
// inner 20
// inner 30
// outer
// 11 12 12
// 21 22 22
// 31 32 32
// 40 true
// 80
// 1 false true
