// phase: 05

// A promoted method with a parameter larger than 128 KiB: the declared method takes a const reference
// (and copies the value into a box at its entry), and the forwarder's signature must be the same, or
// the call through the path would copy 320 KB on the C++ stack; the copy is still a copy.
package main

import "fmt"

type Big [40000]int

type Sink struct{ last int }

func (s *Sink) Take(b Big, k int) int {
	b[0] += 100 // a copy: the caller's array is unchanged
	s.last = b[0] + b[39999] + k
	return s.last
}

func (s Sink) Peek(b Big) (int, Big) {
	b[1] = 7
	return b[1] + s.last, b
}

type Mid struct{ *Sink }
type Top struct {
	Mid
	name string
}

func main() {
	var b Big
	b[0], b[39999] = 1, 2
	t := Top{Mid{&Sink{}}, "t"}
	fmt.Println(t.Take(b, 3), b[0], t.last)
	n, c := t.Peek(b)
	fmt.Println(n, c[1], b[1])
	var v Top
	v.Mid.Sink = &Sink{}
	fmt.Println(v.Take(b, 0))
}

// Output:
// 106 1 106
// 113 7 0
// 103
