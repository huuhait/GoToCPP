// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
	"time"
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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	p := d + "/f.txt"
	must(os.WriteFile(p, []byte("12345"), 0o640))
	must(os.Chmod(p, 0o640))
	fi, err := os.Stat(p)
	fmt.Println(e(err))
	fmt.Println(fi.Name(), fi.Size(), fi.Mode(), fi.Mode().Perm(), fi.Mode().IsRegular(), fi.IsDir())
	fmt.Printf("%T %v\n", fi.ModTime(), time.Since(fi.ModTime()) < time.Minute)
	fmt.Printf("%T\n", fi.Sys())
	di, err := os.Stat(d)
	fmt.Println(di.IsDir(), di.Mode().IsDir(), di.Mode()&os.ModeDir != 0, e(err))
	fmt.Println(di.Name() == d[len(d)-len(di.Name()):])
	_, err = os.Stat(d + "/missing")
	fmt.Println(e(err), os.IsNotExist(err))
	fmt.Println(fi.Mode().String(), di.Mode().String()[0:1])
}

// Output:
// <nil>
// f.txt 5 -rw-rw-rw- -rw-rw-rw- true false
// time.Time true
// *syscall.Win32FileAttributeData
// true true true <nil>
// true
// GetFileAttributesEx $D/missing: The system cannot find the file specified. true
// -rw-rw-rw- d
