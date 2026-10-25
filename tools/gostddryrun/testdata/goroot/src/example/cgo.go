//go:build cgo

package example

import "C"

// CgoOnly exists only with cgo enabled.
func CgoOnly() {}
