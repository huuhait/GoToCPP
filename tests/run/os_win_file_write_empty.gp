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
	f, _ := os.Create(d + "/e")
	n, err := f.Write(nil)
	fmt.Println(n, e(err))
	n, err = f.WriteString("")
	fmt.Println(n, e(err))
	st, _ := f.Stat()
	fmt.Println(st.Size())
	f.Close()
	n, err = f.Write(nil)
	fmt.Println(n, e(err))
	must(os.WriteFile(d+"/e2", nil, 0o600))
	st, _ = os.Stat(d + "/e2")
	fmt.Println(st.Size(), st.Mode())
}

// Output:
// 0 <nil>
// 0 <nil>
// 0
// 0 write $D/e: file already closed
// 0 -rw-rw-rw-
