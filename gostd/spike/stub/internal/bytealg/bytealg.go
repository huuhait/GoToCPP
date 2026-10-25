// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bytealg is job 16.1f-stubs' pure-Go stand-in for Go 1.25's
// internal/bytealg (Phase 16 README §3 step 16.1: "Hand-written stubs stand
// in for runtime, internal/bytealg and internal/reflectlite").
//
// Go's package is mostly assembly: Compare, Count, Equal, Index, IndexByte and
// LastIndexByte each pick an amd64/arm64/... implementation, and Index,
// IndexString and MakeNoZero are bodyless there. This package provides only
// the identifiers the four spike packages and their closure use
// (gostd/spike/closure.json records every call site); everything else is
// absent on purpose, so a new use is a compile error, not a silent wrong
// answer.
//
// The algorithms of HashStr, HashStrRev and IndexRabinKarp are Go's own
// (internal/bytealg/bytealg.go); the rest are the straightforward pure-Go
// versions of the assembly contracts. Where the real implementation's result
// is architecture-dependent, the value used here is the amd64 one (Go 1.25's
// index_amd64.go) or a value that is correct on every architecture:
//
//   - MaxLen is 0, which makes internal/stringslite.Index and strings.IndexRune
//     take their general brute-force path (the "n <= MaxLen" fast path is
//     skipped). The real amd64 package sets it to 63 or 31 in init depending
//     on AVX2; the result is the same either way, only the speed differs.
//   - MaxBruteForce is 64, as on amd64 (index_amd64.go), so that
//     strings.IndexRune's haveFastIndex case keeps its shape.
//   - Cutover is amd64's (n+16)/8.
//
// MakeNoZero's contract is "a slice of length n and capacity at least n
// whose bytes are not zeroed; the caller promises not to leak them". A Go
// make zeroes the bytes, which satisfies the contract (it is stronger).
package bytealg

// MaxLen is the maximum length of the string to be searched for (argument b)
// in Index. If MaxLen is not 0, make sure MaxLen >= 4.
//
// The real package sets it in init from internal/cpu; 0 is the safe value
// (see the package comment).
var MaxLen int

// MaxBruteForce is the maximum length of the haystack for which IndexString
// is used directly. amd64's value.
const MaxBruteForce = 64

// PrimeRK is the prime base used in Rabin-Karp algorithm.
const PrimeRK = 16777619

// CompareString returns -1, 0 or +1 comparing a and b byte by byte.
// It is Go's strings.Compare, without the package cycle.
func CompareString(a, b string) int {
	// Compare the common prefix, then the lengths.
	min := len(a)
	if len(b) < min {
		min = len(b)
	}
	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return +1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return +1
	}
	return 0
}

// CountString returns the number of non-overlapping instances of c in s.
func CountString(s string, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}

// IndexByteString returns the index of the first instance of c in s, or -1
// if c is not present in s.
func IndexByteString(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// LastIndexByteString returns the index of the last instance of c in s, or
// -1 if c is not present in s.
func LastIndexByteString(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// IndexString returns the index of the first instance of b in a, or -1 if b
// is not present in a. Go's amd64 version requires 2 <= len(b) <= MaxLen; this
// pure-Go version has no such restriction.
func IndexString(a, b string) int {
	n := len(b)
	switch {
	case n == 0:
		return 0
	case n == 1:
		return IndexByteString(a, b[0])
	case n > len(a):
		return -1
	}
	c0, c1 := b[0], b[1]
	t := len(a) - n + 1
	for i := 0; i < t; i++ {
		if a[i] != c0 {
			o := IndexByteString(a[i+1:t], c0)
			if o < 0 {
				return -1
			}
			i += o + 1
		}
		if a[i+1] == c1 && a[i:i+n] == b {
			return i
		}
	}
	return -1
}

// Cutover reports the number of failures of IndexByte we should tolerate
// before switching over to Index. n is the number of bytes processed so far.
func Cutover(n int) int {
	// 1 error per 8 characters, plus a few slop to start (amd64).
	return (n + 16) / 8
}

// MakeNoZero makes a slice of length n and capacity of at least n bytes
// without zeroing the bytes (including the bytes between len and cap). It is
// the caller's responsibility to ensure uninitialized bytes do not leak to
// the end user.
func MakeNoZero(n int) []byte {
	// make zeroes; the contract allows that (see the package comment).
	return make([]byte, n)
}

// HashStr returns the hash and the appropriate multiplicative factor for use
// in Rabin-Karp algorithm.
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

// HashStrRev returns the hash of the reverse of sep and the appropriate
// multiplicative factor for use in Rabin-Karp algorithm.
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

// IndexRabinKarp uses the Rabin-Karp search algorithm to return the index of
// the first occurrence of sep in s, or -1 if not present.
func IndexRabinKarp[T string | []byte](s, sep T) int {
	// Rabin-Karp search
	hashss, pow := HashStr(sep)
	n := len(sep)
	var h uint32
	for i := 0; i < n; i++ {
		h = h*PrimeRK + uint32(s[i])
	}
	if h == hashss && string(s[:n]) == string(sep) {
		return 0
	}
	for i := n; i < len(s); {
		h *= PrimeRK
		h += uint32(s[i])
		h -= pow * uint32(s[i-n])
		i++
		if h == hashss && string(s[i-n:i]) == string(sep) {
			return i - n
		}
	}
	return -1
}
