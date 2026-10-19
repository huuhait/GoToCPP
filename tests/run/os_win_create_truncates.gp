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
	p := d + "/c"
	must(os.WriteFile(p, []byte("old old old"), 0o644))
	f, err := os.Create(p)
	fmt.Println(e(err))
	st, _ := f.Stat()
	fmt.Println(st.Size(), st.Mode().Perm())
	f.WriteString("new")
	f.Close()
	b, _ := os.ReadFile(p)
	fmt.Println(string(b))
	g, err := os.Create(d + "/sub/file")
	fmt.Println(g == nil, e(err))
}

// Output:
// <nil>
// 0 -rw-rw-rw-
// new
// true open $D/sub/file: The system cannot find the path specified.
