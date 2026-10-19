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
	must(os.WriteFile(d+"/a", []byte("A"), 0o644))
	must(os.WriteFile(d+"/b", []byte("B"), 0o644))
	fmt.Println(e(os.Rename(d+"/a", d+"/c")))
	_, err := os.Stat(d + "/a")
	fmt.Println(os.IsNotExist(err))
	fmt.Println(e(os.Rename(d+"/c", d+"/b")))
	b, _ := os.ReadFile(d + "/b")
	fmt.Println(string(b))
	fmt.Println(e(os.Rename(d+"/zz", d+"/yy")))
	must(os.Mkdir(d+"/dir", 0o755))
	fmt.Println(e(os.Rename(d+"/b", d+"/dir")))
	fmt.Println(e(os.Rename(d+"/dir", d+"/b")))
	fmt.Println(e(os.Rename(d+"/dir", d+"/dir2")))
	fmt.Println(e(os.Rename(d+"/dir2", d+"/dir2")))
}

// Output:
// <nil>
// true
// <nil>
// A
// rename $D/zz $D/yy: no such file or directory
// rename $D/b $D/dir: file exists
// rename $D/dir $D/b: not a directory
// <nil>
// rename $D/dir2 $D/dir2: file exists
