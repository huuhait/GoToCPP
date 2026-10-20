// Package asan stands in for internal/asan: gocpp's sanitizer builds are not Go's, so Enabled is false.
package asan

// Enabled reports whether the code is built with the address sanitizer (it never is, for Go's tests).
const Enabled = false
