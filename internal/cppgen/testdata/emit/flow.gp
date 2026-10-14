// Corner cases of statements: package variables that a call changes (tags and
// parallel assignments need temporaries then), continue from a switch in a loop
// whose post is not one C++ expression, nested labels, a goto inside a case,
// labels at the end of a block, and else if chains inside cases.
package main

import "fmt"

var g = 1

func bump() int {
	g++
	return g
}

func name(k int) string {
	switch k {
	case 0:
		return "0"
	case 1:
		return "1"
	}
	return "other"
}

func main() {
	// The tag is evaluated before the cases: bump changes g afterwards.
	switch g {
	case bump():
		fmt.Println("bumped", g)
	case 1:
		fmt.Println("one", g)
	}
	// The values are evaluated before any assignment.
	x := 10
	g, x = x, g
	fmt.Println(g, x)
	g, x = bump(), g
	fmt.Println(g, x)
	a, b := 1, 2
	a, b = b, a+1
	fmt.Println(a, b)
	a, b = a+1, a
	fmt.Println(a, b)

	// continue in a switch in a loop whose post comes at the end of the body.
	for i, j := 0, 0; i < 6; i, j = i+1, j+i {
		switch i % 3 {
		case 0:
			continue
		case 1:
			if j > 3 {
				break
			}
			fmt.Print("small ")
		}
		fmt.Print(i, ":", j, " ")
	}
	fmt.Println()

	// Labels: nested loops and switches with every kind of jump.
	n := 0
outer:
	for i := range 4 {
	inner:
		for j := 0; j < 4; j++ {
			switch s := i + j; {
			case s == 5:
				break outer
			case s%2 == 1:
				continue inner
			case j == 2:
				continue outer
			case i == j:
				break inner
			}
			n += 10*i + j
		}
		n++
	}
	fmt.Println("n", n)

	// A goto inside a case, and fallthrough into a default at the end.
	for k := range 3 {
		switch name(k) {
		case "0":
			goto zero
		zero:
			fmt.Println("zero")
			fallthrough
		case "1":
			fmt.Println("one or after zero")
			fallthrough
		default:
			fmt.Println("default", k)
		}
	}

	// A label at the end of a block, and goto out of an if chain.
	for i := 0; i < 3; i++ {
		if i == 1 {
			goto next
		} else if j := i * 2; j > 3 {
			fmt.Println("big", j)
		} else {
			fmt.Println("small", j)
		}
		fmt.Println("after if", i)
	next:
	}

	// A big unsigned count, and range with break.
	var big uint64 = 1 << 63
	for i := range big {
		if i == 2 {
			break
		}
		fmt.Println("i", i)
	}
	// else if chains inside cases, with init statements.
	for v := range 4 {
		switch {
		case v < 2:
			if w := v * 3; w == 0 {
				fmt.Println("w zero")
			} else if u := w + 1; u > 3 {
				fmt.Println("u", u)
			} else {
				fmt.Println("w u", w, u)
			}
		default:
			fmt.Println("v", v)
		}
	}
}
