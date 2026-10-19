// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The functions of Go's syscall.go that every system shares (gocpp's subset:
// stdlib/syscall/PORT.md).

package syscall

// ByteSliceFromString returns a NUL-terminated slice containing the text of s.
// If s contains a NUL byte at any location, it returns (nil, [EINVAL]).
func ByteSliceFromString(s string) ([]byte, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return nil, EINVAL
		}
	}
	a := make([]byte, len(s)+1)
	copy(a, s)
	return a, nil
}

// StringByteSlice converts a string to a NUL-terminated []byte.
// If s contains a NUL byte this function panics instead of returning an error.
//
// Deprecated: Use [ByteSliceFromString] instead.
func StringByteSlice(s string) []byte {
	a, err := ByteSliceFromString(s)
	if err != nil {
		panic("syscall: string with NUL passed to StringByteSlice")
	}
	return a
}

// StringBytePtr returns a pointer to a NUL-terminated array of bytes.
// If s contains a NUL byte this function panics instead of returning an error.
//
// Deprecated: Use [BytePtrFromString] instead.
func StringBytePtr(s string) *byte { return &StringByteSlice(s)[0] }

// BytePtrFromString returns a pointer to a NUL-terminated array of
// bytes containing the text of s. If s contains a NUL byte at any
// location, it returns (nil, [EINVAL]).
func BytePtrFromString(s string) (*byte, error) {
	a, err := ByteSliceFromString(s)
	if err != nil {
		return nil, err
	}
	return &a[0], nil
}
