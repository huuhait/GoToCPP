// phase: 04

// C functions called with untyped constants: they are plain C literals there (5 is an int,
// 2.5 a double, "x" a string literal, 'z' a char), as printf wants them. Only C's stdio
// writes to stdout here: it has its own buffer, which gocpp's fmt doesn't share.
package main

import (
	"<cstdio>"
	"<stdio.h>"
)

func main() {
	cstdio.printf("%d %.1f %s %c\n", 5, 2.5, "x", 'z')
	var n C.int = cstdio.printf("%s\n", "hello")
	cstdio.printf("printf wrote %d bytes\n", n)
	cstdio.puts("puts")
	stdio.printf("%lld %llu\n", C.longlong(1<<40), C.ulonglong(1<<63))
	cstdio.printf("EOF is %d\n", cstdio.EOF)
	big := C.longlong(1 << 40)
	cstdio.printf("%lld\n", big)
	cstdio.fflush(cstdio.stdout)
}

// Output:
// 5 2.5 x z
// hello
// printf wrote 6 bytes
// puts
// 1099511627776 9223372036854775808
// EOF is -1
// 1099511627776
