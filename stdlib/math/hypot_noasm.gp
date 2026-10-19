// Code ported from Go 1.25.14 src/math/hypot_noasm.go; DO NOT EDIT by hand; see PORT.md.

// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

const haveArchHypot = false

func archHypot(p, q float64) float64 {
	panic("not implemented")
}
