// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

var tmpDir string

func setup() string {
	d, err := os.MkdirTemp("", "gpos")
	if err != nil {
		panic(err)
	}
	tmpDir = d
	return d
}

func cleanup() { os.RemoveAll(tmpDir) }

func idx(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// norm replaces the temporary directory in s by $D.
func norm(s string) string {
	for {
		i := idx(s, tmpDir)
		if i < 0 {
			return s
		}
		s = s[:i] + "$D" + s[i+len(tmpDir):]
	}
}

// e is the text of err, with the temporary directory replaced.
func e(err error) string {
	if err == nil {
		return "<nil>"
	}
	return norm(err.Error())
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	must(os.WriteFile(d+"/file", []byte("12345678"), 0o644))
	must(os.Symlink("file", d+"/lnk"))
	must(os.Symlink("nowhere", d+"/dang"))
	must(os.Symlink(".", d+"/self"))
	l, err := os.Lstat(d + "/lnk")
	fmt.Println(e(err), l.Mode()&os.ModeSymlink != 0, l.Mode().Type() == os.ModeSymlink, l.Size(), l.Name())
	s, err := os.Stat(d + "/lnk")
	fmt.Println(e(err), s.Mode()&os.ModeSymlink != 0, s.Mode().IsRegular(), s.Size(), s.Name())
	_, err = os.Stat(d + "/dang")
	fmt.Println(e(err))
	l, err = os.Lstat(d + "/dang")
	fmt.Println(e(err), l.Mode()&os.ModeSymlink != 0)
	l, _ = os.Lstat(d + "/self")
	s, _ = os.Stat(d + "/self")
	fmt.Println(l.IsDir(), s.IsDir(), l.Mode().String()[0:1])
	fmt.Println(l.Mode().String()[:1], s.Mode().String()[:1])
	_, err = os.Lstat(d + "/file/x")
	fmt.Println(e(err))
}

// Output:
// <nil> true true 0 lnk
// <nil> false true 8 lnk
// CreateFile $D/dang: The system cannot find the file specified.
// <nil> true
// false true L
// L d
// GetFileAttributesEx $D/file/x: The system cannot find the path specified.
