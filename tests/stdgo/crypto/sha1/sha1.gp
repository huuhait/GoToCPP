// Package sha1 stands in for crypto/sha1 in Go's tests of the standard packages (step 9.10), which use it to fingerprint what an
// io.Writer received: the plain algorithm of RFC 3174 (gocpp has no crypto packages yet).
package sha1

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

// The size of a SHA-1 checksum in bytes.
const Size = 20

// The blocksize of SHA-1 in bytes.
const BlockSize = 64

type digest struct {
	h   [5]uint32
	x   [BlockSize]byte
	nx  int
	len uint64
}

// New returns a new hash.Hash computing the SHA-1 checksum.
func New() hash.Hash {
	d := new(digest)
	d.Reset()
	return d
}

func (d *digest) Reset() {
	d.h = [5]uint32{0x67452301, 0xEFCDAB89, 0x98BADCFE, 0x10325476, 0xC3D2E1F0}
	d.nx = 0
	d.len = 0
}

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return BlockSize }

func (d *digest) Write(p []byte) (nn int, err error) {
	nn = len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == BlockSize {
			block(d, d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	if len(p) >= BlockSize {
		n := len(p) &^ (BlockSize - 1)
		block(d, p[:n])
		p = p[n:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return
}

// Sum appends the current hash to in and returns the resulting slice; it does not change the state.
func (d *digest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *digest) checkSum() [Size]byte {
	length := d.len
	var tmp [64 + 8]byte
	tmp[0] = 0x80
	var t uint64
	if length%64 < 56 {
		t = 56 - length%64
	} else {
		t = 64 + 56 - length%64
	}
	length <<= 3
	padlen := tmp[:t+8]
	binary.BigEndian.PutUint64(padlen[t:], length)
	d.Write(padlen)

	var digest [Size]byte
	for i, s := range d.h {
		binary.BigEndian.PutUint32(digest[i*4:], s)
	}
	return digest
}

// Sum returns the SHA-1 checksum of the data.
func Sum(data []byte) [Size]byte {
	var d digest
	d.Reset()
	d.Write(data)
	return d.checkSum()
}

func block(dig *digest, p []byte) {
	var w [16]uint32
	h0, h1, h2, h3, h4 := dig.h[0], dig.h[1], dig.h[2], dig.h[3], dig.h[4]
	for len(p) >= BlockSize {
		for i := 0; i < 16; i++ {
			w[i] = binary.BigEndian.Uint32(p[i*4:])
		}
		a, b, c, d, e := h0, h1, h2, h3, h4
		for i := 0; i < 80; i++ {
			if i >= 16 {
				tmp := w[(i-3)&0xf] ^ w[(i-8)&0xf] ^ w[(i-14)&0xf] ^ w[(i)&0xf]
				w[i&0xf] = bits.RotateLeft32(tmp, 1)
			}
			var f, k uint32
			switch {
			case i < 20:
				f = b&c | (^b)&d
				k = 0x5A827999
			case i < 40:
				f = b ^ c ^ d
				k = 0x6ED9EBA1
			case i < 60:
				f = ((b | c) & d) | (b & c)
				k = 0x8F1BBCDC
			default:
				f = b ^ c ^ d
				k = 0xCA62C1D6
			}
			t := bits.RotateLeft32(a, 5) + f + e + w[i&0xf] + k
			a, b, c, d, e = t, a, bits.RotateLeft32(b, 30), c, d
		}
		h0 += a
		h1 += b
		h2 += c
		h3 += d
		h4 += e
		p = p[BlockSize:]
	}
	dig.h[0], dig.h[1], dig.h[2], dig.h[3], dig.h[4] = h0, h1, h2, h3, h4
}
