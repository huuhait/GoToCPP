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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	p := d + "/t"
	must(os.WriteFile(p, []byte("abcdefghij"), 0o644))
	fmt.Println(e(os.Truncate(p, 4)))
	b, _ := os.ReadFile(p)
	fmt.Printf("%q\n", b)
	fmt.Println(e(os.Truncate(p, 8)))
	b, _ = os.ReadFile(p)
	fmt.Printf("%q\n", b)
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	must(err)
	fmt.Println(e(f.Truncate(2)))
	st, _ := f.Stat()
	fmt.Println(st.Size())
	fmt.Println(e(f.Truncate(-1)))
	f.Close()
	fmt.Println(e(f.Truncate(1)))
	fmt.Println(e(os.Truncate(d+"/nope", 1)))
	fmt.Println(e(os.Truncate(d, 1)))
}

// Output:
// <nil>
// "abcd"
// <nil>
// "abcd\x00\x00\x00\x00"
// <nil>
// 2
// truncate $D/t: The parameter is incorrect.
// truncate $D/t: file already closed
// open $D/nope: The system cannot find the file specified.
// open $D: is a directory
