// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package abi is gocpp's small replacement of go1.25.14's src/internal/abi (see stdlib/internal/PORT.md). Go's package
// describes the layout of types, functions and stacks for the compiler and the runtime, and gocpp has
// its own (type descriptors, gocpp/type.hpp), so only what the ported packages call is here: NoEscape
// and Escape, which tell Go's escape analysis that a pointer does not escape. gocpp has no escape
// analysis (values are reference counted), so both return their argument.
package abi

import "unsafe"

// NoEscape hides a pointer from escape analysis. NoEscape is
// the identity function but escape analysis doesn't think the
// output depends on the input. NoEscape is inlined and currently
// compiles down to zero instructions.
//
// In gocpp it is the identity function and nothing else: Go's version converts the pointer to a
// uintptr and back, which would drop the owner that keeps the memory alive (divergence D16).
func NoEscape(p unsafe.Pointer) unsafe.Pointer {
	return p
}

// Escape forces any pointers in x to escape to the heap. In gocpp every pointer already
// keeps its memory alive, so it returns x.
func Escape[T any](x T) T {
	return x
}
