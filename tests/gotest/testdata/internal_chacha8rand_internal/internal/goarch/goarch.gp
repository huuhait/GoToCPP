// Package goarch stands in for internal/goarch in the copies of ported packages that Go's tests run in (a program cannot import
// Go's internal packages): the constants for the one target, a little-endian 64-bit machine.
package goarch

// BigEndian reports whether the architecture is big-endian.
const BigEndian = false

// PtrSize is the size of a pointer in bytes.
const PtrSize = 8
