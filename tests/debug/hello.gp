// phase: 02

// The program of the Phase 02 README (§2.1, §2.8): gdb stops at .gp lines,
// prints the locals by their Go names, and bt shows the .gp lines of the
// program's frames.
package main

import "fmt"

func add(a, b int) (int, bool) {
	return a + b, a+b > 10 // gdb: print a = 7; print b = 5; bt gpkg_main::add gpkg_main::main
}

func main() {
	s, big := add(7, 5)
	fmt.Println("hello, gocpp", s, big) // gdb: print s = 12; print big = true; bt gpkg_main::main; next 17
}
