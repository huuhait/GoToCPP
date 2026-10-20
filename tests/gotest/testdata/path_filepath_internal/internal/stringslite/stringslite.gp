// Package stringslite stands in for internal/stringslite in the copy of path/filepath that Go's tests run in (a program cannot import
// Go's internal packages): the three functions that internal/filepathlite calls.
package stringslite

// HasPrefix reports whether the string s begins with prefix.
func HasPrefix(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }

// IndexByte returns the index of the first instance of c in s, or -1 if c is not present in s.
func IndexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// Cut slices s around the first instance of sep, returning the text before and after sep.
func Cut(s, sep string) (before, after string, found bool) {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i], s[i+len(sep):], true
		}
	}
	return s, "", false
}
