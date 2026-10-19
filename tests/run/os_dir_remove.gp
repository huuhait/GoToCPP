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
	must(os.MkdirAll(d+"/dir/sub", 0o755))
	must(os.WriteFile(d+"/file", []byte("x"), 0o644))
	fmt.Println(e(os.Remove(d + "/file")))
	fmt.Println(e(os.Remove(d + "/file")))
	fmt.Println(e(os.Remove(d + "/dir")))
	fmt.Println(e(os.Remove(d + "/dir/sub")))
	fmt.Println(e(os.Remove(d + "/dir")))
	fmt.Println(e(os.Remove(d + "/nope/nope")))
	must(os.WriteFile(d+"/f2", nil, 0o644))
	fmt.Println(e(os.Remove(d + "/f2/x")))
	must(os.Symlink("/nonexistent", d+"/dangling"))
	fmt.Println(e(os.Remove(d + "/dangling")))
	fmt.Println(e(os.Remove(d + "/dangling")))
}

// Output:
// <nil>
// remove $D/file: no such file or directory
// remove $D/dir: directory not empty
// <nil>
// <nil>
// remove $D/nope/nope: no such file or directory
// remove $D/f2/x: not a directory
// <nil>
// remove $D/dangling: no such file or directory
