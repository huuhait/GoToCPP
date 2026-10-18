// phase: 04

// <memory>: unique_ptr (move-only: moved into a local, reset, get), shared_ptr with use_count
// and a copy, make_unique / make_shared with constructor arguments, and a weak_ptr.
package main

import (
	"fmt"

	"<memory>"
	"<string>"
)

func main() {
	u := std.make_unique[std.string](3, 'x')
	fmt.Println(string(*u), u.get() != nil)
	u.reset()
	fmt.Println(u.get() == nil)

	s := std.make_shared[int](42)
	t := s
	fmt.Println(int(*s), int(s.use_count()), int(t.use_count()))
	w := std.weak_ptr[int](s)
	fmt.Println(w.expired(), int(w.use_count()))
	s.reset()
	t.reset()
	fmt.Println(w.expired())
}

// Output:
// xxx true
// true
// 42 2 2
// false 2
// true
