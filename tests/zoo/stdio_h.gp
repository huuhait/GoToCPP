// phase: 04

// <stdio.h> in opaque mode: printf with C varargs (a Go string and an untyped constant go through
// `...`), snprintf into a C buffer, FILE* from tmpfile, and the EOF macro (unqualified in C++).
package main

import (
	"fmt"

	"<stdio.h>"
)

func main() {
	stdio.printf("%d-%s-%.2f\n", 42, "c string", 2.5)
	name := "from a Go string"
	stdio.printf("%s\n", C.cstr(name))

	var buf [32]C.char
	n := stdio.snprintf(&buf[0], 32, "%03d|%c", 7, 'x')
	fmt.Println(int(n), C.GoString(&buf[0]))

	f := stdio.tmpfile()
	stdio.fputs("hello\nworld\n", f)
	stdio.rewind(f)
	var line [16]C.char
	for stdio.fgets(&line[0], 16, f) != nil {
		fmt.Print(C.GoString(&line[0]))
	}
	fmt.Println(stdio.fgetc(f) == stdio.EOF, stdio.fclose(f))
}

// Output:
// 42-c string-2.50
// from a Go string
// 5 007|x
// hello
// world
// true 0
