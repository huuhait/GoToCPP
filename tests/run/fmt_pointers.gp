// phase: 03

// fmt prints a pointer to a struct, an array, a slice or a map at the top level as &{1 2},
// &[1 2], &map[a:1]; a nil pointer as <nil> (also inside a composite value, where a non-nil
// one prints its address); %+v names the fields through the pointer, too.
package main

import "fmt"

type node struct {
	val  int
	next *node
}

type point struct{ X, Y int }

func main() {
	p := &point{1, 2}
	fmt.Println(p, *p)
	fmt.Printf("%v %+v %d\n", p, p, p)
	arr := [2]string{"a", "b"}
	s := []int{1, 2}
	m := map[string]int{"k": 1}
	fmt.Println(&arr, &s, &m)
	var np *point
	var nn *node
	var ns *[]int
	var nm *map[string]int
	fmt.Println(np, nn, ns, nm)
	fmt.Printf("%v %+v %s %d\n", np, nn, np, ns)
	last := &node{3, nil}
	fmt.Println(*last, last.next, []*point{nil, nil})
	fmt.Printf("%+v\n", *last)
	fmt.Println(node{1, nil}, map[string]*point{"x": nil})
	pp := &p
	fmt.Println(*pp, **pp)
	q := &p.Y
	*q = 20
	fmt.Println(p, *q)
	n := new(int)
	*n = 7
	fmt.Println(*n, n != nil)
}

// Output:
// &{1 2} {1 2}
// &{1 2} &{X:1 Y:2} &{1 2}
// &[a b] &[1 2] &map[k:1]
// <nil> <nil> <nil> <nil>
// <nil> <nil> %!s(*main.point=<nil>) 0
// {3 <nil>} <nil> [<nil> <nil>]
// {val:3 next:<nil>}
// {1 <nil>} map[x:<nil>]
// &{1 2} {1 2}
// &{1 20} 20
// 7 true
