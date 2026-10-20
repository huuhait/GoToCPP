// Package bytealg stands in for internal/bytealg in the copy of path/filepath that Go's tests run in (a program cannot import Go's
// internal packages; gocpp's own is native code): the two functions that path/filepath and internal/filepathlite call, in Go.
package bytealg

// IndexByteString returns the index of the first instance of c in s, or -1 if c is not present in s.
func IndexByteString(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// CountString returns the number of instances of c in s.
func CountString(s string, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}
