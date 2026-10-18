// phase: 04
// pkg-config: zlib

//gocpp:pkg-config zlib
package main

import (
	"fmt"
	"unsafe"

	"<zlib.h>"
)

// bytesPtr is the C view of a byte slice (the slice must be non-empty and outlive the call).
func bytesPtr(b []byte) *C.uchar {
	return (*C.uchar)(unsafe.Pointer(&b[0]))
}

func compress(src []byte) []byte {
	// compressBound is the largest size compress can produce; make accepts a C number.
	dst := make([]byte, zlib.compressBound(C.ulong(len(src))))
	n := C.ulong(len(dst)) // in: the room in dst; out: the bytes written
	if zlib.compress(bytesPtr(dst), &n, bytesPtr(src), C.ulong(len(src))) != zlib.Z_OK {
		panic("compress failed")
	}
	return dst[:n]
}

func decompress(src []byte, size int) []byte {
	dst := make([]byte, size)
	n := C.ulong(size)
	if zlib.uncompress(bytesPtr(dst), &n, bytesPtr(src), C.ulong(len(src))) != zlib.Z_OK {
		panic("not zlib data")
	}
	return dst[:n]
}

func main() {
	text := []byte("hello hello hello hello hello hello hello hello")
	packed := compress(text)
	back := decompress(packed, 100)
	fmt.Println(len(text), len(packed) < len(text), string(back) == string(text))
	fmt.Println(zlib.crc32(0, bytesPtr(text), C.uint(len(text))))
}

// Output:
// 47 true true
// 3106698858
