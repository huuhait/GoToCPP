//go:build !unix

package load

// openNoFollow has no flag here: the size check of the open file is the guard.
const openNoFollow = 0

// openNonblock has no flag here: a named pipe has no name in a folder.
const openNonblock = 0

// isNoDevice is false here: a socket has no name in a folder.
func isNoDevice(error) bool { return false }
