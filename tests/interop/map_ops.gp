// phase: 04

// A std::map keeps C++'s rules: m[k] inserts the key (also a read, D14), m[k]++ and m[k] += x
// change the entry in place, and count and size are member calls.
package main

import (
	"fmt"

	"<map>"
	"<string>"
)

func main() {
	var ages std.map[std.string, int]
	ages["bob"] = 31
	ages["alice"] = 29
	ages["bob"]++
	ages["alice"] += 2
	n := 10
	ages["carol"] = n // a gocpp int into the C++ entry: C++'s conversion (an int64_t here)
	dave := ages["dave"] // inserts "dave", as operator[] does in C++
	fmt.Println(int(ages["bob"]), int(ages["alice"]), int(ages["carol"]), int(dave))
	fmt.Println(int(ages.size()), int(ages.count("eve")), bool(ages.count("dave") == 1))
	ages.erase("dave")
	fmt.Println(int(ages.size()), bool(ages.find("dave") == ages.end()))

	var counts std.map[C.int, C.double]
	counts[3] = 1.5
	counts[3] *= 2
	counts[4] -= 1
	fmt.Println(float64(counts[3]), float64(counts[4]), int(counts.size()))
}

// Output:
// 32 31 10 0
// 4 0 true
// 3 true
// 3 -1 2
