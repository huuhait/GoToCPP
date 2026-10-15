// phase: 03

// fmt's arguments larger than 128 KiB (arrays and structs) are copied to the heap at the call,
// not onto the C++ stack, which doesn't grow (an 8 MiB copy would overflow it).
package main

import "fmt"

type big struct {
	n   int
	arr [1 << 20]int
}

type grid [1 << 20]int64

var a [1 << 20]int
var b big
var g grid

func change() int {
	a[5] = 9
	b.n = 9
	return 1
}

func main() {
	a[5] = 3
	b.n = 2
	b.arr[1] = 7
	g[0] = 4
	s := fmt.Sprint(a, b, g, &a)
	fmt.Println(len(s), s[:20], len(fmt.Sprintf("%v %d %s", a, b, g)))
	fmt.Println(len(fmt.Sprint(a, change(), [1 << 20]int{1: 5}, big{n: 1})), a[5], b.n)
	var l [1 << 20]bool
	l[3] = true
	fmt.Println(len(fmt.Sprint(l)), len(fmt.Sprintln(l, b)))
	defer func2(a)
}

func func2(x [1 << 20]int) {
	fmt.Println(len(fmt.Sprint(x)), x[5])
}
