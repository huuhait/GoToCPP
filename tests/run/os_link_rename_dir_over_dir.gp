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
	must(os.MkdirAll(d+"/a/x", 0o755))
	must(os.MkdirAll(d+"/b", 0o755))
	must(os.MkdirAll(d+"/c/y", 0o755))
	fmt.Println(e(os.Rename(d+"/a", d+"/b")))
	_, err := os.Stat(d + "/b/x")
	fmt.Println(e(err))
	fmt.Println(e(os.Rename(d+"/b", d+"/c")))
	_, err = os.Stat(d + "/c/y")
	fmt.Println(e(err))
	fmt.Println(e(os.Rename(d+"/c", d+"/c/y/z")))
}

// Output:
// rename $D/a $D/b: file exists
// stat $D/b/x: no such file or directory
// rename $D/b $D/c: file exists
// <nil>
// rename $D/c $D/c/y/z: invalid argument
