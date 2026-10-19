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
	f, err := os.OpenFile(d+"/ap", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	must(err)
	n, err := f.WriteAt([]byte("abc"), 0)
	fmt.Println(n, e(err))
	f.WriteString("one")
	f.WriteString("two")
	f.Close()
	b, _ := os.ReadFile(d + "/ap")
	fmt.Printf("%q\n", b)
}

// Output:
// 0 os: invalid use of WriteAt on file opened with O_APPEND
// "onetwo"
