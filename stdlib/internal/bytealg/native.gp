// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The routines that Go writes in assembly for each processor (indexbyte_*.s, count_*.s, ...) and that
// have a Go fallback (the *_generic.go files). Here each one is a C++ function over memchr, memcmp
// and memmem (ADR-027: //gocpp:native names the function; runtime/include/gocpp/bytealg.hpp
// declares it, runtime/src/bytealg.cpp defines the ones that are not inline).
//
// Every function takes a nil slice as an empty one, and none of them keeps a reference to its
// arguments.

package bytealg

// IndexByte returns the index of the first instance of c in b, or -1 if c is not present in b.
//
//gocpp:native gp::bytealg::index_byte
func IndexByte(b []byte, c byte) int

// IndexByteString returns the index of the first instance of c in s, or -1 if c is not present in s.
//
//gocpp:native gp::bytealg::index_byte_string
func IndexByteString(s string, c byte) int

// LastIndexByte returns the index of the last instance of c in s, or -1 if c is not present in s.
//
//gocpp:native gp::bytealg::last_index_byte
func LastIndexByte(s []byte, c byte) int

// LastIndexByteString returns the index of the last instance of c in s, or -1 if c is not present in s.
//
//gocpp:native gp::bytealg::last_index_byte_string
func LastIndexByteString(s string, c byte) int

// Count returns the number of bytes of b that are equal to c.
//
//gocpp:native gp::bytealg::count
func Count(b []byte, c byte) int

// CountString returns the number of bytes of s that are equal to c.
//
//gocpp:native gp::bytealg::count_string
func CountString(s string, c byte) int

// Equal reports whether a and b
// are the same length and contain the same bytes.
// A nil argument is equivalent to an empty slice.
//
// Equal is equivalent to bytes.Equal.
// It is provided here for convenience,
// because some packages cannot depend on bytes.
//
//gocpp:native gp::bytealg::equal
func Equal(a, b []byte) bool

// Compare returns -1, 0 or +1: the order of a and b as byte strings.
//
//gocpp:native gp::bytealg::compare
func Compare(a, b []byte) int

// CompareString returns -1, 0 or +1: the order of a and b as byte strings.
//
//gocpp:native gp::bytealg::compare_string
func CompareString(a, b string) int

// Index returns the index of the first instance of b in a, or -1 if b is not present in a.
// Requires 2 <= len(b) <= MaxLen.
//
//gocpp:native gp::bytealg::index
func Index(a, b []byte) int

// IndexString returns the index of the first instance of b in a, or -1 if b is not present in a.
// Requires 2 <= len(b) <= MaxLen.
//
//gocpp:native gp::bytealg::index_string
func IndexString(a, b string) int

// MakeNoZero makes a slice of length n and capacity of at least n Bytes
// without zeroing the bytes (including the bytes between len and cap).
// It is the caller's responsibility to ensure uninitialized bytes
// do not leak to the end user.
//
//gocpp:native gp::bytealg::make_no_zero
func MakeNoZero(n int) []byte
