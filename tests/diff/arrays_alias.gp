// phase: 03

// Arrays are values and slices of them share their memory, as in Go: a copy is independent,
// slices of one array and pointers into it see each other's writes, an append within the
// capacity writes into the array, and a pointer or a slice keeps a local array alive after
// its function returns.
package main

import "fmt"

func show(name string, s []int) {
	fmt.Print(name, " [")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println("]")
}

func window(n int) []int {
	var buf [8]int
	for i := range buf {
		buf[i] = i * n
	}
	return buf[2:5]
}

func cell() *int {
	var a [4]int
	p := &a[3]
	a[3] = 42
	return p
}

func main() {
	var a [6]int
	for i := range a {
		a[i] = i
	}
	b := a
	b[0] = 100
	s := a[1:4]
	t := a[2:]
	s[1] = 20 // a[2]
	t[2] = 40 // a[4]
	show("a", a[:])
	show("b", b[:])
	u := append(s, 300) // cap 5: writes a[4]
	show("a", a[:])
	show("u", u)
	v := append(a[1:3:3], 7) // full: a new array
	v[0] = -1
	show("a", a[:])
	show("v", v)

	p := &a[5]
	q := &a
	q[5] = 55
	fmt.Println(*p, a[5], len(q))
	pa := &a
	pa[0], *p = 9, 99
	show("a", pa[:])

	w := window(3)
	w2 := w[:cap(w)]
	w2[5] = -2
	show("w", w)
	show("w2", w2)
	c := cell()
	*c++
	fmt.Println(*c)

	m := [3][3]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	row := m[1][:]
	row[0] = 40
	col := [3]*int{&m[0][2], &m[1][2], &m[2][2]}
	for _, pp := range col {
		*pp *= 10
	}
	for _, r := range m {
		show("m", r[:])
	}
	mm := m
	mm[0][0] = -1
	fmt.Println(m[0][0], mm[0][0], m == mm)

	arrs := make([][2]int, 2, 4)
	ps := &arrs[1]
	arrs = append(arrs, [2]int{5, 6}) // room: the same array
	ps[0] = 7
	arrs[1][1] = 8
	fmt.Println(arrs[2][0], arrs[1][0], arrs[1][1], ps[1], len(arrs))
}
