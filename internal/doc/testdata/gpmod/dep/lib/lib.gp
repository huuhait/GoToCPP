// Package lib is a dependency of the main module.
package lib

// Version is the version of the library.
const Version = "1.2.3"

// Clamp limits v to the range lo..hi.
func Clamp[T int | float64](v, lo, hi T) T {
	return min(max(v, lo), hi)
}
