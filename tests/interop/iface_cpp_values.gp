// phase: 05

// C/C++ values in interfaces (step 5.4): an interface without methods holds a C/C++ value as it is,
// with the foreign descriptor of its C++ type. std::string, C.int and std::vector[int] are
// dynamic types of their own, different from the Go types next to them: x.(std.string), x.(C.int)
// and the cases of a type switch compare with them.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func kind(x any) string {
	switch x.(type) {
	case std.string:
		return "std::string"
	case C.int:
		return "C.int"
	case int:
		return "int"
	case std.vector[int]:
		return "vector"
	case nil:
		return "nil"
	}
	return "other"
}

func main() {
	var a any = std.string("hello")
	s := a.(std.string)
	fmt.Println(string(s))
	var b any = C.int(5)
	n := b.(C.int)
	fmt.Println(int(n))
	fmt.Println(kind(a), kind(b), kind(1), kind(std.vector[int]{}), kind("s"), kind(nil))
	var c C.double = 2.5
	d := any(c)
	fmt.Println(kind(d), d.(C.double) == 2.5)
	t, okT := a.(std.string)
	_, okI := a.(C.int)
	m, okM := b.(C.int)
	var u, okU = a.(std.string)
	fmt.Println(string(t), okT, okI, int(m), okM, string(u), okU)
	if v, ok := b.(C.int); ok {
		fmt.Println("C.int", int(v))
	}
	var items []any
	items = append(items, any(std.string("x")), any(C.int(1)), 7, "go")
	count := 0
	for _, it := range items {
		if kind(it) != "other" {
			count++
		}
	}
	fmt.Println(len(items), count)
	var same any = C.int(5)
	fmt.Println(b == same, b == any(C.int(6)), b == any(5))
}

// Output:
// hello
// 5
// std::string C.int int vector other nil
// other true
// hello true false 5 true hello true
// C.int 5
// 4 3
// true false false
