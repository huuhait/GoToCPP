// Code ported from Go 1.25.14 src/math/abs.go; DO NOT EDIT by hand; see PORT.md.

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Abs returns the absolute value of x.
//
// Special cases are:
//
//	Abs(±Inf) = +Inf
//	Abs(NaN) = NaN
//
//gocpp:native gp::math::abs
func Abs(x float64) float64
