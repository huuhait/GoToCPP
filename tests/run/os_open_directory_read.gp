// phase: 09

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
	f, err := os.Open(d)
	fmt.Println(e(err))
	buf := make([]byte, 4)
	n, err := f.Read(buf)
	fmt.Println(n, e(err))
	n, err = f.Write(buf)
	fmt.Println(n, e(err))
	_, err = os.ReadFile(d)
	fmt.Println(e(err))
	err = os.WriteFile(d, []byte("x"), 0o644)
	fmt.Println(e(err))
	_, err = os.Create(d)
	fmt.Println(e(err))
	f.Close()
}

// Output:
// <nil>
// 0 read $D: is a directory
// 0 write $D: bad file descriptor
// read $D: is a directory
// open $D: is a directory
// open $D: is a directory
