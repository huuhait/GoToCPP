// phase: 04

// An extern function with C++ linkage that only a C file defines does not link: the C++ name has
// the parameter types in it. The hint names the declaration and the fix (//gocpp:extern C).
package main

//gocpp:extern
func plain_c_function(x C.int) C.int

func main() {
	println(plain_c_function(1)) // ERROR "(?s)C\+\+: undefined reference to 'plain_c_function\(int\)'.*plain_c_function is declared with //gocpp:extern at .*main\.gp:8 as the C\+\+ function plain_c_function\(int\).*If a C file defines it, write //gocpp:extern C"
}
