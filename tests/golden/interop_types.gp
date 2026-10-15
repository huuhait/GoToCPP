// phase: 04

package main

import (
	"<array>"
	"<bitset>"
	"<cstdio>"
	"<vector>"
)

type Point struct{ X, Y int }

type Node struct{ next *Node }

// A struct with fields of C/C++ types: value-initialized, without ==, gp_hash and the layout
// check, which the C++ compiler knows.
type File struct {
	f    *cstdio.FILE
	name **C.char
	buf  std.vector[C.uchar]
}

var files std.vector[File]

func open(name *C.char, mode *C.char) *cstdio.FILE { return cstdio.fopen(name, mode) }

func sizes(a std.array[C.int, 4], b std.bitset[64]) (C.size_t, C.size_t) {
	return std.size(a), std.size(b)
}

func main() {
	var c C.char
	var sc C.schar
	var uc C.uchar
	var s C.short
	var us C.ushort
	var i C.int = 5
	var ui C.uint
	var l C.long
	var ul C.ulong
	var ll C.longlong
	var ull C.ulonglong = 1 << 63
	var f C.float = 1.5
	var d C.double
	var z C.size_t
	var v std.vector[int]
	var pts std.vector[Point]
	var nodes std.vector[*Node]
	var it std.vector[int].iterator
	var cs []C.int
	var cm map[string]*cstdio.FILE
	var ca [4]C.double
	var pv *[]C.int
	n := int(i) + int(C.int(l)) + int(C.ulonglong(z))
	_, _, _, _, _, _, _, _, _, _, _, _, _, _ = c, sc, uc, s, us, ui, ul, ll, ull, f, d, v, pts, nodes
	_, _, _, _, _, _, _ = it, cs, cm, ca, pv, n, files
}
