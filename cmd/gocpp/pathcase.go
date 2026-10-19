package main

import "os"

// Paths on Windows are case-insensitive, and the same folder reaches gocpp in several spellings: the
// working folder as the shell remembers it (c:\Proj), a pattern as the user typed it (C:\proj\x),
// the module folder as the loader walked up to it. The loader compares them as strings (does this
// package lie below that module?), so gocpp spells every path it takes from outside the one way the
// file system does (canonPath, the case of each element as stored, the drive letter in capitals).
// On other systems a path is what it is.

// getwd is os.Getwd with the canonical spelling.
func getwd() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return canonPath(dir), nil
}
