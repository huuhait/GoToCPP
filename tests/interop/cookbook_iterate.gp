// phase: 04

package main

import (
	"fmt"

	"<map>"
	"<set>"
	"<string>"
	"<vector>"
)

func main() {
	v := std.vector[int]{10, 20, 30}

	// range works over anything with begin() and end(). The index and the element are C++ values
	// (a copy of the element, like Go): convert them to use them as Go numbers.
	sum := 0
	for i, x := range v {
		sum += int(i) * int(x)
	}
	fmt.Println(sum)

	// A map gives the key and a copy of the value, in the map's own order.
	m := std.map[int, std.string]{{2, "two"}, {1, "one"}}
	for k, name := range m {
		fmt.Println(int(k), string(name))
	}

	// The size is read once, as Go reads a slice's length once: v does not grow under the loop.
	for _, x := range v {
		if x == 10 {
			v.push_back(99)
		}
	}
	fmt.Println(int(v.size()))

	// The C++ way works too: an iterator is a value, and ++ is the prefix ++it.
	s := std.set[int]{3, 1, 2}
	for it := s.begin(); it != s.end(); it++ {
		fmt.Print(int(*it), ";")
	}
	fmt.Println()
}

// Output:
// 80
// 1 one
// 2 two
// 4
// 1;2;3;
