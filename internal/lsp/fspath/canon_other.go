//go:build !windows

package fspath

// nativeLongName: no short names outside Windows.
func nativeLongName(string) (string, bool) { return "", false }
