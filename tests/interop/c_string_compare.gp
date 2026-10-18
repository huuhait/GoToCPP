// phase: 04

// The valid forms next to c_string_compare_error.gp: the text of a C string through C.GoString, a
// std::string (compares by content), two C pointers (an address comparison, which is what it says),
// nil, and a gocpp string that came back through C++.
package main

import (
	"fmt"

	"./cunion.h"
	"<string>"
	"<vector>"
)

type Named struct{ Name string }

func main() {
	it := cunion.cu_item(3, 1.5)
	fmt.Println(C.GoString(&it.name[0]) == "item", C.GoString(&it.name[0]) != "item")

	s := std.string("item")
	fmt.Println(s == "item", s != "other", "item" == s)

	p := &it.name[0]
	q := &it.name[0]
	var none *C.char
	fmt.Println(p == q, p != nil, none == nil, p == none)

	// As conditions too.
	if s == "item" && C.GoString(&it.name[0]) == "item" {
		fmt.Println("cond")
	}
	for s != "item" {
		break
	}

	v := std.vector[*Named]{&Named{"a"}}
	fmt.Println(v[0].Name == "a")
}

// Output:
// true false
// true true true
// true true true false
// cond
// true
