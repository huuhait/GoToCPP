// phase: 12
// pkg-config: zlib
// opaque: reject

// zlib with gocpp strings for the file name, the mode and the text, and C numbers into gocpp numbers.
//
//gocpp:pkg-config zlib
package main

import (
	"fmt"
	"unsafe"

	"<cstdio>"
	"<unistd.h>"
	"<zlib.h>"
)

func main() {
	path := "/tmp/gocpp-typed-" + fmt.Sprint(unistd.getpid()) + ".gz"
	defer cstdio.remove(path)

	mode := "wb"
	gz := zlib.gzopen(path, mode)
	text := "hello hello hello\n"
	zlib.gzputs(gz, text)
	zlib.gzputs(gz, text+"second line\n")
	zlib.gzclose(gz)

	rd := zlib.gzopen(path, "rb")
	var buf [64]C.char
	var lines int
	for zlib.gzgets(rd, &buf[0], 64) != nil {
		lines++
		fmt.Print(C.GoString(&buf[0]))
	}
	zlib.gzclose(rd)
	fmt.Println("lines:", lines)

	src := []byte("hello hello hello hello hello hello hello hello")
	var bound int = zlib.compressBound(C.ulong(len(src)))
	fmt.Println(bound > len(src))
	crc := zlib.crc32(0, (*C.uchar)(unsafe.Pointer(&src[0])), C.uint(len(src)))
	var sum uint32 = crc
	fmt.Println(sum, C.GoString(zlib.zError(zlib.Z_DATA_ERROR)))
}

// Output:
// hello hello hello
// hello hello hello
// second line
// lines: 3
// true
// 3106698858 data error
