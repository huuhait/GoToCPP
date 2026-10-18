// phase: 03

// A positional struct literal evaluates the value of a blank field and drops it: the field stays zero.
package main

import "fmt"

type Blank struct {
	A int
	_ int
	B string
}

type Inner struct {
	_ [2]int
	X int
}

func main() {
	f := func() int {
		fmt.Println("f called")
		return 5
	}
	fmt.Printf("%v %+v\n", Blank{1, 2, "x"}, Blank{1, f(), "y"})
	b := Blank{1, 2, "z"}
	fmt.Println(b == Blank{A: 1, B: "z"}, b)
	p := &Blank{3, f(), "p"}
	fmt.Printf("%+v\n", *p)
	fmt.Println([]Blank{{4, 9, "s"}}, Inner{[2]int{1, 2}, 7})
}

// Output:
// f called
// {1 0 x} {A:1 _:0 B:y}
// true {1 0 z}
// f called
// {A:3 _:0 B:p}
// [{4 0 s}] {[0 0] 7}
