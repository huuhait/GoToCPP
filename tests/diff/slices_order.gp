// phase: 03

// Go's order of evaluation (gc's) with slices: an element is read after the calls of its
// statement (a call can change it through another slice of the array), also a byte as an
// argument of fmt; len, cap, a slice expression, append, copy and []byte(s) are evaluated in
// their places among the calls, and string(b) after them; a store s[i] = f() goes into the
// slice that s is after f; in s, s[0] = t, v and i, s[i] = 1, v the store goes into the old s
// and to the old i; the operands of a target run before the values' calls.
package main

import "fmt"

var gs = []int{1, 2, 3}
var gb = []byte("abc")
var gx = 1

func f() int {
	fmt.Print("f ")
	gs = append(gs, 100, 200, 300, 400)
	gs[0] = 77
	gb[0] = 'z'
	gx = 50
	return 9
}

func show(s []int) {
	fmt.Print(" [")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Print("]")
}

func at(i int) int {
	fmt.Print("at", i, " ")
	return i
}

func reset() {
	gs = []int{1, 2, 3}
	gb = []byte("abc")
	gx = 1
}

func main() {
	old := gs
	gs[1] = f()
	show(old)
	show(gs)
	fmt.Println()

	reset()
	fmt.Println(len(gs), f(), len(gs))
	reset()
	fmt.Println(string(gb), f(), gb[1], len([]byte(string(gb))))
	reset()
	fmt.Println(gb[0], f(), gb[0] == 'z')
	reset()
	fmt.Println(cap(gs)-len(gs), f(), len(gs))
	reset()
	fmt.Println(len(gs[1:]), f(), len(gs[1:]))
	reset()
	fmt.Println(gs[0]+f(), gs[0])
	reset()
	al := gs
	fmt.Println(al[0], f(), al[0])
	reset()
	al = make([]int, 3, 10)
	gs = al[:2]
	fmt.Println(al[2], f(), al[2])

	reset()
	x := []int{gx, f(), gx}
	show(x)
	fmt.Println()
	reset()
	t := append(gs, f())
	show(t)
	fmt.Println()
	reset()
	fmt.Println(copy(gs, []int{5, 6}), f(), gs[0])
	reset()
	old = gs
	gs[0] += f()
	show(old)
	show(gs)
	fmt.Println()

	v := make([]int, 6)
	v[at(1)] = at(5)
	v[at(2)], v[at(3)] = at(4), at(0)
	show(v)
	fmt.Println()

	s := []int{1, 2}
	u := []int{3, 4}
	old2 := s
	s, s[0] = u, 5
	show(s)
	show(u)
	show(old2)
	fmt.Println()
	i := 0
	w := []int{0, 0, 0}
	i, w[i] = 1, 2
	fmt.Print(i)
	show(w)
	fmt.Println()
	for i, w[i] = range []int{7, 8, 9} {
	}
	fmt.Print(i)
	show(w)
	fmt.Println()
	w[0], w[0] = 1, 2
	w[at(0)]++
	fmt.Println(w[0])

	reset()
	p := &gs[0]
	f()
	*p += 1000
	show(gs)
	fmt.Println(*p)
}
