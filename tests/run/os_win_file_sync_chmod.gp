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
	f, err := os.Create(d + "/c")
	must(err)
	fmt.Println(e(f.Sync()))
	fmt.Println(e(f.Chmod(0o600)))
	st, _ := f.Stat()
	fmt.Println(st.Mode())
	fmt.Println(e(f.Chmod(0o755)))
	st, _ = f.Stat()
	fmt.Println(st.Mode())
	fmt.Println(e(os.Chmod(d+"/c", 0o444)))
	st, _ = os.Stat(d + "/c")
	fmt.Println(st.Mode())
	fmt.Println(e(os.Chmod(d+"/none", 0o444)))
	f.Close()
	fmt.Println(e(f.Sync()))
	fmt.Println(e(f.Chmod(0o600)))
}

// Output:
// <nil>
// <nil>
// -rw-rw-rw-
// <nil>
// -rw-rw-rw-
// <nil>
// -r--r--r--
// chmod $D/none: The system cannot find the file specified.
// sync $D/c: file already closed
// chmod $D/c: file already closed
