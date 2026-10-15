// phase: 04
// pkg-config: sqlite3

// A library's header without the library: the program uses sqlite3.h but has no
// //gocpp:pkg-config sqlite3 line, so the link finds no sqlite3_libversion_number.
package main

import (
	"fmt"

	"<sqlite3.h>"
)

func main() {
	fmt.Println(int(sqlite3.sqlite3_libversion_number())) // ERROR "C\+\+: undefined reference to 'sqlite3_libversion_number'"
}
