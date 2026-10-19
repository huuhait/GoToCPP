// phase: 09
// std-internal: tests internal/byteorder, which only the standard library may import

// internal/byteorder, Go's own code: the little- and big-endian decoders, writers and appenders of
// 16, 32 and 64 bits. The bytes are written out by hand, and a seeded round trip runs each function
// on 20,000 values, with the endianness checked against the other one (the big-endian bytes are the
// little-endian ones in reverse). The functions that write check the slice first, so a short slice
// is an index panic and leaves the bytes as they were.
package main

import (
	"fmt"
	"internal/byteorder"
)

type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return r.s
}

func reversed(b []byte) []byte {
	r := make([]byte, len(b))
	for i := range b {
		r[len(b)-1-i] = b[i]
	}
	return r
}

func tryPut(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	f()
	return "no panic"
}

func main() {
	var checks, failures int
	check := func(what string, got, want any) {
		checks++
		if fmt.Sprint(got) != fmt.Sprint(want) {
			failures++
			fmt.Printf("FAIL %s: got %v, want %v\n", what, got, want)
		}
	}

	b := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	check("LEUint16", byteorder.LEUint16(b), 0x0201)
	check("BEUint16", byteorder.BEUint16(b), 0x0102)
	check("LEUint32", byteorder.LEUint32(b), 0x04030201)
	check("BEUint32", byteorder.BEUint32(b), 0x01020304)
	check("LEUint64", byteorder.LEUint64(b), uint64(0x0807060504030201))
	check("BEUint64", byteorder.BEUint64(b), uint64(0x0102030405060708))
	check("LEUint64 of a view", byteorder.LEUint64(b[1:]), uint64(0x0908070605040302))
	check("BEUint16 high bit", byteorder.BEUint16([]byte{0xff, 0xfe}), 0xfffe)
	check("LEUint32 high bit", byteorder.LEUint32([]byte{0xff, 0xff, 0xff, 0xff}), uint32(0xffffffff))
	check("BEUint64 high bit", byteorder.BEUint64([]byte{0x80, 0, 0, 0, 0, 0, 0, 1}), uint64(0x8000000000000001))

	check("append LE16", byteorder.LEAppendUint16([]byte{9}, 0xabcd), []byte{9, 0xcd, 0xab})
	check("append BE16", byteorder.BEAppendUint16(nil, 0xabcd), []byte{0xab, 0xcd})
	check("append LE32", byteorder.LEAppendUint32(nil, 0x01020304), []byte{4, 3, 2, 1})
	check("append BE32", byteorder.BEAppendUint32(nil, 0x01020304), []byte{1, 2, 3, 4})
	check("append LE64", byteorder.LEAppendUint64(nil, 0x0102030405060708), []byte{8, 7, 6, 5, 4, 3, 2, 1})
	check("append BE64", byteorder.BEAppendUint64(nil, 0x0102030405060708), []byte{1, 2, 3, 4, 5, 6, 7, 8})

	// Put writes only the bytes of its width.
	p := []byte{0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee}
	byteorder.LEPutUint16(p, 0x0102)
	check("LEPutUint16", p[:3], []byte{2, 1, 0xee})
	byteorder.BEPutUint32(p[1:], 0x0a0b0c0d)
	check("BEPutUint32", p[:6], []byte{2, 0x0a, 0x0b, 0x0c, 0x0d, 0xee})

	// A short slice: Go checks the last index first, so nothing is written.
	short := []byte{0xee, 0xee}
	check("short LEPutUint32", tryPut(func() { byteorder.LEPutUint32(short, 1) }), "runtime error: index out of range [3] with length 2")
	check("short BEPutUint64", tryPut(func() { byteorder.BEPutUint64(short, 1) }), "runtime error: index out of range [7] with length 2")
	check("short LEUint16", tryPut(func() { byteorder.LEUint16(short[:1]) }), "runtime error: index out of range [1] with length 1")
	check("short is untouched", short, []byte{0xee, 0xee})

	r := &rng{0x853c49e6748fea9b}
	buf := make([]byte, 8)
	for i := 0; i < 20000; i++ {
		v := r.next()
		byteorder.LEPutUint64(buf, v)
		check("LE64 round trip", byteorder.LEUint64(buf), v)
		le := append([]byte(nil), buf...)
		byteorder.BEPutUint64(buf, v)
		check("BE64 round trip", byteorder.BEUint64(buf), v)
		check("BE64 is LE64 reversed", buf, reversed(le))
		check("BE64 append", byteorder.BEAppendUint64(nil, v), buf)
		check("LE64 append", byteorder.LEAppendUint64(nil, v), le)
		v32 := uint32(v >> 11)
		byteorder.LEPutUint32(buf, v32)
		check("LE32 round trip", byteorder.LEUint32(buf), v32)
		check("LE32 append", byteorder.LEAppendUint32(nil, v32), buf[:4])
		byteorder.BEPutUint32(buf, v32)
		check("BE32 round trip", byteorder.BEUint32(buf), v32)
		check("BE32 append", byteorder.BEAppendUint32(nil, v32), buf[:4])
		v16 := uint16(v >> 29)
		byteorder.LEPutUint16(buf, v16)
		check("LE16 round trip", byteorder.LEUint16(buf), v16)
		check("LE16 append", byteorder.LEAppendUint16(nil, v16), buf[:2])
		byteorder.BEPutUint16(buf, v16)
		check("BE16 round trip", byteorder.BEUint16(buf), v16)
		check("BE16 append", byteorder.BEAppendUint16(nil, v16), buf[:2])
	}
	fmt.Println(checks, "checks,", failures, "failures")
}

// Output:
// 260022 checks, 0 failures
