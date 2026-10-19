# C++ containers

Importing a C++ header makes its namespace usable like a package. There is no glue code and no
binding file: `std.vector[int]` is `std::vector<int64_t>`, and its methods are the C++ methods.

<!-- program: 04 id=vector -->
```go
package main

import (
	"fmt"

	"<algorithm>"
	"<map>"
	"<string>"
	"<vector>"
)

func main() {
	// A std::vector. gocpp's int is 64 bits, so this is a vector of int64_t.
	nums := std.vector[int]{5, 3, 8, 1}
	nums.push_back(4)
	std.sort(nums.begin(), nums.end())

	// range over a C++ container works; each element is a C++ value,
	// and int(n) turns it into a Go int.
	var sorted []int
	for _, n := range nums {
		sorted = append(sorted, int(n))
	}
	fmt.Println("sorted:", sorted, "size:", int(nums.size()))

	// A std::map keeps its keys sorted, and m[k] inserts a missing key, as in C++.
	ages := std.map[std.string, int]{}
	ages["bob"] = 31
	ages["alice"] = 29
	ages["bob"]++
	for name, age := range ages {
		fmt.Println(name, age)
	}
	_ = ages["dave"]
	fmt.Println("size after reading dave:", int(ages.size()))

	// A std::string is a C++ value; string(s) makes a Go string.
	s := std.string("hello")
	s.append(", world")
	std.reverse(s.begin(), s.end())
	fmt.Println(string(s), int(s.size()))
}
```

<!-- output -->
```text
sorted: [1 3 4 5 8] size: 5
alice 29
bob 32
size after reading dave: 3
dlrow ,olleh 12
```

## What to notice

- **C++ values follow C++ rules.** Reading a missing key of a `std::map` inserts it. A
  `std::vector` has no bounds check on `v[i]` (use `v.at(i)`). The C++ compiler checks the types:
  gocpp does not model them (this is the "opaque" mode), and a mistake comes back as the C++ error
  with the line of your `.gp` file.
- What a C++ function returns is a C++ value, not a Go value. Convert it where Go needs a Go
  number or string: `int(nums.size())`, `string(s)`.
- `range` works over a C++ container ([D21](../spec/language.md#d21-range-over-c-containers)),
  with the value types of C++.
- Memory: a Go value that you put into a container is copied or moved by C++'s rules, and gocpp
  keeps the reference counts right. The recipe [who owns what](../spec/interop-cookbook.md#13-memory-who-owns-what)
  has the rules.
- The program is also [examples/cpp-std](../../examples/cpp-std/main.gp), which adds
  `std::filesystem` through a renamed namespace (`fs "::std::filesystem"`).

More: the [interop guide](../spec/interop.md) and the [cookbook](../spec/interop-cookbook.md).

Next: [C libraries](10-c-libraries.md).
