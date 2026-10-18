// phase: 04
// pkg-config: zlib

// zlib: compress/uncompress round trip through C buffers (Bytef is unsigned char; the in/out
// length is an in-out C.ulong; make takes a C number as its size, as in cgo), crc32 and adler32
// on a byte slice's memory, the Z_* macros, and a z_stream struct with deflateInit (a function-like
// macro around deflateInit_ that uses sizeof(z_stream)).
//
//gocpp:pkg-config zlib
package main

import (
	"fmt"
	"unsafe"

	"<zlib.h>"
)

func main() {
	src := []byte("hello hello hello hello hello hello hello hello")
	bound := zlib.compressBound(C.ulong(len(src)))
	dst := make([]byte, bound)
	dlen := C.ulong(len(dst))
	rc := zlib.compress((*C.uchar)(unsafe.Pointer(&dst[0])), &dlen, (*C.uchar)(unsafe.Pointer(&src[0])), C.ulong(len(src)))
	fmt.Println(rc == zlib.Z_OK, int(dlen) < len(src))

	out := make([]byte, 100)
	olen := C.ulong(len(out))
	rc = zlib.uncompress((*C.uchar)(unsafe.Pointer(&out[0])), &olen, (*C.uchar)(unsafe.Pointer(&dst[0])), dlen)
	fmt.Println(rc == zlib.Z_OK, int(olen), string(out[:olen]) == string(src))

	crc := zlib.crc32(0, (*C.uchar)(unsafe.Pointer(&src[0])), C.uint(len(src)))
	fmt.Println(uint32(crc), uint32(zlib.adler32(1, (*C.uchar)(unsafe.Pointer(&src[0])), 5)))

	var zs zlib.z_stream
	rc = zlib.deflateInit(&zs, zlib.Z_BEST_COMPRESSION)
	fmt.Println(rc == zlib.Z_OK, C.GoString(zlib.zlibVersion()) != "")
	zlib.deflateEnd(&zs)

	bad := []byte("not zlib data at all")
	blen := C.ulong(100)
	rc = zlib.uncompress((*C.uchar)(unsafe.Pointer(&out[0])), &blen, (*C.uchar)(unsafe.Pointer(&bad[0])), C.ulong(len(bad)))
	fmt.Println(rc == zlib.Z_DATA_ERROR)
}

// Output:
// true true
// true 47 true
// 3106698858 103547413
// true true
// true
