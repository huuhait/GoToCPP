// Package msan stands in for internal/msan: gocpp's sanitizer builds are not Go's, so Enabled is false.
package msan

// Enabled reports whether the code is built with the memory sanitizer (it never is, for Go's tests).
const Enabled = false
