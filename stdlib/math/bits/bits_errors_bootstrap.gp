// Code ported from Go 1.25.14 src/math/bits/bits_errors_bootstrap.go; DO NOT EDIT by hand; see PORT.md.

// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This version used only for bootstrap (on this path we want
// to avoid use of go:linkname as applied to variables).

package bits

type errorString string

func (e errorString) RuntimeError() {}

func (e errorString) Error() string {
	return "runtime error: " + string(e)
}

var overflowError = error(errorString("integer overflow"))

var divideError = error(errorString("integer divide by zero"))
