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
	fmt.Println(e(os.Mkdir(d+"/a", 0o755)))
	fmt.Println(e(os.Mkdir(d+"/a", 0o755)))
	fmt.Println(e(os.Mkdir(d+"/x/y", 0o755)))
	must(os.WriteFile(d+"/file", nil, 0o644))
	fmt.Println(e(os.Mkdir(d+"/file", 0o755)))
	fmt.Println(e(os.Mkdir(d+"/file/sub", 0o755)))
	fi, _ := os.Stat(d + "/a")
	fmt.Println(fi.IsDir(), fi.Mode()&os.ModeDir != 0)
	fmt.Println(e(os.Mkdir(d+"/sticky", 0o1777)))
	fi, _ = os.Stat(d + "/sticky")
	fmt.Println(fi.Mode()&os.ModeSticky != 0)
	fmt.Println(e(os.Mkdir(d+"/trail/", 0o755)))
}

// Output:
// <nil>
// mkdir $D/a: file exists
// mkdir $D/x/y: no such file or directory
// mkdir $D/file: file exists
// mkdir $D/file/sub: not a directory
// true true
// <nil>
// false
// <nil>
