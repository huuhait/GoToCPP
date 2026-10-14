// phase: 03

// Aliasing as in Go, independent of how a slice grows: an append to a slice with room writes
// into the shared array (other slices see it), an append to a full slice (s[a:b:b]) copies
// into a new array (they don't), copies of a slice share the array, and pointers into an old
// array keep it.
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

func main() {
	base := make([]int, 5, 8)
	for i := range base {
		base[i] = i
	}
	a := base[:2]
	b := append(a, 100) // room: writes base[2]
	show("base", base)
	show("b", b)
	c := append(base[:2:2], 200) // full: a new array
	c[0] = -1
	show("base", base)
	show("c", c)

	d := base[3:5]
	d = append(d, 300, 400) // room up to cap 8
	show("base", base[:7])
	show("d", d)
	e := append(base[3:5:5], 500)
	e[0] = 33
	show("base", base)
	show("e", e)

	q := base
	q[4] = 44
	q = append(q[:5:5], 6) // q gets a new array
	q[4] = 99
	show("base", base)
	show("q", q)

	p := &base[1]
	base = append(base[:5:5], 7)
	*p = 11
	base[1] = 22
	fmt.Println(*p, base[1], len(base))

	stack := []int{}
	for i := range 6 {
		stack = append(stack, i*i)
	}
	top := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	stack = append(stack, -1)
	show("stack", stack)
	fmt.Println(top, len(stack))

	queue := []string{"a", "b", "c"}
	head := queue[0]
	queue = queue[1:]
	queue = append(queue, head)
	fmt.Println(len(queue), queue[0], queue[2])
}
