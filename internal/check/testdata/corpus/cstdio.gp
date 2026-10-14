// C headers: <cstdio>, a named <string.h>, C's built-in types and the
// boundary helpers of C.
package main

import (
	"fmt"
	"os"

	cstring "<string.h>"
	"<cstdio>"
	"<cstdlib>"
)

func readFirst(path string) (string, error) {
	f := cstdio.fopen(C.cstr(path), C.cstr("r"))
	if f == nil {
		return "", os.ErrNotExist
	}
	defer cstdio.fclose(f)
	var buf [256]C.char
	if cstdio.fgets(&buf[0], 256, f) == nil {
		return "", nil
	}
	return C.GoString(&buf[0]), nil
}

func main() {
	s := "hello"
	n := cstring.strlen(C.cstr(s))
	fmt.Println(int(n), cstdio.EOF)
	cs := C.CString(s)
	defer cstdlib.free(cs)
	var count C.int = 3
	count++
	var total C.size_t = C.size_t(n) + 1
	cstdio.printf(C.cstr("%d %zu\n"), count, total)
	line, err := readFirst("/etc/hostname")
	fmt.Println(len(line) >= 0, err == nil)
	var out *cstdio.FILE = cstdio.stdout
	cstdio.fputs(C.cstr("done\n"), out)
	x := cstdlib.atoi(C.cstr("42"))
	fmt.Println(int(x) + 1)
}
