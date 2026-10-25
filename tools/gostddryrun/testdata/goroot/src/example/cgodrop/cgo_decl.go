//go:build cgo

package cgodrop

import "C"

// FromCgo exists only with cgo enabled.
func FromCgo() int { return 1 }
