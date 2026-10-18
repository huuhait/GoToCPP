// phase: 08

// Channels are comparable (the same channel is equal to itself) and can be map keys and
// interface values.
package main

import "fmt"

func main() {
	c1 := make(chan int)
	c2 := make(chan int)
	c3 := c1
	fmt.Println(c1 == c2, c1 == c3, c1 != c2)

	names := map[chan int]string{c1: "one", c2: "two"}
	fmt.Println(names[c1], names[c2], names[c3], len(names), names[nil] == "")
	names[nil] = "nil"
	var nc chan int
	fmt.Println(names[nc], len(names))
	delete(names, c3)
	_, found := names[c1]
	fmt.Println(found, len(names))

	var a, b any = c1, c3
	fmt.Println(a == b, a == any(c2), a != nil)
	set := map[any]int{c1: 1, c2: 2}
	set[c3]++
	fmt.Println(set[c1], set[c2], len(set))

	both := make(chan int, 1)
	var ro <-chan int = both
	var wo chan<- int = both
	fmt.Println(ro == both, wo == both, any(ro) == any(both), any(ro) == any(ro))
}

// Output:
// false true true
// one two one 2 true
// nil 3
// false 2
// true false true
// 2 2 2
// true true false true
