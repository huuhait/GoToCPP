// Code ported from Go 1.25.14 src/math/signbit.go; DO NOT EDIT by hand; see PORT.md.

// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Signbit reports whether x is negative or negative zero.
func Signbit(x float64) bool {
	return Float64bits(x)&(1<<63) != 0
}
