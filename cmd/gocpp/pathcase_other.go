//go:build !windows

package main

// canonPath returns p as it is (see pathcase.go).
func canonPath(p string) string { return p }
