// phase: 14

// zlib built for MinGW-w64 (step 14.4, ADR-082): the zoo program of tests/zoo/zlib.gp on Windows, where uLong is a
// 32-bit unsigned long (LLP64), so C.ulong is 4 bytes. The library is a static libz.a (GOCPP_WINDOWS_LIBS).

//gocpp:link z

package main

import (
	"fmt"
	"unsafe"

	"<zlib.h>"
)

func main() {
	fmt.Println("uLong:", unsafe.Sizeof(C.ulong(0)), "uInt:", unsafe.Sizeof(C.uint(0)))
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
