// Names that are macros in the runtime's headers (EOF, errno, stdin, NULL,
// INT64_C, ...) and the integer type names that the generated code uses get a
// _ suffix too.
package main

import "fmt"

var EOF = "end of file"

func errno() int {
	return 22
}

func assert(b bool) bool {
	return b
}

func INT64_C(x int) int {
	return x * 2
}

func main() {
	stdin, stdout, stderr := 0, 1, 2
	NULL, EINVAL, BUFSIZ, SEEK_SET := 0, errno(), 8192, 0
	int64_t, uint8_t := 64, 8
	__x86_64__, __GNUC__ := true, 15
	linux, unix := "linux", "unix" // not macros with -std=c++20
	fmt.Println(EOF, stdin, stdout, stderr, NULL, EINVAL, BUFSIZ, SEEK_SET)
	fmt.Println(int64_t, uint8_t, __x86_64__, __GNUC__, linux, unix)
	fmt.Println(assert(true), INT64_C(21))
}
