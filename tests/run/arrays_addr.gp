// phase: 03

// &a[i] is a pointer that shares the owner of the array: a boxed local (the pointer keeps it
// alive after the function returns), a package array, an array behind a pointer or in a
// slice's array, also nested; &[N]T{...} makes a new one.
package main

import "fmt"

var g [3]int

func elem(i int) *int {
	a := [3]int{1, 2, 3}
	return &a[i]
}

func row() *[2]int {
	fmt.Println("row")
	return &[2]int{5, 6}
}

func main() {
	p := elem(1)
	*p += 10
	fmt.Println(*p)
	q := &g[2]
	*q = 7
	fmt.Println(g[2])
	a := [2][2]int{{1, 2}, {3, 4}}
	r := &a[1][0]
	*r = 30
	pr := &a[0]
	pr[1] = 20
	fmt.Println(a[0][1], a[1][0])
	pa := &a
	t := &pa[1][1]
	*t = 40
	fmt.Println(a[1][1])
	s := [][3]int{{1, 2, 3}}
	u := &s[0][2]
	s[0][2] = 33
	fmt.Println(*u)
	s = nil
	*u = 34
	fmt.Println(*u)
	v := &row()[1]
	*v++
	fmt.Println(*v)
	n := &[3]string{"a", "b", "c"}
	n[0] = "z"
	m := n
	m[1] = "y"
	fmt.Println(n[0], n[1], n[2])
	ptrs := [3]*int{}
	for i := range ptrs {
		ptrs[i] = &g[i]
	}
	*ptrs[0] = 100
	fmt.Println(g[0], ptrs[1] == &g[1])
}

// Output:
// 12
// 7
// 20 30
// 40
// 33
// 34
// row
// 7
// z y c
// 100 true
