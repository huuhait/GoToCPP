//go:build !unix

package load

// openNoFollow has no flag here: the size check of the open file is the guard.
const openNoFollow = 0
