// phase: 12
// opaque: reject

// Members of an embedded class that are named like Go keywords and built-in functions are promoted
// like any other (`s.type()`, `s.range(1, 4)`), also from two classes in one struct, and the name of
// a Go method of the outer struct hides the class's member of the same name only at a greater depth.
package main

import (
	"fmt"

	"./embed_names.hpp"
)

type S struct {
	embed_names.Words
	embed_names.More
}

func (s *S) Double() int { return int(s.select(2)) * 2 }

func main() {
	var s S
	fmt.Println(int(s.type()), int(s.range(1, 4)), int(s.select(3)), int(s.Double()))
	fmt.Println(int(s.func), int(s.var), int(s.go), int(s.map), int(s.len()), int(s.append(1)))
	fmt.Println(int(s.interface), int(s.defer), int(s.chan), int(s.fallthrough))
	fmt.Println(int(s.package), int(s.import), int(s.cap()), int(s.make()))
	s.go = 50
	s.func++
	defer s.select(1)
	defer fmt.Println("deferred", int(s.go), int(s.func))
}

// Output:
// 1 3 6 8
// 3 4 5 6 7 9
// 9 10 11 12
// 13 14 15 16
// deferred 50 4
