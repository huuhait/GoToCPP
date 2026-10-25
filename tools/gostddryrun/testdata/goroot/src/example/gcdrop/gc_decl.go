//go:build gc

package gcdrop

// FromGC exists only under the gc compiler tag.
func FromGC() int { return 1 }
