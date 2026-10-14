// A variable that shadows another one keeps the Go name, in a C++ block of its
// own from its declaration to the end of its Go block; a value that reads the
// name goes into a temporary first (in C++ the new name is already in scope in
// its own initializer). Also with labels and goto, labeled continue, fallthrough,
// a for init or range variable that the body declares again, and var x T.
package main

import "fmt"

var w = 3

func f() int {
	return 40
}

func main() {
	v := 1
	v_1 := 100 // a user name like a new name of old
	{
		v := v + 1
		fmt.Println(v, v_1)
		{
			v := v * 2
			fmt.Println(v)
		}
	}
	x := 5
	{
		x := x
		var w = w + x
		f := f()
		fmt.Println(x, w, f)
	}
	{
		x, y := 2, x // y gets the outer x
		fmt.Println(x, y)
	}
	{
		y, x := x, 2 // the outer x, read before the new x is declared
		fmt.Println(x, y)
	}
	a, b := 1, 2
	{
		a := 10
		a, b := b, a // b is new and gets the old a; a is assigned the outer b
		fmt.Println(a, b)
		var c, d = d(b), b
		fmt.Println(c, d)
	}
	{
		v := v + f() // with a call
		x, y := f(), x
		var p, q = 1, 2
		{
			var p, q = q, p
			fmt.Println(v, x, y, p, q)
		}
	}
	for x := x; x < 8; x++ {
		fmt.Print(x, " ")
	}
	fmt.Println()
	for i := 0; i < 3; i++ {
		i := i * 10
		fmt.Print(i, " ")
	}
	fmt.Println()
	for i := range 3 {
		i := i + 1
		if i == 2 {
			continue
		}
		fmt.Print(i, " ")
	}
	fmt.Println()
	k := 0
	{
		k := k + 1
	retry:
		if k < 4 {
			k++
			goto retry
		}
		fmt.Println("k", k)
	}
	t := 0
	{
	top:
		t++
		t := t * 10
		if t < 30 {
			goto top // back, out of the block of the inner t
		}
		fmt.Println("t", t)
	}
outer:
	for j := range 3 {
		j := j * 2
		for m := range 2 {
			if m == 1 {
				continue outer
			}
			fmt.Println("j", j, m)
		}
	}
	switch s := 1; s {
	case 1:
		s := s + 1
		fmt.Println("s", s)
		fallthrough
	case 2:
		var s int8
		fmt.Println("two", s)
	}
	str := "go"
	{
		str := str + "!" // a class type: moved out of its temporary
		fmt.Println(str)
	}
	fmt.Println(v, x, w, f(), a, b, k, t, str)
}

func d(n int) int {
	return n * 2
}
