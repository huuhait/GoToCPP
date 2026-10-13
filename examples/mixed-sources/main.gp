// phase: 04

// Mixed-sources is one package made of gocpp, C and C++ files: main.gp and
// report.gp, checksum.c with checksum.h, and textutil.cpp with textutil.hpp.
// gocpp compiles the .c file with the C compiler and the .cpp file with the
// C++ compiler, and links everything into one program. Each header is a
// package named after its file: checksum.fnv1a32 is a C function, and
// textutil.title is a C++ function in the namespace textutil.
package main

import (
	"fmt"
	"unsafe"

	"./checksum.h"
	"./textutil.hpp"
)

func main() {
	// C: a hash function and a function that returns a C struct.
	for _, s := range []string{"", "a", "gocpp"} {
		h := uint32(checksum.fnv1a32(C.cstr(s), C.size_t(len(s))))
		fmt.Println("fnv1a32(\""+s+"\") =", h)
	}
	xs := []C.int{4, -2, 9, 7}
	st := checksum.stats_of(unsafe.SliceData(xs), C.size_t(len(xs)))
	show("min max sum", int(st.min), int(st.max), int(st.sum))
	show("checksum version", int(checksum.CHECKSUM_VERSION))

	// C++: functions that take a std::string_view (a gocpp string converts
	// to it) and return a std::string, and a class.
	text := "gocpp builds c and c++ files together"
	fmt.Println(string(textutil.title(text)))
	show("words", int(textutil.word_count(text)))
	var t textutil.tally
	for _, w := range []string{"go", "cpp", "go", "c", "go", "cpp"} {
		t.add(w)
	}
	show("go cpp rust", int(t.count("go")), int(t.count("cpp")), int(t.count("rust")))
	show("distinct", int(t.size()))
	fmt.Println("top:", string(t.top()))
}
