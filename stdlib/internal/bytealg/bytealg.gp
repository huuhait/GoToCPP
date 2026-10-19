// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bytealg is gocpp's version of go1.25.14's src/internal/bytealg (see stdlib/internal/PORT.md). Go's package is
// the byte-searching routines of the standard library: assembly for each processor, with Go fallbacks.
// Here the routines are C++ (native.gp, runtime/src/bytealg.cpp, runtime/include/gocpp/bytealg.hpp),
// and what is Go in Go's package (the Rabin-Karp functions, which are generic) is Go here.
package bytealg

// MaxLen is the maximum length of the string to be searched for (argument b) in Index.
// If MaxLen is not 0, make sure MaxLen >= 4.
//
// Go sets it to 63 on amd64 with AVX2 (31 without it; arm64 has 32, other processors 0), in init.
// Index here is memmem, which has no such limit; 63 keeps the code of strings and bytes on the path
// that Go takes on amd64 with AVX2.
var MaxLen int = 63

// MaxBruteForce is the largest haystack that strings.Index and bytes.Index search with Index
// right away (Go's amd64 value; arm64 has 16).
const MaxBruteForce = 64

// Cutover reports the number of failures of IndexByte we should tolerate
// before switching over to Index.
// n is the number of bytes processed so far.
// See the bytes.Index implementation for details.
func Cutover(n int) int {
	// 1 error per 8 characters, plus a few slop to start.
	return (n + 16) / 8
}

// PrimeRK is the prime base used in Rabin-Karp algorithm.
const PrimeRK = 16777619

// HashStr returns the hash and the appropriate multiplicative
// factor for use in Rabin-Karp algorithm.
func HashStr[T string | []byte](sep T) (uint32, uint32) {
	hash := uint32(0)
	for i := 0; i < len(sep); i++ {
		hash = hash*PrimeRK + uint32(sep[i])
	}
	var pow, sq uint32 = 1, PrimeRK
	for i := len(sep); i > 0; i >>= 1 {
		if i&1 != 0 {
			pow *= sq
		}
		sq *= sq
	}
	return hash, pow
}

// HashStrRev returns the hash of the reverse of sep and the
// appropriate multiplicative factor for use in Rabin-Karp algorithm.
func HashStrRev[T string | []byte](sep T) (uint32, uint32) {
	hash := uint32(0)
	for i := len(sep) - 1; i >= 0; i-- {
		hash = hash*PrimeRK + uint32(sep[i])
	}
	var pow, sq uint32 = 1, PrimeRK
	for i := len(sep); i > 0; i >>= 1 {
		if i&1 != 0 {
			pow *= sq
		}
		sq *= sq
	}
	return hash, pow
}

// sameBytes reports whether s[i:i+n] and sep[:n] hold the same bytes. Go writes
// string(s[i:i+n]) == string(sep) and its compiler converts nothing; here the conversion of a
// []byte would copy, so the bytes are compared one by one.
func sameBytes[T string | []byte](s T, i, n int, sep T) bool {
	for j := 0; j < n; j++ {
		if s[i+j] != sep[j] {
			return false
		}
	}
	return true
}

// IndexRabinKarp uses the Rabin-Karp search algorithm to return the index of the
// first occurrence of sep in s, or -1 if not present.
func IndexRabinKarp[T string | []byte](s, sep T) int {
	// Rabin-Karp search
	hashss, pow := HashStr(sep)
	n := len(sep)
	var h uint32
	for i := 0; i < n; i++ {
		h = h*PrimeRK + uint32(s[i])
	}
	if h == hashss && sameBytes(s, 0, n, sep) {
		return 0
	}
	for i := n; i < len(s); {
		h *= PrimeRK
		h += uint32(s[i])
		h -= pow * uint32(s[i-n])
		i++
		if h == hashss && sameBytes(s, i-n, n, sep) {
			return i - n
		}
	}
	return -1
}

// LastIndexRabinKarp uses the Rabin-Karp search algorithm to return the last index of the
// occurrence of sep in s, or -1 if not present.
func LastIndexRabinKarp[T string | []byte](s, sep T) int {
	// Rabin-Karp search from the end of the string
	hashss, pow := HashStrRev(sep)
	n := len(sep)
	last := len(s) - n
	var h uint32
	for i := len(s) - 1; i >= last; i-- {
		h = h*PrimeRK + uint32(s[i])
	}
	if h == hashss && sameBytes(s, last, n, sep) {
		return last
	}
	for i := last - 1; i >= 0; i-- {
		h *= PrimeRK
		h += uint32(s[i])
		h -= pow * uint32(s[i+n])
		if h == hashss && sameBytes(s, i, n, sep) {
			return i
		}
	}
	return -1
}
