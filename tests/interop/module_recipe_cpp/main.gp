// phase: 04

// A C++ library that ships its own recipe (Phase 06 step 6.1d): third_party/vecmath has a gocpp.mod with
// `cxx github.com/example/vecmath { ... }`, and this project says nothing about the library. The recipe
// compiles the library as C++17 (src/*.cpp and the file of the operating system), and its
// `define VECMATH_WIDE` reaches this program's compile too: vec_header_size() is compiled here,
// vec_lib_size() inside the library, and both must see the same struct. The header includes
// <vecmath/detail.h>, which only the recipe's include folder makes findable.
package main

import (
	"fmt"

	"github.com/example/vecmath/include/vecmath/vec.hpp"
)

func main() {
	fmt.Println(vec.vec_header_size() == vec.vec_lib_size(), vec.vec_header_size())
	fmt.Println(vec.vec_dot3(1, 2, 3, 4, 5, 6), vec.vec_os_code())
	fmt.Println(vec.vec_cxx17())
}

// Output:
// true 32
// 32 1
// 1
