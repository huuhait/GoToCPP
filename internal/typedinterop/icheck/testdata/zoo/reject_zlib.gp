// Wrong calls of zlib (the dev shell has it).
//
//gocpp:pkg-config zlib
package main

import "<zlib.h>"

func main() {
	var strm zlib.z_stream
	_ = strm
	_ = zlib.compressBound("x")
	_ = zlib.zlibVersion(1)
	_ = strm.nosuch
	_ = zlib.crc32(0, 1, 2)
}
