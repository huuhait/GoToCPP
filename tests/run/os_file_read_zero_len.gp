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
	must(os.WriteFile(d+"/z", []byte("abc"), 0o644))
	f, _ := os.Open(d + "/z")
	n, err := f.Read(nil)
	fmt.Println(n, e(err))
	n, err = f.Read([]byte{})
	fmt.Println(n, e(err))
	empty, _ := os.Create(d + "/empty")
	n, err = empty.Read(make([]byte, 1))
	fmt.Println(n, err)
	n, err = empty.Read(nil)
	fmt.Println(n, err)
	f.Close()
	n, err = f.Read(nil)
	fmt.Println(n, e(err))
	empty.Close()
}

// Output:
// 0 <nil>
// 0 <nil>
// 0 EOF
// 0 <nil>
// 0 read $D/z: file already closed
