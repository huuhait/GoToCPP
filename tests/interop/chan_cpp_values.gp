// phase: 08

// Channels and goroutines with C/C++ values (steps 8.2 and 8.3): a C++ value that goes into a
// channel is copied if it is a variable and moved if it is a temporary; a C++ function or method
// called by a go statement runs in the new goroutine on the object that the go statement named,
// which the goroutine keeps alive; the elements can be C numbers, std::string, std::unique_ptr
// (move only) and pointers to C++ objects.
package main

import (
	"fmt"

	"./chanvals.h"
	"<memory>"
	"<string>"
	"<utility>"
	"<vector>"
)

func main() {
	// go on a member of a C++ object and on C++ functions
	var c chanvals.Counter
	go c.add(5)
	go chanvals.add_to(&c, 7)
	chanvals.wait_for(&c, 12)
	fmt.Println(int(c.get()))

	// a C number channel, and a goroutine that sends the result of a C++ function
	res := make(chan C.int)
	go func() { res <- chanvals.twice(21) }()
	fmt.Println(int(<-res))

	// std::string: a variable is copied into the channel, a temporary is moved
	words := make(chan std.string, 3)
	s := std.string("go")
	words <- s
	s.append("lang")
	words <- s
	words <- chanvals.shout(s)
	a, b := <-words, <-words
	fmt.Println(string(a), string(b), string(<-words), string(s))

	// a move-only element
	owned := make(chan std.unique_ptr[C.int], 1)
	p := std.make_unique[C.int](42)
	owned <- std.move(p)
	fmt.Println(p == nil, int(*<-owned))

	// a pointer to a C++ object in a channel, and a vector element
	ptrs := make(chan *chanvals.Counter, 1)
	ptrs <- &c
	fmt.Println(int((<-ptrs).get()))
	vecs := make(chan std.vector[C.int], 1)
	var xs std.vector[C.int]
	xs.push_back(1)
	xs.push_back(2)
	vecs <- xs
	xs.push_back(3)
	ys := <-vecs
	fmt.Println(int(xs.size()), int(ys.size()))

	// the goroutine keeps what it was given alive after the function that started it has returned
	done := make(chan std.string)
	start := func(name std.string) {
		go func() { done <- chanvals.shout(name) }()
	}
	start(std.string("kept"))
	fmt.Println(string(<-done))
	close(res)
	var ok bool
	_, ok = <-res
	fmt.Println(ok)
}

// Output:
// 12
// 42
// go golang golang! golang
// true 42
// 12
// 3 2
// kept!
// false
